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

// closeAndReadWinnerLua atomically sets status=CLOSED and reads the winner(s).
// Prevents a race where a new bid changes the winner between reading and closing.
//
// Supports both single-winner (quantity=1) and multi-winner (quantity>1) auctions.
// For quantity>1, returns all winners as comma-separated "bidder:amount" pairs.
//
// KEYS[1] = auction:{auctionID} (hash)
// KEYS[2] = auction:{auctionID}:bids (sorted set)
//
// Returns:
//
//	quantity=1:  {status_string, winner_id, winning_bid_string}
//	quantity>1: {'OK_MULTI', 'bidder1:amt1,bidder2:amt2,...', quantity_string}
var closeAndReadWinnerLua = redis.NewScript(`
local hashKey = KEYS[1]
local bidsKey = KEYS[2]

local status = redis.call('HGET', hashKey, 'status')
if status == false then
	return {'ERR_NOT_FOUND', '', '0'}
end
if status ~= 'OPEN' then
	return {'ERR_NOT_OPEN', '', '0'}
end

redis.call('HSET', hashKey, 'status', 'CLOSED')

local quantity = tonumber(redis.call('HGET', hashKey, 'quantity')) or 1

if quantity <= 1 then
	-- Single-winner: original logic
	local top = redis.call('ZREVRANGE', bidsKey, 0, 0, 'WITHSCORES')
	if #top >= 2 then
		return {'OK', top[1], top[2]}
	end
	local bidder = redis.call('HGET', hashKey, 'highest_bidder') or ''
	local amount = redis.call('HGET', hashKey, 'current_highest') or '0'
	return {'OK', bidder, amount}
end

-- Multi-winner: return top-N from sorted set
local top = redis.call('ZREVRANGE', bidsKey, 0, quantity - 1, 'WITHSCORES')
if #top == 0 then
	return {'OK_MULTI', '', tostring(quantity)}
end

local pairs = {}
for i = 1, #top, 2 do
	pairs[#pairs + 1] = top[i] .. ':' .. top[i + 1]
end

return {'OK_MULTI', table.concat(pairs, ','), tostring(quantity)}
`)

// rollbackCloseLua sets status back to OPEN if it is currently CLOSED.
// Used when event publishing fails and the close must be retried.
//
// KEYS[1] = auction:{auctionID}
var rollbackCloseLua = redis.NewScript(`
local hashKey = KEYS[1]
local status = redis.call('HGET', hashKey, 'status')
if status == 'CLOSED' then
	redis.call('HSET', hashKey, 'status', 'OPEN')
	return 1
end
return 0
`)

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
	Quantity       int    `dynamodbav:"quantity"`
	ImageURL       string `dynamodbav:"image_url"`
	ShopLogoURL    string `dynamodbav:"shop_logo_url"`
	Description    string `dynamodbav:"description"`
	Category       string `dynamodbav:"category,omitempty"`
	StartTime      string `dynamodbav:"start_time"`
	EndTime        string `dynamodbav:"end_time"`
	CurrentHighest int64  `dynamodbav:"current_highest"`
	BidCount       int64  `dynamodbav:"bid_count"`
	HighestBidder  string `dynamodbav:"highest_bidder"`
	Status         string           `dynamodbav:"status"`
	Version        int64            `dynamodbav:"version"`
	Winners        map[string]int64 `dynamodbav:"winners"`
}

