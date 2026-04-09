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

// Start begins listening for bid_placed and auction_closed events.
func (c *Consumer) Start() {
	go c.subscribeBidPlaced()
	go c.subscribeAuctionClosed()
	log.Println("bid event consumer started")
}

// Stop signals the consumer to shut down.
func (c *Consumer) Stop() {
	close(c.done)
	log.Println("bid event consumer stopped")
}

func (c *Consumer) subscribeBidPlaced() {
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

func (c *Consumer) subscribeAuctionClosed() {
	sub := c.rdb.Subscribe(context.Background(), "auction_closed")
	defer sub.Close()

	ch := sub.Channel()
	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return
			}
			c.handleAuctionClosed(msg.Payload)
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
		ItemTitle: event.ItemTitle,
		ShopName:  event.ShopName,
		Amount:    event.Amount,
		Timestamp: ts,
		Status:    "ACCEPTED",
	}

	ctx := context.Background()
	if err := c.bidSvc.RecordBid(ctx, b); err != nil {
		log.Printf("failed to record bid from event: %v", err)
	}
}

func (c *Consumer) handleAuctionClosed(payload string) {
	var event events.AuctionClosedEvent
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		log.Printf("failed to unmarshal auction_closed event: %v", err)
		return
	}

	// Build list of all winners (multi-winner auctions have a Winners map).
	winners := make(map[string]struct{})
	for winnerID := range event.Winners {
		winners[winnerID] = struct{}{}
	}
	// Backwards compatible: use WinnerID if Winners map is empty.
	if len(winners) == 0 && event.WinnerID != "" {
		winners[event.WinnerID] = struct{}{}
	}

	if len(winners) == 0 {
		log.Printf("auction %s closed with no winner", event.AuctionID)
		return
	}

	ctx := context.Background()
	for winnerID := range winners {
		if err := c.bidSvc.MarkWinnerBid(ctx, event.AuctionID, winnerID); err != nil {
			log.Printf("failed to mark winner bid for auction %s, winner %s: %v", event.AuctionID, winnerID, err)
		} else {
			log.Printf("marked winning bid for auction %s, winner %s", event.AuctionID, winnerID)
		}
	}
}
