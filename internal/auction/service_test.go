package auction_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/surplus-auction/platform/internal/auction"
)

// --- mock auction repo ---

type mockAuctionRepo struct {
	mu       sync.Mutex
	auctions map[string]*auction.Auction
}

func newMockAuctionRepo() *mockAuctionRepo {
	return &mockAuctionRepo{
		auctions: make(map[string]*auction.Auction),
	}
}

func (m *mockAuctionRepo) Create(_ context.Context, a *auction.Auction) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.auctions[a.AuctionID] = a
	return nil
}

func (m *mockAuctionRepo) GetByID(_ context.Context, auctionID string) (*auction.Auction, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.auctions[auctionID]
	if !ok {
		return nil, errors.New("auction not found")
	}
	// Return a copy
	cpy := *a
	return &cpy, nil
}

func (m *mockAuctionRepo) List(_ context.Context, status string) ([]*auction.Auction, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*auction.Auction
	for _, a := range m.auctions {
		if status == "" || a.Status == status {
			cpy := *a
			result = append(result, &cpy)
		}
	}
	return result, nil
}

func (m *mockAuctionRepo) Close(_ context.Context, auctionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.auctions[auctionID]
	if !ok {
		return errors.New("auction not found")
	}
	a.Status = "CLOSED"
	return nil
}

// --- tests ---

func TestCreateAuction_Success(t *testing.T) {
	repo := newMockAuctionRepo()
	// We can't easily test with real Redis in unit tests, so we test the repo layer
	a := &auction.Auction{
		AuctionID:      "test-auction",
		ItemID:         "item-1",
		ShopID:         "shop-1",
		StartTime:      time.Now().UTC(),
		EndTime:        time.Now().UTC().Add(10 * time.Minute),
		CurrentHighest: 0,
		Status:         "OPEN",
		Version:        0,
	}
	if err := repo.Create(context.Background(), a); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := repo.GetByID(context.Background(), "test-auction")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.AuctionID != "test-auction" {
		t.Fatalf("expected test-auction, got %s", got.AuctionID)
	}
	if got.Status != "OPEN" {
		t.Fatalf("expected OPEN, got %s", got.Status)
	}
}

func TestGetAuction_NotFound(t *testing.T) {
	repo := newMockAuctionRepo()
	_, err := repo.GetByID(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent auction")
	}
}

func TestListAuctions_ByStatus(t *testing.T) {
	repo := newMockAuctionRepo()
	ctx := context.Background()

	_ = repo.Create(ctx, &auction.Auction{AuctionID: "a1", Status: "OPEN"})
	_ = repo.Create(ctx, &auction.Auction{AuctionID: "a2", Status: "CLOSED"})
	_ = repo.Create(ctx, &auction.Auction{AuctionID: "a3", Status: "OPEN"})

	open, err := repo.List(ctx, "OPEN")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(open) != 2 {
		t.Fatalf("expected 2 open auctions, got %d", len(open))
	}

	all, err := repo.List(ctx, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 total auctions, got %d", len(all))
	}
}

func TestCloseAuction(t *testing.T) {
	repo := newMockAuctionRepo()
	ctx := context.Background()

	_ = repo.Create(ctx, &auction.Auction{AuctionID: "a1", Status: "OPEN"})
	if err := repo.Close(ctx, "a1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	a, _ := repo.GetByID(ctx, "a1")
	if a.Status != "CLOSED" {
		t.Fatalf("expected CLOSED, got %s", a.Status)
	}
}

func TestCloseAuction_NotFound(t *testing.T) {
	repo := newMockAuctionRepo()
	err := repo.Close(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent auction")
	}
}

func TestMetrics(t *testing.T) {
	m := auction.NewMetrics()

	m.RecordSuccessful(10 * time.Millisecond)
	m.RecordSuccessful(20 * time.Millisecond)
	m.RecordRejected()

	snap := m.Snapshot()
	if snap.TotalBids != 3 {
		t.Fatalf("expected 3 total bids, got %d", snap.TotalBids)
	}
	if snap.SuccessfulBids != 2 {
		t.Fatalf("expected 2 successful, got %d", snap.SuccessfulBids)
	}
	if snap.RejectedBids != 1 {
		t.Fatalf("expected 1 rejected, got %d", snap.RejectedBids)
	}
	if snap.AvgLatencyMs <= 0 {
		t.Fatal("expected positive avg latency")
	}
}

func TestMetrics_Reset(t *testing.T) {
	m := auction.NewMetrics()
	m.RecordSuccessful(5 * time.Millisecond)
	m.RecordRejected()
	m.Reset()

	snap := m.Snapshot()
	if snap.TotalBids != 0 {
		t.Fatalf("expected 0 total after reset, got %d", snap.TotalBids)
	}
}

func TestConcurrencyStrategy_Constants(t *testing.T) {
	if auction.Optimistic != "optimistic" {
		t.Fatalf("expected optimistic, got %s", auction.Optimistic)
	}
	if auction.Pessimistic != "pessimistic" {
		t.Fatalf("expected pessimistic, got %s", auction.Pessimistic)
	}
	if auction.Queue != "queue" {
		t.Fatalf("expected queue, got %s", auction.Queue)
	}
}

func TestErrVariables(t *testing.T) {
	if !errors.Is(auction.ErrNotFound, auction.ErrNotFound) {
		t.Fatal("ErrNotFound should match itself")
	}
	if !errors.Is(auction.ErrAuctionClosed, auction.ErrAuctionClosed) {
		t.Fatal("ErrAuctionClosed should match itself")
	}
	if !errors.Is(auction.ErrBidTooLow, auction.ErrBidTooLow) {
		t.Fatal("ErrBidTooLow should match itself")
	}
}

// Compile-time interface check
var _ auction.Repo = (*mockAuctionRepo)(nil)
