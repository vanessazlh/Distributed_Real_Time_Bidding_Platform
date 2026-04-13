package events

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
	"rtb/shared/events"
)

const (
	StreamBidPlaced     = "bid:placed"
	StreamAuctionClosed = "auction:closed"
)

// Publisher publishes domain events to Redis Streams.
type Publisher struct {
	rdb *redis.Client
}

// NewPublisher creates a new Publisher.
func NewPublisher(rdb *redis.Client) *Publisher {
	return &Publisher{rdb: rdb}
}

// PublishBidPlaced publishes a bid-placed event to the bid:placed stream.
func (p *Publisher) PublishBidPlaced(ctx context.Context, event events.BidPlacedEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal bid placed event: %w", err)
	}
	return p.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: StreamBidPlaced,
		Values: map[string]interface{}{"payload": string(data)},
	}).Err()
}

// PublishAuctionClosed publishes an auction-closed event to the auction:closed stream.
func (p *Publisher) PublishAuctionClosed(ctx context.Context, event events.AuctionClosedEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal auction closed event: %w", err)
	}
	return p.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: StreamAuctionClosed,
		Values: map[string]interface{}{"payload": string(data)},
	}).Err()
}
