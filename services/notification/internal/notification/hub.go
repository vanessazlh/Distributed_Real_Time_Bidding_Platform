package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
	"rtb/shared/events"
)

// Client is implemented by both WebSocket and SSE clients.
type Client interface {
	Send(msg []byte) error
	ClientType() string
}

// BidPlacedEvent is the shared event type from the events package.
type BidPlacedEvent = events.BidPlacedEvent

// OutbidMessage is broadcast to all clients watching the affected auction.
type OutbidMessage struct {
	Type           string `json:"type"`
	AuctionID      string `json:"auction_id"`
	UserID         string `json:"user_id"`          // new highest bidder
	Amount         int64  `json:"amount"`            // cents
	PreviousBidder string `json:"previous_bidder"`   // the outbid user
	ItemTitle      string `json:"item_title"`
	Message        string `json:"message"`           // human-readable outbid notification
	BidAcceptedAt  string `json:"bid_accepted_at"`   // when Auction Service accepted the bid
	DeliveredAt    string `json:"delivered_at"`      // when notification was sent (for latency calc)
	Timestamp      string `json:"timestamp"`
}

// Metrics holds hub statistics in the format Person 4 requires for Experiment 3.
type Metrics struct {
	ActiveConnections   int64   `json:"active_connections"`
	TotalBroadcasts     int64   `json:"total_broadcasts"`
	AvgDeliveryLatency  float64 `json:"avg_delivery_latency_ms"`
	P99DeliveryLatency  float64 `json:"p99_delivery_latency_ms"`
}

// latencyTracker stores delivery latency samples and computes avg / p99.
// Capped at maxSamples to bound memory usage during long load test runs.
type latencyTracker struct {
	mu         sync.Mutex
	samples    []float64
	maxSamples int
}

func newLatencyTracker() *latencyTracker {
	return &latencyTracker{maxSamples: 10_000}
}

// record adds a latency sample in milliseconds.
func (t *latencyTracker) record(ms float64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.samples) >= t.maxSamples {
		// Drop the oldest sample (sliding window).
		t.samples = t.samples[1:]
	}
	t.samples = append(t.samples, ms)
}

// stats returns avg and p99 over all recorded samples.
// Returns (0, 0) if no samples have been recorded yet.
func (t *latencyTracker) stats() (avg, p99 float64) {
	t.mu.Lock()
	cp := make([]float64, len(t.samples))
	copy(cp, t.samples)
	t.mu.Unlock()

	if len(cp) == 0 {
		return 0, 0
	}
	var sum float64
	for _, v := range cp {
		sum += v
	}
	avg = sum / float64(len(cp))

	sort.Float64s(cp)
	idx := int(math.Ceil(float64(len(cp))*0.99)) - 1
	if idx < 0 {
		idx = 0
	}
	p99 = cp[idx]
	return avg, p99
}

// Hub manages the in-memory client registry and Redis subscription.
// It maintains the mapping auction_id → []Client and fans out
// bid_placed events from Redis to all connected watchers.
type Hub struct {
	mu             sync.RWMutex
	clients        map[string]map[Client]struct{} // auction_id → set of clients
	wsCount        atomic.Int64
	sseCount       atomic.Int64
	broadcastCount atomic.Int64
	latency        *latencyTracker
	rdb            *redis.Client
}

// NewHub creates a new Hub backed by the given Redis client.
func NewHub(rdb *redis.Client) *Hub {
	return &Hub{
		clients: make(map[string]map[Client]struct{}),
		latency: newLatencyTracker(),
		rdb:     rdb,
	}
}

// Register adds c to the subscriber list for auctionID.
func (h *Hub) Register(auctionID string, c Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.clients[auctionID] == nil {
		h.clients[auctionID] = make(map[Client]struct{})
	}
	h.clients[auctionID][c] = struct{}{}
	if c.ClientType() == "websocket" {
		h.wsCount.Add(1)
	} else {
		h.sseCount.Add(1)
	}
}

