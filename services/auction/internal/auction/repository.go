package auction

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbTypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/redis/go-redis/v9"
)

const (
	activeSetKey   = "auctions:active"
	auctionsTable  = "Auctions"
	rebuildLockTTL = 2 * time.Second
)

// dynamoAuction is the DynamoDB-friendly representation of an Auction.
type dynamoAuction struct {
	AuctionID      string `dynamodbav:"auction_id"`
	SellerID       string `dynamodbav:"seller_id"`
	ItemID         string `dynamodbav:"item_id"`
	ItemTitle      string `dynamodbav:"item_title"`
	ShopID         string `dynamodbav:"shop_id"`
	ShopName       string `dynamodbav:"shop_name"`
	RetailPrice    int64  `dynamodbav:"retail_price"`
	MaxPrice       int64  `dynamodbav:"max_price"`
	ImageURL       string `dynamodbav:"image_url"`
	ShopLogoURL    string `dynamodbav:"shop_logo_url"`
	Description    string `dynamodbav:"description"`
	Category       string `dynamodbav:"category,omitempty"`
	StartTime      string `dynamodbav:"start_time"`
	EndTime        string `dynamodbav:"end_time"`
	CurrentHighest int64  `dynamodbav:"current_highest"`
	BidCount       int64  `dynamodbav:"bid_count"`
	HighestBidder  string `dynamodbav:"highest_bidder"`
	Status         string `dynamodbav:"status"`
	Version        int64  `dynamodbav:"version"`
}

func toDynamo(a *Auction) dynamoAuction {
	return dynamoAuction{
		AuctionID:      a.AuctionID,
		SellerID:       a.SellerID,
		ItemID:         a.ItemID,
		ItemTitle:      a.ItemTitle,
		ShopID:         a.ShopID,
		ShopName:       a.ShopName,
		RetailPrice:    a.RetailPrice,
		MaxPrice:       a.MaxPrice,
		ImageURL:       a.ImageURL,
		ShopLogoURL:    a.ShopLogoURL,
		Description:    a.Description,
		Category:       a.Category,
		StartTime:      a.StartTime.Format(time.RFC3339),
		EndTime:        a.EndTime.Format(time.RFC3339),
		CurrentHighest: a.CurrentHighest,
		BidCount:       a.BidCount,
		HighestBidder:  a.HighestBidder,
		Status:         a.Status,
		Version:        a.Version,
	}
}

func fromDynamo(d dynamoAuction) *Auction {
	startTime, _ := time.Parse(time.RFC3339, d.StartTime)
	endTime, _ := time.Parse(time.RFC3339, d.EndTime)
	return &Auction{
		AuctionID:      d.AuctionID,
		SellerID:       d.SellerID,
		ItemID:         d.ItemID,
		ItemTitle:      d.ItemTitle,
		ShopID:         d.ShopID,
		ShopName:       d.ShopName,
		RetailPrice:    d.RetailPrice,
		MaxPrice:       d.MaxPrice,
		ImageURL:       d.ImageURL,
		ShopLogoURL:    d.ShopLogoURL,
		Description:    d.Description,
		Category:       d.Category,
		StartTime:      startTime,
		EndTime:        endTime,
		CurrentHighest: d.CurrentHighest,
		BidCount:       d.BidCount,
		HighestBidder:  d.HighestBidder,
		Status:         d.Status,
		Version:        d.Version,
	}
}

// Repository handles Redis + DynamoDB operations for auctions.
// Redis is the hot path; DynamoDB is the durable backing store.
type Repository struct {
	rdb *redis.Client
	db  *dynamodb.Client // nil = DynamoDB disabled (backwards-compatible)
}

// NewRepository creates a new Repository with Redis only.
func NewRepository(rdb *redis.Client) *Repository {
	return &Repository{rdb: rdb}
}

// NewRepositoryWithDynamo creates a Repository with Redis + DynamoDB backing store.
func NewRepositoryWithDynamo(rdb *redis.Client, db *dynamodb.Client) *Repository {
	return &Repository{rdb: rdb, db: db}
}

func auctionKey(id string) string         { return "auction:" + id }
func shopAuctionsKey(shopID string) string { return "shop:" + shopID + ":auctions" }
func rebuildLockKey(id string) string      { return "rebuild:auction:" + id }

// Create stores a new auction in both Redis and DynamoDB.
func (r *Repository) Create(ctx context.Context, a *Auction) error {
	key := auctionKey(a.AuctionID)
	pipe := r.rdb.Pipeline()
	pipe.HSet(ctx, key, auctionToRedisMap(a))
	pipe.SAdd(ctx, activeSetKey, a.AuctionID)
	pipe.SAdd(ctx, shopAuctionsKey(a.ShopID), a.AuctionID)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("create auction (redis): %w", err)
	}

	// Write-through to DynamoDB
	if r.db != nil {
		if err := r.saveToDynamo(ctx, a); err != nil {
			log.Printf("WARN: create auction dynamo write-through failed for %s: %v", a.AuctionID, err)
		}
	}

	return nil
}

