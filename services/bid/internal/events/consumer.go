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
	"rtb/services/bid/internal/bid"
	"rtb/shared/events"
)

const (
	StreamBidPlaced     = "bid:placed"
	StreamAuctionClosed = "auction:closed"
	consumerGroup       = "bid-service"
	pendingTimeout      = 60 * time.Second
	reclaimInterval     = 30 * time.Second
)

// Consumer reads from Redis Streams using consumer groups and records bids.
type Consumer struct {
	rdb        *redis.Client
	bidSvc     *bid.Service
	consumerID string
	stopCh     chan struct{}
	numWorkers int
}

// NewConsumer creates a new event consumer. numWorkers controls concurrency;
// pass 0 to use the default of 10.
func NewConsumer(rdb *redis.Client, bidSvc *bid.Service, numWorkers int) *Consumer {
	if numWorkers <= 0 {
		numWorkers = 10
	}
	return &Consumer{
		rdb:        rdb,
		bidSvc:     bidSvc,
		consumerID: uuid.New().String(),
		stopCh:     make(chan struct{}),
		numWorkers: numWorkers,
	}
}

// Start begins consuming from bid:placed and auction:closed streams.
func (c *Consumer) Start() {
	go c.runStream(StreamBidPlaced, c.handleBidPlaced)
	go c.runStream(StreamAuctionClosed, c.handleAuctionClosed)
	log.Println("bid event consumer started (Redis Streams)")
}

// Stop signals the consumer to shut down.
func (c *Consumer) Stop() {
	close(c.stopCh)
	log.Println("bid event consumer stopped")
}

// runStream consumes a single stream with a dedicated consumer group loop.
func (c *Consumer) runStream(stream string, handler func(payload string) error) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-c.stopCh
		cancel()
	}()

	// Create the consumer group if it doesn't exist.
	// "$" means only consume messages published after this point.
	if err := c.rdb.XGroupCreateMkStream(ctx, stream, consumerGroup, "$").Err(); err != nil {
		if !isGroupExistsErr(err) {
			log.Fatalf("bid consumer: create consumer group for %s: %v", stream, err)
		}
	}

	log.Printf("bid consumer: stream=%q group=%q consumer=%q workers=%d",
		stream, consumerGroup, c.consumerID, c.numWorkers)

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
				c.reclaimPending(ctx, stream, handler, sem, &wg)
			}
		}
	}()

	for {
		msgs, err := c.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    consumerGroup,
			Consumer: c.consumerID,
			Streams:  []string{stream, ">"},
			Count:    int64(c.numWorkers),
			Block:    2 * time.Second,
		}).Result()
		if err != nil {
			if errors.Is(err, context.Canceled) {
				break
			}
			if errors.Is(err, redis.Nil) {
				continue
			}
			log.Printf("bid consumer: read error on %s: %v", stream, err)
			time.Sleep(time.Second)
			continue
		}
		for _, s := range msgs {
			for _, msg := range s.Messages {
				c.dispatch(ctx, stream, msg, handler, sem, &wg)
			}
		}
	}

	wg.Wait()
	log.Printf("bid consumer: stream %s stopped", stream)
}

// dispatch hands off a single stream message to a worker goroutine.
// Messages that fail processing are not ACKed and will be reclaimed after pendingTimeout.
func (c *Consumer) dispatch(ctx context.Context, stream string, msg redis.XMessage, handler func(string) error, sem chan struct{}, wg *sync.WaitGroup) {
	payload, ok := msg.Values["payload"].(string)
	if !ok {
		log.Printf("bid consumer: missing payload in message %s on %s, discarding", msg.ID, stream)
		c.rdb.XAck(ctx, stream, consumerGroup, msg.ID)
		return
	}

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
		if err := handler(payload); err != nil {
			log.Printf("bid consumer: handler error on %s message %s: %v", stream, msg.ID, err)
			// No ACK — message stays pending for retry via XAutoClaim.
			return
		}
		if err := c.rdb.XAck(ctx, stream, consumerGroup, msg.ID).Err(); err != nil {
			log.Printf("bid consumer: ack error for message %s: %v", msg.ID, err)
		}
	}()
}

// reclaimPending uses XAUTOCLAIM to take ownership of messages that have been
// pending for longer than pendingTimeout (typically from a crashed consumer).
func (c *Consumer) reclaimPending(ctx context.Context, stream string, handler func(string) error, sem chan struct{}, wg *sync.WaitGroup) {
	msgs, _, err := c.rdb.XAutoClaim(ctx, &redis.XAutoClaimArgs{
		Stream:   stream,
		Group:    consumerGroup,
		Consumer: c.consumerID,
		MinIdle:  pendingTimeout,
		Start:    "0-0",
		Count:    int64(c.numWorkers),
	}).Result()
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			log.Printf("bid consumer: autoclaim error on %s: %v", stream, err)
		}
		return
	}
	for _, msg := range msgs {
		log.Printf("bid consumer: reclaiming pending message %s on %s", msg.ID, stream)
		c.dispatch(ctx, stream, msg, handler, sem, wg)
	}
}

func (c *Consumer) handleBidPlaced(payload string) error {
	var event events.BidPlacedEvent
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		// Malformed message — log and return nil to ACK (no point retrying).
		log.Printf("bid consumer: unmarshal bid_placed error (discarding): %v", err)
		return nil
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
		ShopID:    event.ShopID,
		ShopName:  event.ShopName,
		Amount:    event.Amount,
		Timestamp: ts,
		Status:    "ACCEPTED",
	}

	ctx := context.Background()
	if err := c.bidSvc.RecordBid(ctx, b); err != nil {
		return err
	}
	return nil
}

func (c *Consumer) handleAuctionClosed(payload string) error {
	var event events.AuctionClosedEvent
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		log.Printf("bid consumer: unmarshal auction_closed error (discarding): %v", err)
		return nil
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
		return nil
	}

	ctx := context.Background()
	var lastErr error
	for winnerID := range winners {
		if err := c.bidSvc.MarkWinnerBid(ctx, event.AuctionID, winnerID); err != nil {
			log.Printf("failed to mark winner bid for auction %s, winner %s: %v", event.AuctionID, winnerID, err)
			lastErr = err
		} else {
			log.Printf("marked winning bid for auction %s, winner %s", event.AuctionID, winnerID)
		}
	}
	return lastErr
}

func isGroupExistsErr(err error) bool {
	return err != nil && err.Error() == "BUSYGROUP Consumer Group name already exists"
}
