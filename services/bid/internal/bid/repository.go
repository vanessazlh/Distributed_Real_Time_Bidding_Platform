package bid

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Repository handles Redis operations for bids.
type Repository struct {
	rdb *redis.Client
}

// NewRepository creates a new Repository.
func NewRepository(rdb *redis.Client) *Repository {
	return &Repository{rdb: rdb}
}

func auctionBidsKey(auctionID string) string { return "bids:auction:" + auctionID }
func userBidsKey(userID string) string        { return "bids:user:" + userID }
func bidDetailKey(bidID string) string         { return "bid:" + bidID }

// Create stores a bid in Redis.
func (r *Repository) Create(ctx context.Context, b *Bid) error {
	data, err := json.Marshal(b)
	if err != nil {
		return fmt.Errorf("marshal bid: %w", err)
	}

	pipe := r.rdb.Pipeline()
	score := float64(b.Timestamp.UnixMilli())

	// Store bid detail
	pipe.Set(ctx, bidDetailKey(b.BidID), data, 0)
	// Add to auction sorted set
	pipe.ZAdd(ctx, auctionBidsKey(b.AuctionID), redis.Z{Score: score, Member: b.BidID})
	// Add to user sorted set
	pipe.ZAdd(ctx, userBidsKey(b.UserID), redis.Z{Score: score, Member: b.BidID})

	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("save bid: %w", err)
	}
	return nil
}

// GetByAuction returns all bids for an auction, ordered by timestamp.
func (r *Repository) GetByAuction(ctx context.Context, auctionID string) ([]*Bid, error) {
	return r.getBidsByKey(ctx, auctionBidsKey(auctionID))
}

// GetByUser returns all bids placed by a user, ordered by timestamp.
func (r *Repository) GetByUser(ctx context.Context, userID string) ([]*Bid, error) {
	return r.getBidsByKey(ctx, userBidsKey(userID))
}

func (r *Repository) getBidsByKey(ctx context.Context, key string) ([]*Bid, error) {
	ids, err := r.rdb.ZRevRange(ctx, key, 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("get bid ids: %w", err)
	}
	if len(ids) == 0 {
		return []*Bid{}, nil
	}

	keys := make([]string, len(ids))
	for i, id := range ids {
		keys[i] = bidDetailKey(id)
	}

	vals, err := r.rdb.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, fmt.Errorf("get bid details: %w", err)
	}

	bids := make([]*Bid, 0, len(vals))
	for _, v := range vals {
		if v == nil {
			continue
		}
		var b Bid
		if err := json.Unmarshal([]byte(v.(string)), &b); err != nil {
			return nil, fmt.Errorf("unmarshal bid: %w", err)
		}
		bids = append(bids, &b)
	}
	return bids, nil
}

// MarkOutbid marks all ACCEPTED bids for an auction as OUTBID, except excludeBidID.
func (r *Repository) MarkOutbid(ctx context.Context, auctionID string, excludeBidID string) error {
	ids, err := r.rdb.ZRange(ctx, auctionBidsKey(auctionID), 0, -1).Result()
	if err != nil {
		return fmt.Errorf("get bid ids for outbid: %w", err)
	}

	for _, id := range ids {
		if id == excludeBidID {
			continue
		}
		raw, err := r.rdb.Get(ctx, bidDetailKey(id)).Result()
		if err != nil {
			continue
		}
		var b Bid
		if err := json.Unmarshal([]byte(raw), &b); err != nil {
			continue
		}
		if b.Status == "ACCEPTED" {
			b.Status = "OUTBID"
			b.Timestamp = time.Now().UTC()
			data, _ := json.Marshal(b)
			r.rdb.Set(ctx, bidDetailKey(id), data, 0)
		}
	}
	return nil
}
