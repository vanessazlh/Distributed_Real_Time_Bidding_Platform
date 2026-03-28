package events

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
)

const (
	channelBidPlaced     = "bid_placed"
	channelAuctionClosed = "auction_closed"
)

// Publisher publishes domain events to Redis Pub/Sub.
type Publisher struct {
	rdb *redis.Client
}

// NewPublisher creates a new Publisher.
func NewPublisher(rdb *redis.Client) *Publisher {
	return &Publisher{rdb: rdb}
}

// PublishBidPlaced publishes a bid-placed event.
func (p *Publisher) PublishBidPlaced(ctx context.Context, event BidPlacedEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal bid placed event: %w", err)
	}
	return p.rdb.Publish(ctx, channelBidPlaced, data).Err()
}

// PublishAuctionClosed publishes an auction-closed event.
func (p *Publisher) PublishAuctionClosed(ctx context.Context, event AuctionClosedEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal auction closed event: %w", err)
	}
	return p.rdb.Publish(ctx, channelAuctionClosed, data).Err()
}
