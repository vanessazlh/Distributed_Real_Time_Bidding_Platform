package notification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"rtb/shared/events"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	streamBidPlaced     = "bid:placed"
	streamAuctionClosed = "auction:closed"
	notifConsumerGroup  = "notification-service"
	notifPendingTimeout = 60 * time.Second
	notifReclaimInterval = 30 * time.Second
)

// Client is implemented by WebSocket clients.
type Client interface {
	Send(msg []byte) error
}

// BidPlacedEvent is the shared event type from the events package.
type BidPlacedEvent = events.BidPlacedEvent

// OutbidMessage is broadcast to all clients watching the affected auction.
type OutbidMessage struct {
	Type           string `json:"type"`
	AuctionID      string `json:"auction_id"`
	UserID         string `json:"user_id"`         // new highest bidder
	Amount         int64  `json:"amount"`          // cents
	PreviousBidder string `json:"previous_bidder"` // the outbid user
	ItemTitle      string `json:"item_title"`
	Message        string `json:"message"`         // human-readable outbid notification
	BidAcceptedAt  string `json:"bid_accepted_at"` // when Auction Service accepted the bid
	DeliveredAt    string `json:"delivered_at"`    // when notification was sent (for latency calc)
	Timestamp      string `json:"timestamp"`
}

// AuctionClosedMessage is broadcast when an auction ends.
type AuctionClosedMessage struct {
	Type       string `json:"type"`
	AuctionID  string `json:"auction_id"`
	WinnerID   string `json:"winner_id"`
	WinningBid int64  `json:"winning_bid"`
	Message    string `json:"message"`
	ClosedAt   string `json:"closed_at"`
}

// AuctionClosedEvent is the shared event type from the events package.
type AuctionClosedEvent = events.AuctionClosedEvent

// Metrics holds hub statistics
type Metrics struct {
	ActiveConnections  int64   `json:"active_connections"`
	TotalBroadcasts    int64   `json:"total_broadcasts"`
	AvgDeliveryLatency float64 `json:"avg_delivery_latency_ms"`
	P99DeliveryLatency float64 `json:"p99_delivery_latency_ms"`
}

type notificationStore interface {
	Add(ctx context.Context, userID string, n StoredNotification) error
	UnreadCount(ctx context.Context, userID string) (int, error)
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
// It also tracks user-level WebSocket clients for global notifications.
type Hub struct {
	mu             sync.RWMutex
	clients        map[string]map[Client]struct{} // auction_id → set of clients
	userMu         sync.RWMutex
	userClients    map[string]map[Client]struct{} // user_id → set of clients
	wsCount        atomic.Int64
	broadcastCount atomic.Int64
	latency        *latencyTracker
	rdb            *redis.Client
	store          notificationStore
}

// NewHub creates a new Hub backed by the given Redis client.
func NewHub(rdb *redis.Client, store notificationStore) *Hub {
	return &Hub{
		clients:     make(map[string]map[Client]struct{}),
		userClients: make(map[string]map[Client]struct{}),
		latency:     newLatencyTracker(),
		rdb:         rdb,
		store:       store,
	}
}

// Register adds c to the subscriber list for auctionID (per-auction WS).
func (h *Hub) Register(auctionID string, c Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.clients[auctionID] == nil {
		h.clients[auctionID] = make(map[Client]struct{})
	}
	h.clients[auctionID][c] = struct{}{}
	h.wsCount.Add(1)
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
	h.wsCount.Add(-1)
	if len(clients) == 0 {
		delete(h.clients, auctionID)
	}
}

// RegisterUser adds c to the global user-level client list.
func (h *Hub) RegisterUser(userID string, c Client) {
	h.userMu.Lock()
	defer h.userMu.Unlock()
	if h.userClients[userID] == nil {
		h.userClients[userID] = make(map[Client]struct{})
	}
	h.userClients[userID][c] = struct{}{}
	h.wsCount.Add(1)
}

// UnregisterUser removes c from the user-level client list.
func (h *Hub) UnregisterUser(userID string, c Client) {
	h.userMu.Lock()
	defer h.userMu.Unlock()
	clients, ok := h.userClients[userID]
	if !ok {
		return
	}
	if _, exists := clients[c]; !exists {
		return
	}
	delete(clients, c)
	h.wsCount.Add(-1)
	if len(clients) == 0 {
		delete(h.userClients, userID)
	}
}

// SendToUser sends a message to all WebSocket connections for a given user.
func (h *Hub) SendToUser(userID string, msg []byte) {
	h.userMu.RLock()
	targets := make([]Client, 0, len(h.userClients[userID]))
	for c := range h.userClients[userID] {
		targets = append(targets, c)
	}
	h.userMu.RUnlock()

	for _, c := range targets {
		if err := c.Send(msg); err != nil {
			log.Printf("hub: user send error (user %s): %v", userID, err)
		}
	}
}

// storeAndPushNotification persists a notification and pushes it via user WS.
func (h *Hub) storeAndPushNotification(userID string, n StoredNotification) {
	if h.store == nil {
		return
	}
	ctx := context.Background()
	if err := h.store.Add(ctx, userID, n); err != nil {
		log.Printf("hub: failed to store notification for user %s: %v", userID, err)
		return
	}
	unread, err := h.store.UnreadCount(ctx, userID)
	if err != nil {
		log.Printf("hub: failed to get unread count for user %s: %v", userID, err)
		unread = 0
	}
	wrapper := UserNotificationMessage{
		Type:         "notification",
		Notification: n,
		UnreadCount:  unread,
	}
	data, err := json.Marshal(wrapper)
	if err != nil {
		log.Printf("hub: failed to marshal user notification: %v", err)
		return
	}
	h.SendToUser(userID, data)
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
			log.Printf("hub: send error (auction %s): %v", auctionID, err)
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
		ActiveConnections:  h.wsCount.Load(),
		TotalBroadcasts:    h.broadcastCount.Load(),
		AvgDeliveryLatency: math.Round(avg*10) / 10,
		P99DeliveryLatency: math.Round(p99*10) / 10,
	}
}

