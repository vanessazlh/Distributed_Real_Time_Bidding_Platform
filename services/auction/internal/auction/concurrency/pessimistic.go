package concurrency

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// Pessimistic implements pessimistic locking using Redis SETNX.
type Pessimistic struct {
	rdb *redis.Client
}

// NewPessimistic creates a new Pessimistic controller.
func NewPessimistic(rdb *redis.Client) *Pessimistic {
	return &Pessimistic{rdb: rdb}
}

func lockKey(auctionID string) string { return "lock:auction:" + auctionID }

// TryPlaceBid attempts to place a bid using pessimistic locking.
func (p *Pessimistic) TryPlaceBid(ctx context.Context, auctionID string, amount int64, bidderID string) (int64, error) {
	lk := lockKey(auctionID)
	maxRetries := 10
	lockTTL := 500 * time.Millisecond

	// Acquire lock
	for i := 0; i < maxRetries; i++ {
		ok, err := p.rdb.SetNX(ctx, lk, "1", lockTTL).Result()
		if err != nil {
			return 0, fmt.Errorf("acquire lock: %w", err)
		}
		if ok {
			break
		}
		if i == maxRetries-1 {
			return 0, errors.New("pessimistic lock: failed to acquire lock")
		}
		time.Sleep(50 * time.Millisecond)
	}
	defer p.rdb.Del(ctx, lk)

	// Read and validate
	key := "auction:" + auctionID
	vals, err := p.rdb.HGetAll(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("get auction: %w", err)
	}
	if len(vals) == 0 {
		return 0, errors.New("auction not found")
	}
	if vals["status"] != "OPEN" {
		return 0, errors.New("auction is not open")
	}

	currentHighest, _ := strconv.ParseInt(vals["current_highest"], 10, 64)
	if amount <= currentHighest {
		return 0, fmt.Errorf("bid amount %d must be higher than current highest %d", amount, currentHighest)
	}

	version, _ := strconv.ParseInt(vals["version"], 10, 64)
	newVersion := version + 1

	// Update
	err = p.rdb.HSet(ctx, key, map[string]interface{}{
		"current_highest": amount,
		"highest_bidder":  bidderID,
		"version":         newVersion,
	}).Err()
	if err != nil {
		return 0, fmt.Errorf("update auction: %w", err)
	}

	return newVersion, nil
}
