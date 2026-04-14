// Package auction_test exercises exported auction service behavior with
// lightweight in-memory test doubles instead of real Redis or DynamoDB.
package auction_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"rtb/services/auction/internal/auction"
	localEvents "rtb/services/auction/internal/events"
)

// mockAuctionRepo is an in-memory implementation of auction.Repo used by
// service-level unit tests.
type mockAuctionRepo struct {
	mu       sync.Mutex
	auctions map[string]*auction.Auction
}

func newMockAuctionRepo() *mockAuctionRepo {
	return &mockAuctionRepo{auctions: make(map[string]*auction.Auction)}
}

func (m *mockAuctionRepo) Create(_ context.Context, a *auction.Auction) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *a
	m.auctions[a.AuctionID] = &cp
	return nil
}

func (m *mockAuctionRepo) GetByID(_ context.Context, auctionID string) (*auction.Auction, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.auctions[auctionID]
	if !ok {
		return nil, errors.New("auction not found")
	}
	cp := *a
	return &cp, nil
}

func (m *mockAuctionRepo) List(_ context.Context, status string) ([]*auction.Auction, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*auction.Auction
	for _, a := range m.auctions {
		if status == "" || a.Status == status {
			cp := *a
			result = append(result, &cp)
		}
	}
	return result, nil
}

func (m *mockAuctionRepo) ListByShop(_ context.Context, shopID string) ([]*auction.Auction, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*auction.Auction
	for _, a := range m.auctions {
		if a.ShopID == shopID {
			cp := *a
			result = append(result, &cp)
		}
	}
	return result, nil
}

func (m *mockAuctionRepo) Open(_ context.Context, auctionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.auctions[auctionID]
	if !ok {
		return errors.New("auction not found")
	}
	a.Status = "OPEN"
	return nil
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

func (m *mockAuctionRepo) AtomicCloseAndReadWinner(_ context.Context, auctionID string) (string, int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.auctions[auctionID]
	if !ok {
		return "", 0, errors.New("auction not found")
	}
	if a.Status != "OPEN" {
		return "", 0, errors.New("auction is not open")
	}
	a.Status = "CLOSED"
	return a.HighestBidder, a.CurrentHighest, nil
}

func (m *mockAuctionRepo) AtomicCloseAndReadWinners(_ context.Context, auctionID string) (map[string]int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.auctions[auctionID]
	if !ok {
		return nil, errors.New("auction not found")
	}
	if a.Status != "OPEN" {
		return nil, errors.New("auction is not open")
	}
	a.Status = "CLOSED"
	if len(a.Winners) > 0 {
		cp := make(map[string]int64, len(a.Winners))
		for k, v := range a.Winners {
			cp[k] = v
		}
		return cp, nil
	}
	if a.HighestBidder != "" {
		return map[string]int64{a.HighestBidder: a.CurrentHighest}, nil
	}
	return map[string]int64{}, nil
}

func (m *mockAuctionRepo) RollbackClose(_ context.Context, auctionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if a, ok := m.auctions[auctionID]; ok && a.Status == "CLOSED" {
		a.Status = "OPEN"
	}
	return nil
}

func (m *mockAuctionRepo) PersistClosedState(_ context.Context, _ string) error { return nil }
func (m *mockAuctionRepo) CleanupRedis(_ context.Context, _ string) error       { return nil }
func (m *mockAuctionRepo) GetDynamoWinners(_ context.Context, _ string) (string, int64, error) {
	return "", 0, errors.New("not configured")
}
func (m *mockAuctionRepo) GetDynamoAllWinners(_ context.Context, _ string) (map[string]int64, error) {
	return nil, errors.New("not configured")
}

func newTestService(repo auction.Repo) *auction.Service {
	return auction.NewService(repo, nil, nil, auction.Pessimistic)
}

func newClosingTestService(repo auction.Repo) *auction.Service {
	// Use an invalid local Redis address so publisher calls fail immediately.
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"})
	return auction.NewService(repo, localEvents.NewPublisher(rdb), rdb, auction.Pessimistic)
}

func validCreateRequest() auction.CreateAuctionRequest {
	return auction.CreateAuctionRequest{
		ItemID:      "item-1",
		ItemTitle:   "Test Item",
		ShopID:      "shop-1",
		ShopName:    "Test Shop",
		RetailPrice: 1000,
		ImageURL:    "https://example.com/item.png",
		ShopLogoURL: "https://example.com/logo.png",
		Description: "desc",
		Category:    "bakery",
		Duration:    10,
		StartBid:    200,
		PickupStart: time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339),
		PickupEnd:   time.Now().UTC().Add(25 * time.Hour).Format(time.RFC3339),
	}
}

// --- auction creation / validation ---

func TestCreateAuction_Success(t *testing.T) {
	repo := newMockAuctionRepo()
	svc := newTestService(repo)

	a, err := svc.CreateAuction(context.Background(), validCreateRequest(), "seller-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.AuctionID == "" {
		t.Fatal("expected non-empty auction_id")
	}
	if a.SellerID != "seller-1" {
		t.Fatalf("seller mismatch: got %s", a.SellerID)
	}
	if a.Status != "OPEN" {
		t.Fatalf("expected OPEN, got %s", a.Status)
	}
	if a.Quantity != 1 {
		t.Fatalf("expected default quantity 1, got %d", a.Quantity)
	}
	if a.PickupEnd.Before(a.PickupStart) || a.PickupEnd.Equal(a.PickupStart) {
		t.Fatal("expected pickup_end after pickup_start")
	}
}