// GetByID retrieves an auction. Tries Redis first; on miss, falls back to
// DynamoDB and rebuilds the Redis cache with stampede protection.
func (r *Repository) GetByID(ctx context.Context, auctionID string) (*Auction, error) {
	vals, err := r.rdb.HGetAll(ctx, auctionKey(auctionID)).Result()
	if err != nil {
		return nil, fmt.Errorf("get auction: %w", err)
	}
	if len(vals) > 0 {
		return parseAuction(vals)
	}

	// Cache miss — try DynamoDB fallback
	if r.db == nil {
		return nil, errors.New("auction not found")
	}

	return r.ensureRedisCached(ctx, auctionID)
}

// ensureRedisCached rebuilds the Redis cache from DynamoDB with double-checked
// locking to prevent thundering herd.
func (r *Repository) ensureRedisCached(ctx context.Context, auctionID string) (*Auction, error) {
	lockKey := rebuildLockKey(auctionID)

	// Try to acquire rebuild lock
	acquired, err := r.rdb.SetNX(ctx, lockKey, "1", rebuildLockTTL).Result()
	if err != nil {
		return nil, fmt.Errorf("rebuild lock: %w", err)
	}

	if !acquired {
		// Another goroutine is rebuilding — wait briefly and re-read
		time.Sleep(100 * time.Millisecond)
		vals, err := r.rdb.HGetAll(ctx, auctionKey(auctionID)).Result()
		if err == nil && len(vals) > 0 {
			return parseAuction(vals)
		}
		// Still missing — fall through to load from DynamoDB ourselves
	}
	defer r.rdb.Del(ctx, lockKey)

	// Double-check: maybe another process rebuilt while we acquired the lock
	vals, err := r.rdb.HGetAll(ctx, auctionKey(auctionID)).Result()
	if err == nil && len(vals) > 0 {
		return parseAuction(vals)
	}

	// Load from DynamoDB
	a, err := r.getFromDynamo(ctx, auctionID)
	if err != nil {
		return nil, err
	}

	// Rebuild Redis cache
	key := auctionKey(auctionID)
	pipe := r.rdb.Pipeline()
	pipe.HSet(ctx, key, auctionToRedisMap(a))
	if a.Status != "CLOSED" {
		pipe.SAdd(ctx, activeSetKey, a.AuctionID)
	}
	pipe.SAdd(ctx, shopAuctionsKey(a.ShopID), a.AuctionID)
	if _, err := pipe.Exec(ctx); err != nil {
		log.Printf("WARN: rebuild redis cache failed for %s: %v", auctionID, err)
	}

	return a, nil
}

// List returns auctions filtered by status.
func (r *Repository) List(ctx context.Context, status string) ([]*Auction, error) {
	ids, err := r.rdb.SMembers(ctx, activeSetKey).Result()
	if err != nil {
		return nil, fmt.Errorf("list active auctions: %w", err)
	}

	auctions := make([]*Auction, 0, len(ids))
	for _, id := range ids {
		a, err := r.GetByID(ctx, id)
		if err != nil {
			continue
		}
		if status == "" || a.Status == status {
			auctions = append(auctions, a)
		}
	}
	return auctions, nil
}

// ListByShop returns all auctions belonging to a shop.
func (r *Repository) ListByShop(ctx context.Context, shopID string) ([]*Auction, error) {
	ids, err := r.rdb.SMembers(ctx, shopAuctionsKey(shopID)).Result()
	if err != nil {
		return nil, fmt.Errorf("list shop auctions: %w", err)
	}

	auctions := make([]*Auction, 0, len(ids))
	for _, id := range ids {
		a, err := r.GetByID(ctx, id)
		if err != nil {
			continue
		}
		auctions = append(auctions, a)
	}
	return auctions, nil
}

// UpdateHighestBid atomically updates the highest bid using optimistic locking.
func (r *Repository) UpdateHighestBid(ctx context.Context, auctionID string, amount int64, bidderID string, expectedVersion int64) error {
	key := auctionKey(auctionID)

	err := r.rdb.Watch(ctx, func(tx *redis.Tx) error {
		versionStr, err := tx.HGet(ctx, key, "version").Result()
		if err != nil {
			return fmt.Errorf("get version: %w", err)
		}
		version, _ := strconv.ParseInt(versionStr, 10, 64)
		if version != expectedVersion {
			return errors.New("version conflict")
		}

		_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			pipe.HSet(ctx, key, map[string]interface{}{
				"current_highest": amount,
				"highest_bidder":  bidderID,
				"version":         expectedVersion + 1,
			})
			pipe.HIncrBy(ctx, key, "bid_count", 1)
			return nil
		})
		return err
	}, key)

	if err != nil {
		return fmt.Errorf("update highest bid: %w", err)
	}
	return nil
}

// Open transitions a PENDING auction to OPEN.
func (r *Repository) Open(ctx context.Context, auctionID string) error {
	key := auctionKey(auctionID)
	if err := r.rdb.HSet(ctx, key, "status", "OPEN").Err(); err != nil {
		return fmt.Errorf("open auction: %w", err)
	}
	return nil
}

