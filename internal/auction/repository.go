package auction

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// Repository handles Redis operations for auctions.
type Repository struct {
	rdb *redis.Client
}

// NewRepository creates a new Repository.
func NewRepository(rdb *redis.Client) *Repository {
	return &Repository{rdb: rdb}
}

func auctionKey(id string) string { return "auction:" + id }

const activeSetKey = "auctions:active"

// Create stores a new auction in Redis.
func (r *Repository) Create(ctx context.Context, a *Auction) error {
	key := auctionKey(a.AuctionID)
	pipe := r.rdb.Pipeline()
	pipe.HSet(ctx, key, map[string]interface{}{
		"auction_id":      a.AuctionID,
		"item_id":         a.ItemID,
		"shop_id":         a.ShopID,
		"start_time":      a.StartTime.Format(time.RFC3339),
		"end_time":        a.EndTime.Format(time.RFC3339),
		"current_highest": a.CurrentHighest,
		"highest_bidder":  a.HighestBidder,
		"status":          a.Status,
		"version":         a.Version,
	})
	pipe.SAdd(ctx, activeSetKey, a.AuctionID)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("create auction: %w", err)
	}
	return nil
}

// GetByID retrieves an auction by ID.
func (r *Repository) GetByID(ctx context.Context, auctionID string) (*Auction, error) {
	vals, err := r.rdb.HGetAll(ctx, auctionKey(auctionID)).Result()
	if err != nil {
		return nil, fmt.Errorf("get auction: %w", err)
	}
	if len(vals) == 0 {
		return nil, errors.New("auction not found")
	}
	return parseAuction(vals)
}

// List returns auctions filtered by status. If status is empty, returns all active auctions.
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

// UpdateHighestBid atomically updates the highest bid using optimistic locking.
func (r *Repository) UpdateHighestBid(ctx context.Context, auctionID string, amount int64, bidderID string, expectedVersion int64) error {
	key := auctionKey(auctionID)

	// Use a transaction with WATCH for optimistic locking
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
			return nil
		})
		return err
	}, key)

	if err != nil {
		return fmt.Errorf("update highest bid: %w", err)
	}
	return nil
}

// Close marks an auction as CLOSED and removes it from the active set.
func (r *Repository) Close(ctx context.Context, auctionID string) error {
	key := auctionKey(auctionID)
	pipe := r.rdb.Pipeline()
	pipe.HSet(ctx, key, "status", "CLOSED")
	pipe.SRem(ctx, activeSetKey, auctionID)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("close auction: %w", err)
	}
	return nil
}

// GetRedisClient returns the underlying Redis client (needed by concurrency strategies).
func (r *Repository) GetRedisClient() *redis.Client {
	return r.rdb
}

func parseAuction(vals map[string]string) (*Auction, error) {
	startTime, _ := time.Parse(time.RFC3339, vals["start_time"])
	endTime, _ := time.Parse(time.RFC3339, vals["end_time"])
	currentHighest, _ := strconv.ParseInt(vals["current_highest"], 10, 64)
	version, _ := strconv.ParseInt(vals["version"], 10, 64)

	return &Auction{
		AuctionID:      vals["auction_id"],
		ItemID:         vals["item_id"],
		ShopID:         vals["shop_id"],
		StartTime:      startTime,
		EndTime:        endTime,
		CurrentHighest: currentHighest,
		HighestBidder:  vals["highest_bidder"],
		Status:         vals["status"],
		Version:        version,
	}, nil
}