// Unregister removes c from the subscriber list for auctionID.
// If no clients remain for that auction the entry is deleted.
func (h *Hub) Unregister(auctionID string, c Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	clients, ok := h.clients[auctionID]
	if !ok {
		return
	}
	if _, exists := clients[c]; !exists {
		return
	}
	delete(clients, c)
	if c.ClientType() == "websocket" {
		h.wsCount.Add(-1)
	} else {
		h.sseCount.Add(-1)
	}
	if len(clients) == 0 {
		delete(h.clients, auctionID)
	}
}

// Broadcast sends msg to all clients currently watching auctionID.
// The client list is copied under a read lock so sends happen without holding the lock.
// bidAcceptedAt is the ISO timestamp from the original bid event used to record latency.
func (h *Hub) Broadcast(auctionID string, msg []byte, bidAcceptedAt string) {
	h.mu.RLock()
	targets := make([]Client, 0, len(h.clients[auctionID]))
	for c := range h.clients[auctionID] {
		targets = append(targets, c)
	}
	h.mu.RUnlock()

	for _, c := range targets {
		if err := c.Send(msg); err != nil {
			log.Printf("hub: send error to %s client (auction %s): %v", c.ClientType(), auctionID, err)
		}
	}
	h.broadcastCount.Add(1)

	// Record delivery latency: time from bid acceptance to notification send.
	if bidAcceptedAt != "" {
		if accepted, err := time.Parse(time.RFC3339Nano, bidAcceptedAt); err == nil {
			h.latency.record(float64(time.Since(accepted).Microseconds()) / 1000.0)
		}
	}
}

// GetMetrics returns hub statistics in the format required by Person 4 for Experiment 3.
func (h *Hub) GetMetrics() Metrics {
	avg, p99 := h.latency.stats()
	return Metrics{
		ActiveConnections:  h.wsCount.Load() + h.sseCount.Load(),
		TotalBroadcasts:    h.broadcastCount.Load(),
		AvgDeliveryLatency: math.Round(avg*10) / 10,
		P99DeliveryLatency: math.Round(p99*10) / 10,
	}
}

// SubscribeRedis blocks and listens to the "bid_placed" Redis channel.
// For each event it calls handleBidEvent; returns when ctx is cancelled.
func (h *Hub) SubscribeRedis(ctx context.Context) {
	sub := h.rdb.Subscribe(ctx, "bid_placed")
	defer sub.Close()

	log.Println("hub: subscribed to Redis channel 'bid_placed'")
	ch := sub.Channel()
	for {
		select {
		case <-ctx.Done():
			log.Println("hub: Redis subscriber shutting down")
			return
		case msg, ok := <-ch:
			if !ok {
				log.Println("hub: Redis subscription channel closed")
				return
			}
			h.handleBidEvent(msg.Payload)
		}
	}
}

// handleBidEvent parses a raw bid_placed payload and broadcasts an outbid
// notification to all watchers of the affected auction.
// If previous_bidder is empty this is the first bid — no notification is sent.
func (h *Hub) handleBidEvent(payload string) {
	var event BidPlacedEvent
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		log.Printf("hub: failed to parse bid_placed event: %v", err)
		return
	}

	// First bid on this auction — nobody has been outbid yet.
	if event.PreviousBidder == "" {
		return
	}

	outbid := OutbidMessage{
		Type:           "bid_placed",
		AuctionID:      event.AuctionID,
		UserID:         event.UserID,
		Amount:         event.Amount,
		PreviousBidder: event.PreviousBidder,
		ItemTitle:      event.ItemTitle,
		Message:        fmt.Sprintf("You've been outbid on %s! Current: $%.2f", event.ItemTitle, float64(event.Amount)/100),
		BidAcceptedAt:  event.BidAcceptedAt,
		DeliveredAt:    time.Now().UTC().Format(time.RFC3339Nano),
		Timestamp:      event.Timestamp,
	}

	data, err := json.Marshal(outbid)
	if err != nil {
		log.Printf("hub: failed to marshal outbid message: %v", err)
		return
	}

	h.Broadcast(event.AuctionID, data, event.BidAcceptedAt)
	log.Printf("hub: broadcast to auction %s — new bid $%.2f, outbid user %s",
		event.AuctionID, float64(event.Amount)/100, event.PreviousBidder)
}
