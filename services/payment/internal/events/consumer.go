package events

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	StreamAuctionClosed = "auction:closed"
	consumerGroup       = "payment-service"
	pendingTimeout      = 60 * time.Second
	reclaimInterval     = 30 * time.Second
)

// PaymentInitiator is implemented by the payment service to handle auction closed events.
type PaymentInitiator interface {
	InitiatePayment(ctx context.Context, event AuctionClosedEvent) error
}

// Consumer reads from the auction:closed Redis Stream using a consumer group,
// allowing multiple instances to process events without duplication.
type Consumer struct {
	rdb        *redis.Client
	svc        PaymentInitiator
	consumerID string
	stopCh     chan struct{}
	numWorkers int
}

// NewConsumer creates a new Consumer. numWorkers controls how many payments can
// be processed concurrently; pass 0 to use the default of 10.
func NewConsumer(rdb *redis.Client, svc PaymentInitiator, numWorkers int) *Consumer {
	if numWorkers <= 0 {
		numWorkers = 10
	}
	return &Consumer{
		rdb:        rdb,
		svc:        svc,
		consumerID: uuid.New().String(),
		stopCh:     make(chan struct{}),
		numWorkers: numWorkers,
	}
}

// Start begins consuming from the stream in a background goroutine.
func (c *Consumer) Start() {
	go c.run()
}

// Stop signals the consumer to stop and waits for in-flight payments to finish.
func (c *Consumer) Stop() {
	close(c.stopCh)
}

func (c *Consumer) run() {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-c.stopCh
		cancel()
	}()

	// Create the consumer group if it doesn't exist.
	// "$" means only consume messages published after this point — no history replay.
	if err := c.rdb.XGroupCreateMkStream(ctx, StreamAuctionClosed, consumerGroup, "$").Err(); err != nil {
		if !isGroupExistsErr(err) {
			log.Fatalf("payment consumer: create consumer group: %v", err)
		}
	}

	log.Printf("payment consumer: stream=%q group=%q consumer=%q workers=%d",
		StreamAuctionClosed, consumerGroup, c.consumerID, c.numWorkers)

	sem := make(chan struct{}, c.numWorkers)
	var wg sync.WaitGroup

	// Periodically reclaim messages that have been pending too long
	// (i.e. delivered to a now-dead consumer instance that never ACKed).
	go func() {
		ticker := time.NewTicker(reclaimInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				c.reclaimPending(ctx, sem, &wg)
			}
		}
	}()

	for {
		msgs, err := c.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    consumerGroup,
			Consumer: c.consumerID,
			Streams:  []string{StreamAuctionClosed, ">"},
			Count:    int64(c.numWorkers),
			Block:    2 * time.Second,
		}).Result()
		if err != nil {
			if errors.Is(err, context.Canceled) {
				break
			}
			if errors.Is(err, redis.Nil) {
				// Block timeout — no new messages, loop again.
				continue
			}
			log.Printf("payment consumer: read error: %v", err)
			time.Sleep(time.Second)
			continue
		}
		for _, stream := range msgs {
			for _, msg := range stream.Messages {
				c.dispatch(ctx, msg, sem, &wg)
			}
		}
	}

	wg.Wait()
	log.Println("payment consumer: stopped")
}

// dispatch hands off a single stream message to a worker goroutine.
// Messages that fail processing are not ACKed and will be reclaimed after pendingTimeout.
func (c *Consumer) dispatch(ctx context.Context, msg redis.XMessage, sem chan struct{}, wg *sync.WaitGroup) {
	payload, ok := msg.Values["payload"].(string)
	if !ok {
		log.Printf("payment consumer: missing payload field in message %s, discarding", msg.ID)
		c.rdb.XAck(ctx, StreamAuctionClosed, consumerGroup, msg.ID)
		return
	}
	var event AuctionClosedEvent
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		log.Printf("payment consumer: unmarshal error in message %s: %v, discarding", msg.ID, err)
		c.rdb.XAck(ctx, StreamAuctionClosed, consumerGroup, msg.ID)
		return
	}

	// Acquire a worker slot. Respect context cancellation to avoid blocking shutdown.
	select {
	case sem <- struct{}{}:
	case <-ctx.Done():
		return
	}
	wg.Add(1)
	go func() {
		defer func() {
			<-sem
			wg.Done()
		}()
		if err := c.svc.InitiatePayment(ctx, event); err != nil {
			log.Printf("payment consumer: initiate payment error for auction %s: %v", event.AuctionID, err)
			// No ACK — message stays in pending and will be reclaimed after pendingTimeout.
			return
		}
		if err := c.rdb.XAck(ctx, StreamAuctionClosed, consumerGroup, msg.ID).Err(); err != nil {
			log.Printf("payment consumer: ack error for message %s: %v", msg.ID, err)
		}
	}()
}

// reclaimPending uses XAUTOCLAIM to take ownership of messages that have been
// pending for longer than pendingTimeout (typically from a crashed consumer).
func (c *Consumer) reclaimPending(ctx context.Context, sem chan struct{}, wg *sync.WaitGroup) {
	msgs, _, err := c.rdb.XAutoClaim(ctx, &redis.XAutoClaimArgs{
		Stream:   StreamAuctionClosed,
		Group:    consumerGroup,
		Consumer: c.consumerID,
		MinIdle:  pendingTimeout,
		Start:    "0-0",
		Count:    int64(c.numWorkers),
	}).Result()
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			log.Printf("payment consumer: autoclaim error: %v", err)
		}
		return
	}
	for _, msg := range msgs {
		log.Printf("payment consumer: reclaiming pending message %s", msg.ID)
		c.dispatch(ctx, msg, sem, wg)
	}
}

func isGroupExistsErr(err error) bool {
	return err != nil && err.Error() == "BUSYGROUP Consumer Group name already exists"
}