func toDynamo(a *Auction) dynamoAuction {
	winners := a.Winners
	if winners == nil {
		winners = map[string]int64{}
	}
	return dynamoAuction{
		AuctionID:      a.AuctionID,
		SellerID:       a.SellerID,
		ItemID:         a.ItemID,
		ItemTitle:      a.ItemTitle,
		ShopID:         a.ShopID,
		ShopName:       a.ShopName,
		RetailPrice:    a.RetailPrice,
		MaxPrice:       a.MaxPrice,
		Quantity:       a.Quantity,
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
		Winners:        winners,
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
		Quantity:       d.Quantity,
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
		Winners:        d.Winners,
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
func auctionBidsKey(id string) string      { return "auction:" + id + ":bids" }
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

// AtomicCloseAndReadWinner atomically sets status=CLOSED and reads the winner
// from the Redis ZSET (fallback: hash highest_bidder). Returns winnerID and
// winningBid. If the auction is not OPEN, returns an error.
// For single-winner auctions (quantity=1).
func (r *Repository) AtomicCloseAndReadWinner(ctx context.Context, auctionID string) (string, int64, error) {
	hashKey := auctionKey(auctionID)
	bidsKey := auctionBidsKey(auctionID)

	res, err := closeAndReadWinnerLua.Run(ctx, r.rdb, []string{hashKey, bidsKey}).StringSlice()
	if err != nil {
		return "", 0, fmt.Errorf("atomic close lua: %w", err)
	}
	switch res[0] {
	case "ERR_NOT_FOUND":
		return "", 0, errors.New("auction not found")
	case "ERR_NOT_OPEN":
		return "", 0, errors.New("auction is not open")
	case "OK_MULTI":
		// Multi-winner auction closed via single-winner path — parse first winner
		winners := parseWinnerPairs(res[1])
		for id, amt := range winners {
			return id, amt, nil
		}
		return "", 0, nil
	}
	amount, _ := strconv.ParseInt(res[2], 10, 64)
	return res[1], amount, nil
}

// AtomicCloseAndReadWinners atomically sets status=CLOSED and reads all winners
// for a quantity auction. Returns a map of bidderID → bid amount.
func (r *Repository) AtomicCloseAndReadWinners(ctx context.Context, auctionID string) (map[string]int64, error) {
	hashKey := auctionKey(auctionID)
	bidsKey := auctionBidsKey(auctionID)

	res, err := closeAndReadWinnerLua.Run(ctx, r.rdb, []string{hashKey, bidsKey}).StringSlice()
	if err != nil {
		return nil, fmt.Errorf("atomic close lua: %w", err)
	}
	switch res[0] {
	case "ERR_NOT_FOUND":
		return nil, errors.New("auction not found")
	case "ERR_NOT_OPEN":
		return nil, errors.New("auction is not open")
	case "OK":
		// Single-winner result
		winners := make(map[string]int64)
		if res[1] != "" {
			amount, _ := strconv.ParseInt(res[2], 10, 64)
			winners[res[1]] = amount
		}
		return winners, nil
	case "OK_MULTI":
		return parseWinnerPairs(res[1]), nil
	}
	return nil, fmt.Errorf("unexpected close result: %s", res[0])
}

// parseWinnerPairs parses a comma-separated "bidder:amount" string into a map.
func parseWinnerPairs(encoded string) map[string]int64 {
	winners := make(map[string]int64)
	if encoded == "" {
		return winners
	}
	for _, pair := range splitString(encoded, ',') {
		idx := lastIndexByte(pair, ':')
		if idx < 0 {
			continue
		}
		bidder := pair[:idx]
		amount, _ := strconv.ParseInt(pair[idx+1:], 10, 64)
		winners[bidder] = amount
	}
	return winners
}

func splitString(s string, sep byte) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	parts = append(parts, s[start:])
	return parts
}

func lastIndexByte(s string, c byte) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == c {
			return i
		}
	}
	return -1
}

// RollbackClose sets the auction status back to OPEN. Called when event
// publishing fails so the closer can retry on the next tick.
func (r *Repository) RollbackClose(ctx context.Context, auctionID string) error {
	_, err := rollbackCloseLua.Run(ctx, r.rdb, []string{auctionKey(auctionID)}).Result()
	if err != nil {
		return fmt.Errorf("rollback close: %w", err)
	}
	return nil
}

// PersistClosedState writes the final CLOSED state to DynamoDB.
// Called after the event has been published successfully.
func (r *Repository) PersistClosedState(ctx context.Context, auctionID string) error {
	if r.db == nil {
		return nil
	}
	vals, err := r.rdb.HGetAll(ctx, auctionKey(auctionID)).Result()
	if err == nil && len(vals) > 0 {
		a, _ := parseAuction(vals)
		a.Status = "CLOSED"
		return r.saveToDynamo(ctx, a)
	}
	// Redis data unavailable — update just the status in DynamoDB
	_, err = r.db.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(auctionsTable),
		Key: map[string]ddbTypes.AttributeValue{
			"auction_id": &ddbTypes.AttributeValueMemberS{Value: auctionID},
		},
		UpdateExpression:         aws.String("SET #s = :s"),
		ExpressionAttributeNames: map[string]string{"#s": "status"},
		ExpressionAttributeValues: map[string]ddbTypes.AttributeValue{
			":s": &ddbTypes.AttributeValueMemberS{Value: "CLOSED"},
		},
	})
	return err
}

// CleanupRedis removes the auction from the active set and deletes the
// Redis hash and bids ZSET to reclaim memory.
func (r *Repository) CleanupRedis(ctx context.Context, auctionID string) error {
	pipe := r.rdb.Pipeline()
	pipe.SRem(ctx, activeSetKey, auctionID)
	pipe.Del(ctx, auctionKey(auctionID))
	pipe.Del(ctx, auctionBidsKey(auctionID))
	_, err := pipe.Exec(ctx)
	return err
}

