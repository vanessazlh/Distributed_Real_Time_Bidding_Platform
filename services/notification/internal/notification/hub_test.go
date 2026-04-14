package notification

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"
)

// mockClient implements Client for testing without real network connections.
type mockClient struct {
	mu       sync.Mutex
	messages [][]byte
	ctype    string
	sendErr  error
}

func newMockClient(ctype string) *mockClient {
	return &mockClient{ctype: ctype}
}

func (c *mockClient) Send(msg []byte) error {
	if c.sendErr != nil {
		return c.sendErr
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	cp := make([]byte, len(msg))
	copy(cp, msg)
	c.messages = append(c.messages, cp)
	return nil
}

func (c *mockClient) ClientType() string { return c.ctype }

func (c *mockClient) received() [][]byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([][]byte, len(c.messages))
	copy(out, c.messages)
	return out
}

type mockStore struct {
	mu          sync.Mutex
	added       map[string][]StoredNotification
	unread      map[string]int
	addErr      error
	unreadErr   error
}

func newMockStore() *mockStore {
	return &mockStore{
		added:  make(map[string][]StoredNotification),
		unread: make(map[string]int),
	}
}

func (s *mockStore) Add(_ context.Context, userID string, n StoredNotification) error {
	if s.addErr != nil {
		return s.addErr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.added[userID] = append(s.added[userID], n)
	s.unread[userID]++
	return nil
}

func (s *mockStore) UnreadCount(_ context.Context, userID string) (int, error) {
	if s.unreadErr != nil {
		return 0, s.unreadErr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.unread[userID], nil
}

// newTestHub builds a Hub without Redis stream consumption. This keeps tests
// focused on in-memory fanout and notification shaping.
func newTestHub() *Hub {
	return &Hub{
		clients:     make(map[string]map[Client]struct{}),
		userClients: make(map[string]map[Client]struct{}),
		latency:     newLatencyTracker(),
	}
}

func newTestHubWithStore(store notificationStore) *Hub {
	return &Hub{
		clients:     make(map[string]map[Client]struct{}),
		userClients: make(map[string]map[Client]struct{}),
		latency:     newLatencyTracker(),
		store:       store,
	}
}

func decodeUserMessage(t *testing.T, msg []byte) UserNotificationMessage {
	t.Helper()
	var got UserNotificationMessage
	if err := json.Unmarshal(msg, &got); err != nil {
		t.Fatalf("unmarshal user notification: %v", err)
	}
	return got
}

func TestSingleConnectionReceivesBroadcast(t *testing.T) {
	hub := newTestHub()
	client := newMockClient("websocket")
	hub.Register("auc-001", client)

	hub.Broadcast("auc-001", []byte(`{"type":"bid_placed"}`), "")

	msgs := client.received()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if string(msgs[0]) != `{"type":"bid_placed"}` {
		t.Errorf("unexpected message content: %s", msgs[0])
	}
}

func TestMultipleConnectionsAllReceiveBroadcast(t *testing.T) {
	hub := newTestHub()
	clients := []*mockClient{
		newMockClient("websocket"),
		newMockClient("websocket"),
		newMockClient("sse"),
	}
	for _, c := range clients {
		hub.Register("auc-001", c)
	}

	hub.Broadcast("auc-001", []byte(`{"type":"bid_placed"}`), "")

	for i, c := range clients {
		if msgs := c.received(); len(msgs) != 1 {
			t.Errorf("client %d (%s): expected 1 message, got %d", i, c.ctype, len(msgs))
		}
	}
}

func TestFirstBidBroadcastsToWatchers(t *testing.T) {
	hub := newTestHub()
	client := newMockClient("websocket")
	hub.Register("auc-001", client)

	// previous_bidder is empty → first bid, still broadcast to watchers.
	payload := `{
		"auction_id": "auc-001",
		"user_id":    "usr-001",
		"amount":     1000,
		"previous_bidder": "",
		"item_title": "Pastry Box",
		"timestamp":  "2026-03-28T10:00:00Z",
		"bid_accepted_at": "2026-03-28T10:00:00Z"
	}`
	hub.handleBidEvent(payload)

	if msgs := client.received(); len(msgs) != 1 {
		t.Errorf("expected 1 message on first bid, got %d", len(msgs))
	}
}

func TestAuctionClosedBroadcast(t *testing.T) {
	hub := newTestHub()
	client := newMockClient("websocket")
	hub.Register("auc-001", client)

	payload := `{
		"auction_id": "auc-001",
		"winner_id":  "usr-001",
		"winning_bid": 5000,
		"item_id":    "item-001",
		"shop_id":    "shop-001",
		"closed_at":  "2026-03-28T10:05:00Z"
	}`
	hub.handleAuctionClosedEvent(payload)

	msgs := client.received()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message on auction close, got %d", len(msgs))
	}
}

func TestBroadcastOnSubsequentBid(t *testing.T) {
	hub := newTestHub()
	client := newMockClient("websocket")
	hub.Register("auc-001", client)

	payload := `{
		"auction_id": "auc-001",
		"user_id":    "usr-002",
		"amount":     2000,
		"previous_bidder": "usr-001",
		"item_title": "Pastry Box",
		"timestamp":  "2026-03-28T10:00:01Z",
		"bid_accepted_at": "2026-03-28T10:00:01Z"
	}`
	hub.handleBidEvent(payload)

	if msgs := client.received(); len(msgs) != 1 {
		t.Errorf("expected 1 message on outbid event, got %d", len(msgs))
	}
}

func TestDisconnectedClientCleanedUp(t *testing.T) {
	hub := newTestHub()
	client := newMockClient("websocket")
	hub.Register("auc-001", client)
	hub.Unregister("auc-001", client)

	hub.mu.RLock()
	_, exists := hub.clients["auc-001"]
	hub.mu.RUnlock()

	if exists {
		t.Error("expected auction entry removed after last client unregisters")
	}
	if n := hub.wsCount.Load(); n != 0 {
		t.Errorf("expected wsCount=0 after unregister, got %d", n)
	}
}

func TestUnregisterNonExistentClientIsNoop(t *testing.T) {
	hub := newTestHub()
	client := newMockClient("websocket")
	// Should not panic.
	hub.Unregister("auc-999", client)
}

func TestMetricsCountConnections(t *testing.T) {
	hub := newTestHub()
	ws := newMockClient("websocket")
	sse := newMockClient("sse")

	hub.Register("auc-001", ws)
	hub.Register("auc-001", sse)

	m := hub.GetMetrics()
	if m.ActiveConnections != 2 {
		t.Errorf("expected ActiveConnections=2, got %d", m.ActiveConnections)
	}

	hub.Broadcast("auc-001", []byte("ping"), "")
	if m2 := hub.GetMetrics(); m2.TotalBroadcasts != 1 {
		t.Errorf("expected TotalBroadcasts=1, got %d", m2.TotalBroadcasts)
	}
}

func TestSendToUser_AllConnectionsReceiveMessage(t *testing.T) {
	hub := newTestHub()
	a := newMockClient("websocket")
	b := newMockClient("websocket")
	hub.RegisterUser("user-1", a)
	hub.RegisterUser("user-1", b)

	hub.SendToUser("user-1", []byte(`{"type":"notification"}`))

	if len(a.received()) != 1 || len(b.received()) != 1 {
		t.Fatal("expected both user-level clients to receive notification")
	}
}

func TestHandleBidEvent_PushesOutbidNotificationToPreviousBidder(t *testing.T) {
	store := newMockStore()
	hub := newTestHubWithStore(store)
	userClient := newMockClient("websocket")
	hub.RegisterUser("usr-001", userClient)

	payload := `{
		"auction_id": "auc-001",
		"user_id":    "usr-002",
		"amount":     2000,
		"previous_bidder": "usr-001",
		"item_title": "Pastry Box",
		"timestamp":  "2026-03-28T10:00:01Z",
		"bid_accepted_at": "2026-03-28T10:00:01Z"
	}`
	hub.handleBidEvent(payload)

	if len(store.added["usr-001"]) != 1 {
		t.Fatalf("expected one stored notification, got %d", len(store.added["usr-001"]))
	}
	n := store.added["usr-001"][0]
	if n.Type != "outbid" || n.AuctionID != "auc-001" {
		t.Fatalf("unexpected stored notification: %+v", n)
	}
	msgs := userClient.received()
	if len(msgs) != 1 {
		t.Fatalf("expected one pushed user notification, got %d", len(msgs))
	}
	got := decodeUserMessage(t, msgs[0])
	if got.Notification.Type != "outbid" || got.UnreadCount != 1 {
		t.Fatalf("unexpected pushed notification: %+v", got)
	}
}

func TestHandleBidEvent_DoesNotPushWhenPreviousBidderIsCurrentBidder(t *testing.T) {
	store := newMockStore()
	hub := newTestHubWithStore(store)

	payload := `{
		"auction_id": "auc-001",
		"user_id":    "usr-001",
		"amount":     2000,
		"previous_bidder": "usr-001",
		"item_title": "Pastry Box",
		"timestamp":  "2026-03-28T10:00:01Z",
		"bid_accepted_at": "2026-03-28T10:00:01Z"
	}`
	hub.handleBidEvent(payload)

	if len(store.added["usr-001"]) != 0 {
		t.Fatalf("expected no stored notification, got %d", len(store.added["usr-001"]))
	}
}

func TestHandleAuctionClosedEvent_PushesToAllWinners(t *testing.T) {
	store := newMockStore()
	hub := newTestHubWithStore(store)
	user1 := newMockClient("websocket")
	user2 := newMockClient("websocket")
	hub.RegisterUser("usr-001", user1)
	hub.RegisterUser("usr-002", user2)

	payload := `{
		"auction_id": "auc-001",
		"item_title": "Pastry Box",
		"shop_id":    "shop-001",
		"winners": {
			"usr-001": 5000,
			"usr-002": 4500
		},
		"closed_at":  "2026-03-28T10:05:00Z"
	}`
	hub.handleAuctionClosedEvent(payload)

	if len(store.added["usr-001"]) != 1 || len(store.added["usr-002"]) != 1 {
		t.Fatalf("expected winner notifications for both users, got %+v", store.added)
	}
	if got := decodeUserMessage(t, user1.received()[0]); got.Notification.Type != "won" {
		t.Fatalf("expected won notification for user1, got %+v", got)
	}
	if got := decodeUserMessage(t, user2.received()[0]); got.Notification.Type != "won" {
		t.Fatalf("expected won notification for user2, got %+v", got)
	}
}

func TestStoreAndPushNotification_StoreFailurePreventsPush(t *testing.T) {
	store := newMockStore()
	store.addErr = errors.New("boom")
	hub := newTestHubWithStore(store)
	userClient := newMockClient("websocket")
	hub.RegisterUser("usr-001", userClient)

	hub.storeAndPushNotification("usr-001", StoredNotification{ID: "n1", Type: "outbid"})

	if len(userClient.received()) != 0 {
		t.Fatal("expected no pushed notification when store add fails")
	}
}

func TestStoreAndPushNotification_UnreadCountFailureFallsBackToZero(t *testing.T) {
	store := newMockStore()
	store.unreadErr = errors.New("boom")
	hub := newTestHubWithStore(store)
	userClient := newMockClient("websocket")
	hub.RegisterUser("usr-001", userClient)

	hub.storeAndPushNotification("usr-001", StoredNotification{ID: "n1", Type: "outbid"})

	msgs := userClient.received()
	if len(msgs) != 1 {
		t.Fatalf("expected pushed notification, got %d", len(msgs))
	}
	got := decodeUserMessage(t, msgs[0])
	if got.UnreadCount != 0 {
		t.Fatalf("expected unread fallback 0, got %d", got.UnreadCount)
	}
}

func TestBroadcast_RecordsLatency(t *testing.T) {
	hub := newTestHub()
	client := newMockClient("websocket")
	hub.Register("auc-001", client)
	acceptedAt := time.Now().UTC().Add(-10 * time.Millisecond).Format(time.RFC3339Nano)

	hub.Broadcast("auc-001", []byte("ping"), acceptedAt)

	m := hub.GetMetrics()
	if m.TotalBroadcasts != 1 {
		t.Fatalf("expected one broadcast, got %d", m.TotalBroadcasts)
	}
	if m.AvgDeliveryLatency <= 0 {
		t.Fatalf("expected positive avg latency, got %f", m.AvgDeliveryLatency)
	}
}
