package events

import (
	"context"
	"encoding/json"
	"log"

	"github.com/redis/go-redis/v9"
)

// PaymentInitiator is implemented by the payment service to handle auction closed events.
type PaymentInitiator interface {
	InitiatePayment(ctx context.Context, event AuctionClosedEvent) error
}

// Consumer subscribes to auction_closed events and triggers payment initiation.
type Consumer struct {
	rdb     *redis.Client
	svc     PaymentInitiator
	stopCh  chan struct{}
}

// NewConsumer creates a new Consumer.
func NewConsumer(rdb *redis.Client, svc PaymentInitiator) *Consumer {
	return &Consumer{
		rdb:    rdb,
		svc:    svc,
		stopCh: make(chan struct{}),
	}
}

// Start begins listening for auction_closed events in a background goroutine.
func (c *Consumer) Start() {
	go c.run()
}

// Stop signals the consumer to stop.
func (c *Consumer) Stop() {
	close(c.stopCh)
}

func (c *Consumer) run() {
	ctx := context.Background()
	pubsub := c.rdb.Subscribe(ctx, ChannelAuctionClosed)
	defer pubsub.Close()

	ch := pubsub.Channel()
	log.Printf("payment consumer: listening on channel %q", ChannelAuctionClosed)

	for {
		select {
		case <-c.stopCh:
			log.Println("payment consumer: stopped")
			return
		case msg, ok := <-ch:
			if !ok {
				log.Println("payment consumer: channel closed")
				return
			}
			var event AuctionClosedEvent
			if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
				log.Printf("payment consumer: unmarshal error: %v", err)
				continue
			}
			if err := c.svc.InitiatePayment(ctx, event); err != nil {
				log.Printf("payment consumer: initiate payment error for auction %s: %v", event.AuctionID, err)
			}
		}
	}
}