func TestCreateAuction_ScheduledStartCreatesPendingAuction(t *testing.T) {
	repo := newMockAuctionRepo()
	svc := newTestService(repo)
	req := validCreateRequest()
	req.ScheduledStart = time.Now().UTC().Add(2 * time.Hour).Format(time.RFC3339)

	a, err := svc.CreateAuction(context.Background(), req, "seller-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.Status != "PENDING" {
		t.Fatalf("expected PENDING, got %s", a.Status)
	}
	if !a.StartTime.After(time.Now().UTC()) {
		t.Fatal("expected scheduled start time in the future")
	}
}

func TestCreateAuction_RequiresPickupWindow(t *testing.T) {
	repo := newMockAuctionRepo()
	svc := newTestService(repo)
	req := validCreateRequest()
	req.PickupEnd = ""

	_, err := svc.CreateAuction(context.Background(), req, "seller-1")
	if !errors.Is(err, auction.ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
}

func TestCreateAuction_MaxPriceMustExceedStartBid(t *testing.T) {
	repo := newMockAuctionRepo()
	svc := newTestService(repo)
	req := validCreateRequest()
	req.MaxPrice = req.StartBid

	_, err := svc.CreateAuction(context.Background(), req, "seller-1")
	if !errors.Is(err, auction.ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
}

func TestCreateAuction_RejectsPastScheduledStart(t *testing.T) {
	repo := newMockAuctionRepo()
	svc := newTestService(repo)
	req := validCreateRequest()
	req.ScheduledStart = time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)

	_, err := svc.CreateAuction(context.Background(), req, "seller-1")
	if !errors.Is(err, auction.ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
}

// --- lookup / listing / open transitions ---

func TestGetAuction_NotFound(t *testing.T) {
	svc := newTestService(newMockAuctionRepo())

	_, err := svc.GetAuction(context.Background(), "missing")
	if !errors.Is(err, auction.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestListAuctions_ByStatus(t *testing.T) {
	repo := newMockAuctionRepo()
	ctx := context.Background()
	_ = repo.Create(ctx, &auction.Auction{AuctionID: "a1", Status: "OPEN"})
	_ = repo.Create(ctx, &auction.Auction{AuctionID: "a2", Status: "CLOSED"})
	_ = repo.Create(ctx, &auction.Auction{AuctionID: "a3", Status: "OPEN"})
	svc := newTestService(repo)

	open, err := svc.ListAuctions(ctx, "OPEN")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(open) != 2 {
		t.Fatalf("expected 2 open auctions, got %d", len(open))
	}

	all, err := svc.ListAuctions(ctx, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 auctions, got %d", len(all))
	}
}

func TestListAuctionsByShop(t *testing.T) {
	repo := newMockAuctionRepo()
	ctx := context.Background()
	_ = repo.Create(ctx, &auction.Auction{AuctionID: "a1", ShopID: "shop-1"})
	_ = repo.Create(ctx, &auction.Auction{AuctionID: "a2", ShopID: "shop-1"})
	_ = repo.Create(ctx, &auction.Auction{AuctionID: "a3", ShopID: "shop-2"})
	svc := newTestService(repo)

	auctions, err := svc.ListAuctionsByShop(ctx, "shop-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(auctions) != 2 {
		t.Fatalf("expected 2 auctions for shop-1, got %d", len(auctions))
	}
}

func TestOpenAuction_Success(t *testing.T) {
	repo := newMockAuctionRepo()
	ctx := context.Background()
	_ = repo.Create(ctx, &auction.Auction{AuctionID: "a1", Status: "PENDING"})
	svc := newTestService(repo)

	if err := svc.OpenAuction(ctx, "a1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, _ := repo.GetByID(ctx, "a1")
	if got.Status != "OPEN" {
		t.Fatalf("expected OPEN, got %s", got.Status)
	}
}

// --- reliable close sequence ---

func TestCloseAuction_PublishFailureRollsBackStatus(t *testing.T) {
	repo := newMockAuctionRepo()
	ctx := context.Background()
	_ = repo.Create(ctx, &auction.Auction{
		AuctionID:      "a1",
		ItemID:         "item-1",
		ItemTitle:      "Item",
		ShopID:         "shop-1",
		HighestBidder:  "buyer-1",
		CurrentHighest: 500,
		Quantity:       1,
		Status:         "OPEN",
	})
	svc := newClosingTestService(repo)

	err := svc.CloseAuction(ctx, "a1")
	if err == nil {
		t.Fatal("expected publish failure from invalid redis client")
	}
	got, _ := repo.GetByID(ctx, "a1")
	if got.Status != "OPEN" {
		t.Fatalf("expected rollback to OPEN, got %s", got.Status)
	}
}

// --- metrics / constants / sentinel errors ---

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
		t.Fatalf("expected 2 successful bids, got %d", snap.SuccessfulBids)
	}
	if snap.RejectedBids != 1 {
		t.Fatalf("expected 1 rejected bid, got %d", snap.RejectedBids)
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
	if !errors.Is(auction.ErrValidation, auction.ErrValidation) {
		t.Fatal("ErrValidation should match itself")
	}
}

var _ auction.Repo = (*mockAuctionRepo)(nil)
