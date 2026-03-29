package concurrency

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// Optimistic implements optimistic locking using Redis WATCH/MULTI/EXEC.
type Optimistic struct {
	rdb *redis.Client
}

// NewOptimistic creates a new Optimistic controller.
func NewOptimistic(rdb *redis.Client) *Optimistic {
	return &Optimistic{rdb: rdb}
}

// TryPlaceBid attempts to place a bid using optimistic locking.
// Returns the new version on success.
func (o *Optimistic) TryPlaceBid(ctx context.Context, auctionID string, amount int64, bidderID string) (int64, error) {
	key := "auction:" + auctionID
	maxRetries := 3

	for i := 0; i < maxRetries; i++ {
		newVersion, err := o.tryOnce(ctx, key, amount, bidderID)
		if err == nil {
			return newVersion, nil
		}
		if errors.Is(err, redis.TxFailedErr) {
			// Optimistic lock conflict — retry with exponential backoff
			backoff := time.Duration(math.Pow(2, float64(i))) * 10 * time.Millisecond
			time.Sleep(backoff)
			continue
		}
		return 0, err
	}
	return 0, errors.New("optimistic lock: max retries exceeded")
}

func (o *Optimistic) tryOnce(ctx context.Context, key string, amount int64, bidderID string) (int64, error) {
	var newVersion int64

	err := o.rdb.Watch(ctx, func(tx *redis.Tx) error {
		vals, err := tx.HGetAll(ctx, key).Result()
		if err != nil {
			return fmt.Errorf("get auction: %w", err)
		}
		if len(vals) == 0 {
			return errors.New("auction not found")
		}
		if vals["status"] != "OPEN" {
			return errors.New("auction is not open")
		}

		currentHighest, _ := strconv.ParseInt(vals["current_highest"], 10, 64)
		if amount <= currentHighest {
			return fmt.Errorf("bid amount %d must be higher than current highest %d", amount, currentHighest)
		}

		version, _ := strconv.ParseInt(vals["version"], 10, 64)
		newVersion = version + 1

		_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			pipe.HSet(ctx, key, map[string]interface{}{
				"current_highest": amount,
				"highest_bidder":  bidderID,
				"version":         newVersion,
			})
			return nil
		})
		return err
	}, key)

	return newVersion, err
}
