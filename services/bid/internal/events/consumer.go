package events

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
	"rtb/services/bid/internal/bid"
	"rtb/shared/events"
)

// Consumer subscribes to Redis Pub/Sub channels and records bids.
type Consumer struct {
	rdb    *redis.Client
	bidSvc *bid.Service
	done   chan struct{}
}

// NewConsumer creates a new event consumer.
func NewConsumer(rdb *redis.Client, bidSvc *bid.Service) *Consumer {
	return &Consumer{
		rdb:    rdb,
		bidSvc: bidSvc,
		done:   make(chan struct{}),
	}
}

// Start begins listening for bid_placed events.
func (c *Consumer) Start() {
	go c.subscribe()
	log.Println("bid event consumer started")
}

// Stop signals the consumer to shut down.
func (c *Consumer) Stop() {
	close(c.done)
	log.Println("bid event consumer stopped")
}

func (c *Consumer) subscribe() {
	sub := c.rdb.Subscribe(context.Background(), "bid_placed")
	defer sub.Close()

	ch := sub.Channel()
	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return
			}
			c.handleBidPlaced(msg.Payload)
		case <-c.done:
			return
		}
	}
}

func (c *Consumer) handleBidPlaced(payload string) {
	var event events.BidPlacedEvent
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		log.Printf("failed to unmarshal bid_placed event: %v", err)
		return
	}

	ts, err := time.Parse(time.RFC3339Nano, event.Timestamp)
	if err != nil {
		ts = time.Now().UTC()
	}

	b := &bid.Bid{
		BidID:     event.BidID,
		AuctionID: event.AuctionID,
		UserID:    event.UserID,
		Amount:    event.Amount,
		Timestamp: ts,
		Status:    "ACCEPTED",
	}

	ctx := context.Background()
	if err := c.bidSvc.RecordBid(ctx, b); err != nil {
		log.Printf("failed to record bid from event: %v", err)
	}
}
