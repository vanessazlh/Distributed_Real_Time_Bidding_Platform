package notification

import (
	"sync"
	"testing"
)

// mockClient implements Client for testing without real network connections.
type mockClient struct {
	mu       sync.Mutex
	messages [][]byte
	ctype    string
}

func newMockClient(ctype string) *mockClient {
	return &mockClient{ctype: ctype}
}

func (c *mockClient) Send(msg []byte) error {
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

// newTestHub builds a Hub without a Redis client (safe for unit tests that
// never call SubscribeRedis).
func newTestHub() *Hub {
	return &Hub{
		clients: make(map[string]map[Client]struct{}),
		latency: newLatencyTracker(),
	}
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

func TestNoPreviousBidderSkipsNotification(t *testing.T) {
	hub := newTestHub()
	client := newMockClient("websocket")
	hub.Register("auc-001", client)

	// previous_bidder is empty → first bid, nobody to notify.
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

	if msgs := client.received(); len(msgs) != 0 {
		t.Errorf("expected 0 messages on first bid, got %d", len(msgs))
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