// GetDynamoWinners resolves the winner from DynamoDB using a two-level fallback:
// 1. winners map (most accurate — updated on every bid)
// 2. highestBidder field (last-resort)
func (r *Repository) GetDynamoWinners(ctx context.Context, auctionID string) (string, int64, error) {
	if r.db == nil {
		return "", 0, errors.New("dynamo not configured")
	}
	out, err := r.db.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(auctionsTable),
		Key: map[string]ddbTypes.AttributeValue{
			"auction_id": &ddbTypes.AttributeValueMemberS{Value: auctionID},
		},
	})
	if err != nil {
		return "", 0, fmt.Errorf("dynamo get winners: %w", err)
	}
	if out.Item == nil {
		return "", 0, errors.New("auction not found in dynamo")
	}

	// Level 1: winners map
	if winnersAttr, ok := out.Item["winners"]; ok {
		if m, ok := winnersAttr.(*ddbTypes.AttributeValueMemberM); ok && len(m.Value) > 0 {
			var topBidder string
			var topAmount int64
			for bidder, amtAttr := range m.Value {
				if n, ok := amtAttr.(*ddbTypes.AttributeValueMemberN); ok {
					amt, _ := strconv.ParseInt(n.Value, 10, 64)
					if amt > topAmount {
						topAmount = amt
						topBidder = bidder
					}
				}
			}
			if topBidder != "" {
				return topBidder, topAmount, nil
			}
		}
	}

	// Level 2: highestBidder field
	var d dynamoAuction
	if err := attributevalue.UnmarshalMap(out.Item, &d); err != nil {
		return "", 0, fmt.Errorf("unmarshal auction: %w", err)
	}
	return d.HighestBidder, d.CurrentHighest, nil
}

// GetDynamoAllWinners returns all winners from DynamoDB's winners map.
// Used as a fallback for multi-winner auctions when Redis data is unavailable.
func (r *Repository) GetDynamoAllWinners(ctx context.Context, auctionID string) (map[string]int64, error) {
	if r.db == nil {
		return nil, errors.New("dynamo not configured")
	}
	out, err := r.db.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(auctionsTable),
		Key: map[string]ddbTypes.AttributeValue{
			"auction_id": &ddbTypes.AttributeValueMemberS{Value: auctionID},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("dynamo get all winners: %w", err)
	}
	if out.Item == nil {
		return nil, errors.New("auction not found in dynamo")
	}

	winners := make(map[string]int64)
	if winnersAttr, ok := out.Item["winners"]; ok {
		if m, ok := winnersAttr.(*ddbTypes.AttributeValueMemberM); ok {
			for bidder, amtAttr := range m.Value {
				if n, ok := amtAttr.(*ddbTypes.AttributeValueMemberN); ok {
					amt, _ := strconv.ParseInt(n.Value, 10, 64)
					winners[bidder] = amt
				}
			}
		}
	}

	// If winners map is empty, fall back to highest_bidder
	if len(winners) == 0 {
		var d dynamoAuction
		if err := attributevalue.UnmarshalMap(out.Item, &d); err == nil && d.HighestBidder != "" {
			winners[d.HighestBidder] = d.CurrentHighest
		}
	}

	return winners, nil
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

// UpdateDynamoWinner updates the winners map in DynamoDB for a given auction.
// Called asynchronously after each successful bid. Non-fatal on failure.
func (r *Repository) UpdateDynamoWinner(ctx context.Context, auctionID string, bidderID string, amount int64) {
	if r.db == nil {
		return
	}
	amountStr := strconv.FormatInt(amount, 10)
	_, err := r.db.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(auctionsTable),
		Key: map[string]ddbTypes.AttributeValue{
			"auction_id": &ddbTypes.AttributeValueMemberS{Value: auctionID},
		},
		UpdateExpression:         aws.String("SET winners.#bidder = :amount"),
		ExpressionAttributeNames: map[string]string{"#bidder": bidderID},
		ExpressionAttributeValues: map[string]ddbTypes.AttributeValue{
			":amount": &ddbTypes.AttributeValueMemberN{Value: amountStr},
		},
	})
	if err != nil {
		// winners map may not exist for legacy auctions — initialize it
		initMap := map[string]ddbTypes.AttributeValue{
			bidderID: &ddbTypes.AttributeValueMemberN{Value: amountStr},
		}
		_, err2 := r.db.UpdateItem(ctx, &dynamodb.UpdateItemInput{
			TableName: aws.String(auctionsTable),
			Key: map[string]ddbTypes.AttributeValue{
				"auction_id": &ddbTypes.AttributeValueMemberS{Value: auctionID},
			},
			UpdateExpression: aws.String("SET winners = :w"),
			ExpressionAttributeValues: map[string]ddbTypes.AttributeValue{
				":w": &ddbTypes.AttributeValueMemberM{Value: initMap},
			},
		})
		if err2 != nil {
			log.Printf("WARN: update dynamo winner for %s failed: %v", auctionID, err2)
		}
	}
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
		"quantity":         a.Quantity,
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
	quantity, _ := strconv.Atoi(vals["quantity"])
	if quantity < 1 {
		quantity = 1
	}

	return &Auction{
		AuctionID:      vals["auction_id"],
		SellerID:       vals["seller_id"],
		ItemID:         vals["item_id"],
		ItemTitle:      vals["item_title"],
		ShopID:         vals["shop_id"],
		ShopName:       vals["shop_name"],
		RetailPrice:    retailPrice,
		MaxPrice:       maxPrice,
		Quantity:       quantity,
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