// Close marks an auction as CLOSED, removes from active set, persists final
// state to DynamoDB, and deletes the Redis hash to reclaim memory.
func (r *Repository) Close(ctx context.Context, auctionID string) error {
	key := auctionKey(auctionID)

	// Read full state before closing (needed for DynamoDB write)
	var finalAuction *Auction
	if r.db != nil {
		vals, err := r.rdb.HGetAll(ctx, key).Result()
		if err == nil && len(vals) > 0 {
			finalAuction, _ = parseAuction(vals)
		}
	}

	// Update Redis state
	pipe := r.rdb.Pipeline()
	pipe.HSet(ctx, key, "status", "CLOSED")
	pipe.SRem(ctx, activeSetKey, auctionID)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("close auction: %w", err)
	}

	// Persist final state to DynamoDB
	if r.db != nil && finalAuction != nil {
		finalAuction.Status = "CLOSED"
		if err := r.saveToDynamo(ctx, finalAuction); err != nil {
			log.Printf("WARN: close auction dynamo write failed for %s: %v", auctionID, err)
		}
	}

	// Delete the Redis hash to reclaim memory (closed auctions don't need hot cache)
	r.rdb.Del(ctx, key)

	return nil
}

// GetRedisClient returns the underlying Redis client (needed by concurrency strategies).
func (r *Repository) GetRedisClient() *redis.Client {
	return r.rdb
}

// ── DynamoDB helpers ────────────────────────────────────────────────────────

func (r *Repository) saveToDynamo(ctx context.Context, a *Auction) error {
	item, err := attributevalue.MarshalMap(toDynamo(a))
	if err != nil {
		return fmt.Errorf("marshal auction: %w", err)
	}
	_, err = r.db.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(auctionsTable),
		Item:      item,
	})
	return err
}

func (r *Repository) getFromDynamo(ctx context.Context, auctionID string) (*Auction, error) {
	out, err := r.db.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(auctionsTable),
		Key: map[string]ddbTypes.AttributeValue{
			"auction_id": &ddbTypes.AttributeValueMemberS{Value: auctionID},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("dynamo get auction: %w", err)
	}
	if out.Item == nil {
		return nil, errors.New("auction not found")
	}
	var d dynamoAuction
	if err := attributevalue.UnmarshalMap(out.Item, &d); err != nil {
		return nil, fmt.Errorf("unmarshal auction: %w", err)
	}
	return fromDynamo(d), nil
}

// ── Redis hash helpers ──────────────────────────────────────────────────────

func auctionToRedisMap(a *Auction) map[string]interface{} {
	return map[string]interface{}{
		"auction_id":      a.AuctionID,
		"seller_id":       a.SellerID,
		"item_id":         a.ItemID,
		"item_title":      a.ItemTitle,
		"shop_id":         a.ShopID,
		"shop_name":       a.ShopName,
		"retail_price":    a.RetailPrice,
		"max_price":       a.MaxPrice,
		"image_url":       a.ImageURL,
		"shop_logo_url":   a.ShopLogoURL,
		"description":     a.Description,
		"category":        a.Category,
		"start_time":      a.StartTime.Format(time.RFC3339),
		"end_time":        a.EndTime.Format(time.RFC3339),
		"current_highest": a.CurrentHighest,
		"bid_count":       a.BidCount,
		"highest_bidder":  a.HighestBidder,
		"status":          a.Status,
		"version":         a.Version,
	}
}

func parseAuction(vals map[string]string) (*Auction, error) {
	startTime, _ := time.Parse(time.RFC3339, vals["start_time"])
	endTime, _ := time.Parse(time.RFC3339, vals["end_time"])
	currentHighest, _ := strconv.ParseInt(vals["current_highest"], 10, 64)
	version, _ := strconv.ParseInt(vals["version"], 10, 64)
	bidCount, _ := strconv.ParseInt(vals["bid_count"], 10, 64)
	retailPrice, _ := strconv.ParseInt(vals["retail_price"], 10, 64)
	maxPrice, _ := strconv.ParseInt(vals["max_price"], 10, 64)

	return &Auction{
		AuctionID:      vals["auction_id"],
		SellerID:       vals["seller_id"],
		ItemID:         vals["item_id"],
		ItemTitle:      vals["item_title"],
		ShopID:         vals["shop_id"],
		ShopName:       vals["shop_name"],
		RetailPrice:    retailPrice,
		MaxPrice:       maxPrice,
		ImageURL:       vals["image_url"],
		ShopLogoURL:    vals["shop_logo_url"],
		Description:    vals["description"],
		Category:       vals["category"],
		StartTime:      startTime,
		EndTime:        endTime,
		CurrentHighest: currentHighest,
		BidCount:       bidCount,
		HighestBidder:  vals["highest_bidder"],
		Status:         vals["status"],
		Version:        version,
	}, nil
}
