package bid_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"rtb/services/bid/internal/bid"
)

// --- mock repo ---

type mockRepo struct {
	mu      sync.Mutex
	bids    map[string]*bid.Bid
	auction map[string][]string // auctionID -> bidIDs
	user    map[string][]string // userID -> bidIDs
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		bids:    make(map[string]*bid.Bid),
		auction: make(map[string][]string),
		user:    make(map[string][]string),
	}
}

func (m *mockRepo) Create(_ context.Context, b *bid.Bid) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.bids[b.BidID] = b
	m.auction[b.AuctionID] = append(m.auction[b.AuctionID], b.BidID)
	m.user[b.UserID] = append(m.user[b.UserID], b.BidID)
	return nil
}

func (m *mockRepo) GetByAuction(_ context.Context, auctionID string) ([]*bid.Bid, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ids := m.auction[auctionID]
	result := make([]*bid.Bid, 0, len(ids))
	for _, id := range ids {
		if b, ok := m.bids[id]; ok {
			result = append(result, b)
		}
	}
	return result, nil
}

func (m *mockRepo) GetByUser(_ context.Context, userID string) ([]*bid.Bid, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ids := m.user[userID]
	result := make([]*bid.Bid, 0, len(ids))
	for _, id := range ids {
		if b, ok := m.bids[id]; ok {
			result = append(result, b)
		}
	}
	return result, nil
}

func (m *mockRepo) MarkOutbid(_ context.Context, auctionID string, excludeBidID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	ids := m.auction[auctionID]
	for _, id := range ids {
		if id == excludeBidID {
			continue
		}
		if b, ok := m.bids[id]; ok && b.Status == "ACCEPTED" {
			b.Status = "OUTBID"
		}
	}
	return nil
}

// --- tests ---

func TestRecordBid_Success(t *testing.T) {
	svc := bid.NewService(newMockRepo())
	b := &bid.Bid{
		BidID:     "bid-1",
		AuctionID: "auction-1",
		UserID:    "user-1",
		Amount:    1000,
		Timestamp: time.Now().UTC(),
		Status:    "ACCEPTED",
	}
	if err := svc.RecordBid(context.Background(), b); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRecordBid_MarksOutbid(t *testing.T) {
	repo := newMockRepo()
	svc := bid.NewService(repo)
	ctx := context.Background()

	b1 := &bid.Bid{BidID: "bid-1", AuctionID: "a1", UserID: "u1", Amount: 100, Timestamp: time.Now().UTC(), Status: "ACCEPTED"}
	b2 := &bid.Bid{BidID: "bid-2", AuctionID: "a1", UserID: "u2", Amount: 200, Timestamp: time.Now().UTC(), Status: "ACCEPTED"}

	_ = svc.RecordBid(ctx, b1)
	_ = svc.RecordBid(ctx, b2)

	bids, _ := svc.GetAuctionBids(ctx, "a1")
	for _, b := range bids {
		if b.BidID == "bid-1" && b.Status != "OUTBID" {
			t.Fatalf("expected bid-1 to be OUTBID, got %s", b.Status)
		}
		if b.BidID == "bid-2" && b.Status != "ACCEPTED" {
			t.Fatalf("expected bid-2 to be ACCEPTED, got %s", b.Status)
		}
	}
}

func TestGetAuctionBids(t *testing.T) {
	repo := newMockRepo()
	svc := bid.NewService(repo)
	ctx := context.Background()

	_ = svc.RecordBid(ctx, &bid.Bid{BidID: "b1", AuctionID: "a1", UserID: "u1", Amount: 100, Timestamp: time.Now().UTC(), Status: "ACCEPTED"})
	_ = svc.RecordBid(ctx, &bid.Bid{BidID: "b2", AuctionID: "a1", UserID: "u2", Amount: 200, Timestamp: time.Now().UTC(), Status: "ACCEPTED"})
	_ = svc.RecordBid(ctx, &bid.Bid{BidID: "b3", AuctionID: "a2", UserID: "u1", Amount: 50, Timestamp: time.Now().UTC(), Status: "ACCEPTED"})

	bids, err := svc.GetAuctionBids(ctx, "a1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(bids) != 2 {
		t.Fatalf("expected 2 bids for auction a1, got %d", len(bids))
	}
}

func TestGetUserBids(t *testing.T) {
	repo := newMockRepo()
	svc := bid.NewService(repo)
	ctx := context.Background()

	_ = svc.RecordBid(ctx, &bid.Bid{BidID: "b1", AuctionID: "a1", UserID: "u1", Amount: 100, Timestamp: time.Now().UTC(), Status: "ACCEPTED"})
	_ = svc.RecordBid(ctx, &bid.Bid{BidID: "b2", AuctionID: "a2", UserID: "u1", Amount: 200, Timestamp: time.Now().UTC(), Status: "ACCEPTED"})

	bids, err := svc.GetUserBids(ctx, "u1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(bids) != 2 {
		t.Fatalf("expected 2 bids for user u1, got %d", len(bids))
	}
}

func TestGetUserBids_Empty(t *testing.T) {
	svc := bid.NewService(newMockRepo())
	bids, err := svc.GetUserBids(context.Background(), "nobody")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bids == nil {
		t.Fatal("expected empty slice, got nil")
	}
	if len(bids) != 0 {
		t.Fatalf("expected 0 bids, got %d", len(bids))
	}
}

// Verify that the Repo interface is actually used (compile-time check)
var _ bid.Repo = (*mockRepo)(nil)

// Verify error variable exists
func TestErrNotFound(t *testing.T) {
	if !errors.Is(bid.ErrNotFound, bid.ErrNotFound) {
		t.Fatal("ErrNotFound should match itself")
	}
}