// ConsumeStreams reads from the bid:placed and auction:closed Redis Streams
// using a consumer group. It blocks until ctx is cancelled. numWorkers controls
// the concurrency of message processing; pass 0 for a default of 10.
func (h *Hub) ConsumeStreams(ctx context.Context, numWorkers int) {
	if numWorkers <= 0 {
		numWorkers = 10
	}
	consumerID := uuid.New().String()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		h.consumeStream(ctx, streamBidPlaced, consumerID, numWorkers, h.handleBidEvent)
	}()
	go func() {
		defer wg.Done()
		h.consumeStream(ctx, streamAuctionClosed, consumerID, numWorkers, h.handleAuctionClosedEvent)
	}()
	wg.Wait()
}

// consumeStream runs the read loop for a single stream.
// Notification delivery is best-effort: messages are always ACKed after the
// handler runs, because retrying a failed WebSocket push is unlikely to help.
func (h *Hub) consumeStream(ctx context.Context, stream, consumerID string, numWorkers int, handler func(string)) {
	if err := h.rdb.XGroupCreateMkStream(ctx, stream, notifConsumerGroup, "$").Err(); err != nil {
		if !isGroupExistsErr(err) {
			log.Fatalf("hub: create consumer group for %s: %v", stream, err)
		}
	}

	log.Printf("hub: consuming stream=%q group=%q consumer=%q workers=%d",
		stream, notifConsumerGroup, consumerID, numWorkers)

	sem := make(chan struct{}, numWorkers)
	var wg sync.WaitGroup

	// Periodically reclaim messages stuck in pending (dead consumer instances).
	go func() {
		ticker := time.NewTicker(notifReclaimInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				h.reclaimPending(ctx, stream, consumerID, numWorkers, handler, sem, &wg)
			}
		}
	}()

	for {
		msgs, err := h.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    notifConsumerGroup,
			Consumer: consumerID,
			Streams:  []string{stream, ">"},
			Count:    int64(numWorkers),
			Block:    2 * time.Second,
		}).Result()
		if err != nil {
			if errors.Is(err, context.Canceled) {
				break
			}
			if errors.Is(err, redis.Nil) {
				continue
			}
			log.Printf("hub: read error on %s: %v", stream, err)
			time.Sleep(time.Second)
			continue
		}
		for _, s := range msgs {
			for _, msg := range s.Messages {
				h.dispatchNotif(ctx, stream, msg, consumerID, handler, sem, &wg)
			}
		}
	}

	wg.Wait()
	log.Printf("hub: stream %s consumer stopped", stream)
}

func (h *Hub) dispatchNotif(ctx context.Context, stream string, msg redis.XMessage, consumerID string, handler func(string), sem chan struct{}, wg *sync.WaitGroup) {
	payload, ok := msg.Values["payload"].(string)
	if !ok {
		log.Printf("hub: missing payload in message %s on %s, discarding", msg.ID, stream)
		h.rdb.XAck(ctx, stream, notifConsumerGroup, msg.ID)
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
		handler(payload)
		// Always ACK — notification delivery is best-effort.
		if err := h.rdb.XAck(ctx, stream, notifConsumerGroup, msg.ID).Err(); err != nil {
			log.Printf("hub: ack error for message %s on %s: %v", msg.ID, stream, err)
		}
	}()
}

func (h *Hub) reclaimPending(ctx context.Context, stream, consumerID string, numWorkers int, handler func(string), sem chan struct{}, wg *sync.WaitGroup) {
	msgs, _, err := h.rdb.XAutoClaim(ctx, &redis.XAutoClaimArgs{
		Stream:   stream,
		Group:    notifConsumerGroup,
		Consumer: consumerID,
		MinIdle:  notifPendingTimeout,
		Start:    "0-0",
		Count:    int64(numWorkers),
	}).Result()
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			log.Printf("hub: autoclaim error on %s: %v", stream, err)
		}
		return
	}
	for _, msg := range msgs {
		log.Printf("hub: reclaiming pending message %s on %s", msg.ID, stream)
		h.dispatchNotif(ctx, stream, msg, consumerID, handler, sem, wg)
	}
}

func isGroupExistsErr(err error) bool {
	return err != nil && err.Error() == "BUSYGROUP Consumer Group name already exists"
}

// handleBidEvent parses a raw bid_placed payload and broadcasts a notification
// to all watchers of the affected auction.
func (h *Hub) handleBidEvent(payload string) {
	var event BidPlacedEvent
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		log.Printf("hub: failed to parse bid_placed event: %v", err)
		return
	}

	message := fmt.Sprintf("New bid on %s: $%.2f", event.ItemTitle, float64(event.Amount)/100)
	if event.PreviousBidder != "" {
		message = fmt.Sprintf("You've been outbid on %s! Current: $%.2f", event.ItemTitle, float64(event.Amount)/100)
	}

	outbid := OutbidMessage{
		Type:           "bid_placed",
		AuctionID:      event.AuctionID,
		UserID:         event.UserID,
		Amount:         event.Amount,
		PreviousBidder: event.PreviousBidder,
		ItemTitle:      event.ItemTitle,
		Message:        message,
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
	log.Printf("hub: broadcast to auction %s — new bid $%.2f by %s",
		event.AuctionID, float64(event.Amount)/100, event.UserID)

	// Store + push outbid notification to the previous bidder's global WS.
	if event.PreviousBidder != "" && event.PreviousBidder != event.UserID {
		h.storeAndPushNotification(event.PreviousBidder, StoredNotification{
			ID:        fmt.Sprintf("outbid:%s", event.AuctionID),
			Type:      "outbid",
			AuctionID: event.AuctionID,
			ItemTitle: event.ItemTitle,
			Message:   fmt.Sprintf("You've been outbid on %s! Current: $%.2f", event.ItemTitle, float64(event.Amount)/100),
			Link:      fmt.Sprintf("/auction/%s", event.AuctionID),
			Amount:    event.Amount,
			CreatedAt: time.Now().UnixMilli(),
			Read:      false,
		})
	}
}

// handleAuctionClosedEvent parses an auction_closed payload and broadcasts
// a close notification to all watchers of the auction.
func (h *Hub) handleAuctionClosedEvent(payload string) {
	var event AuctionClosedEvent
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		log.Printf("hub: failed to parse auction_closed event: %v", err)
		return
	}

	message := "Auction closed with no bids."
	if event.WinnerID != "" {
		message = fmt.Sprintf("Auction closed! Winning bid: $%.2f", float64(event.WinningBid)/100)
	}

	closed := AuctionClosedMessage{
		Type:       "auction_closed",
		AuctionID:  event.AuctionID,
		WinnerID:   event.WinnerID,
		WinningBid: event.WinningBid,
		Message:    message,
		ClosedAt:   event.ClosedAt,
	}

	data, err := json.Marshal(closed)
	if err != nil {
		log.Printf("hub: failed to marshal auction_closed message: %v", err)
		return
	}

	h.Broadcast(event.AuctionID, data, "")
	log.Printf("hub: broadcast auction_closed for %s, winner %s", event.AuctionID, event.WinnerID)

	// Store + push "won" notification to all winners.
	// For multi-winner auctions (quantity>1), iterate over the Winners map.
	// For single-winner, fall back to WinnerID/WinningBid (backwards compatible).
	winners := event.Winners
	if len(winners) == 0 && event.WinnerID != "" {
		winners = map[string]int64{event.WinnerID: event.WinningBid}
	}

	for winnerID, winningBid := range winners {
		wonMsg := fmt.Sprintf("You won the auction! Winning bid: $%.2f", float64(winningBid)/100)
		if event.ItemTitle != "" {
			wonMsg = fmt.Sprintf("You won %s! Winning bid: $%.2f", event.ItemTitle, float64(winningBid)/100)
		}
		h.storeAndPushNotification(winnerID, StoredNotification{
			ID:        fmt.Sprintf("won:%s:%s", event.AuctionID, winnerID),
			Type:      "won",
			AuctionID: event.AuctionID,
			ItemTitle: event.ItemTitle,
			Message:   wonMsg,
			Link:      fmt.Sprintf("/auction/%s", event.AuctionID),
			Amount:    winningBid,
			CreatedAt: time.Now().UnixMilli(),
			Read:      false,
		})
	}
}
