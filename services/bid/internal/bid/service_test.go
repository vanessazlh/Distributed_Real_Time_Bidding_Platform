// Package bid_test exercises exported bid service behavior through in-memory
// test doubles instead of real Redis storage.
package bid_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"rtb/services/bid/internal/bid"
)

// mockRepo is an in-memory implementation of bid.Repo used by service-level
// unit tests. Optional error fields let tests verify error wrapping paths.
type mockRepo struct {
	mu      sync.Mutex
	bids    map[string]*bid.Bid
	auction map[string][]string
	user    map[string][]string

	createErr               error
	getByAuctionErr         error
	getByUserErr            error
	markOutbidErr           error
	markUserPreviousBidsErr error
	markWonErr              error
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		bids:    make(map[string]*bid.Bid),
		auction: make(map[string][]string),
		user:    make(map[string][]string),
	}
}

func (m *mockRepo) Create(_ context.Context, b *bid.Bid) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *b
	m.bids[b.BidID] = &cp
	m.auction[b.AuctionID] = append(m.auction[b.AuctionID], b.BidID)
	m.user[b.UserID] = append(m.user[b.UserID], b.BidID)
	return nil
}

func (m *mockRepo) GetByAuction(_ context.Context, auctionID string) ([]*bid.Bid, error) {
	if m.getByAuctionErr != nil {
		return nil, m.getByAuctionErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	ids := m.auction[auctionID]
	result := make([]*bid.Bid, 0, len(ids))
	for _, id := range ids {
		if b, ok := m.bids[id]; ok {
			cp := *b
			result = append(result, &cp)
		}
	}
	return result, nil
}

func (m *mockRepo) GetByUser(_ context.Context, userID string) ([]*bid.Bid, error) {
	if m.getByUserErr != nil {
		return nil, m.getByUserErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	ids := m.user[userID]
	result := make([]*bid.Bid, 0, len(ids))
	for _, id := range ids {
		if b, ok := m.bids[id]; ok {
			cp := *b
			result = append(result, &cp)
		}
	}
	return result, nil
}

func (m *mockRepo) MarkWon(_ context.Context, auctionID string, winnerID string) error {
	if m.markWonErr != nil {
		return m.markWonErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	ids := m.auction[auctionID]
	for _, id := range ids {
		if b, ok := m.bids[id]; ok && b.UserID == winnerID && b.Status == "ACCEPTED" {
			b.Status = "WON"
			return nil
		}
	}
	return nil
}

func (m *mockRepo) MarkUserPreviousBids(_ context.Context, auctionID string, userID string, excludeBidID string) error {
	if m.markUserPreviousBidsErr != nil {
		return m.markUserPreviousBidsErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	ids := m.auction[auctionID]
	for _, id := range ids {
		if id == excludeBidID {
			continue
		}
		if b, ok := m.bids[id]; ok && b.UserID == userID && b.Status == "ACCEPTED" {
			b.Status = "OUTBID"
		}
	}
	return nil
}

func (m *mockRepo) MarkOutbid(_ context.Context, auctionID string, excludeBidID string) error {
	if m.markOutbidErr != nil {
		return m.markOutbidErr
	}
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

func newTestService(repo bid.Repo) *bid.Service {
	return bid.NewService(repo)
}

func newAcceptedBid(bidID, auctionID, userID string, amount int64) *bid.Bid {
	return &bid.Bid{
		BidID:     bidID,
		AuctionID: auctionID,
		UserID:    userID,
		Amount:    amount,
		Timestamp: time.Now().UTC(),
		Status:    "ACCEPTED",
	}
}

// --- record bid flow ---

func TestRecordBid_Success(t *testing.T) {
	repo := newMockRepo()
	svc := newTestService(repo)
	b := newAcceptedBid("bid-1", "auction-1", "user-1", 1000)

	if err := svc.RecordBid(context.Background(), b); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	bids, err := svc.GetAuctionBids(context.Background(), "auction-1")
	if err != nil {
		t.Fatalf("get auction bids: %v", err)
	}
	if len(bids) != 1 {
		t.Fatalf("expected 1 bid, got %d", len(bids))
	}
	if bids[0].Status != "ACCEPTED" {
		t.Fatalf("expected ACCEPTED, got %s", bids[0].Status)
	}
}

func TestRecordBid_SelfRebidMarksPreviousBidOutbid(t *testing.T) {
	repo := newMockRepo()
	svc := newTestService(repo)
	ctx := context.Background()

	if err := svc.RecordBid(ctx, newAcceptedBid("bid-1", "a1", "u1", 100)); err != nil {
		t.Fatalf("record first bid: %v", err)
	}
	if err := svc.RecordBid(ctx, newAcceptedBid("bid-2", "a1", "u1", 200)); err != nil {
		t.Fatalf("record second bid: %v", err)
	}

	bids, err := svc.GetUserBids(ctx, "u1")
	if err != nil {
		t.Fatalf("get user bids: %v", err)
	}

	statusByID := map[string]string{}
	for _, b := range bids {
		statusByID[b.BidID] = b.Status
	}
	if statusByID["bid-1"] != "OUTBID" {
		t.Fatalf("expected bid-1 OUTBID, got %s", statusByID["bid-1"])
	}
	if statusByID["bid-2"] != "ACCEPTED" {
		t.Fatalf("expected bid-2 ACCEPTED, got %s", statusByID["bid-2"])
	}
}

func TestRecordBid_MarksOutbid(t *testing.T) {
	repo := newMockRepo()
	svc := newTestService(repo)
	ctx := context.Background()

	_ = svc.RecordBid(ctx, newAcceptedBid("bid-1", "a1", "u1", 100))
	_ = svc.RecordBid(ctx, newAcceptedBid("bid-2", "a1", "u2", 200))

	bids, err := svc.GetAuctionBids(ctx, "a1")
	if err != nil {
		t.Fatalf("get auction bids: %v", err)
	}
	statusByID := map[string]string{}
	for _, b := range bids {
		statusByID[b.BidID] = b.Status
	}
	if statusByID["bid-1"] != "OUTBID" {
		t.Fatalf("expected bid-1 OUTBID, got %s", statusByID["bid-1"])
	}
	if statusByID["bid-2"] != "ACCEPTED" {
		t.Fatalf("expected bid-2 ACCEPTED, got %s", statusByID["bid-2"])
	}
}

func TestRecordBid_MarkUserPreviousBidsError(t *testing.T) {
	repo := newMockRepo()
	repo.markUserPreviousBidsErr = errors.New("boom")
	svc := newTestService(repo)

	err := svc.RecordBid(context.Background(), newAcceptedBid("bid-1", "a1", "u1", 100))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "mark user previous bids") {
		t.Fatalf("expected wrapped error, got %v", err)
	}
}

func TestRecordBid_CreateError(t *testing.T) {
	repo := newMockRepo()
	repo.createErr = errors.New("boom")
	svc := newTestService(repo)

	err := svc.RecordBid(context.Background(), newAcceptedBid("bid-1", "a1", "u1", 100))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "record bid") {
		t.Fatalf("expected wrapped error, got %v", err)
	}
}

func TestRecordBid_MarkOutbidError(t *testing.T) {
	repo := newMockRepo()
	repo.markOutbidErr = errors.New("boom")
	svc := newTestService(repo)

	err := svc.RecordBid(context.Background(), newAcceptedBid("bid-1", "a1", "u1", 100))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "mark outbid") {
		t.Fatalf("expected wrapped error, got %v", err)
	}
}

// --- winner marking ---

func TestMarkWinnerBid_Success(t *testing.T) {
	repo := newMockRepo()
	svc := newTestService(repo)
	ctx := context.Background()

	_ = svc.RecordBid(ctx, newAcceptedBid("bid-1", "a1", "u1", 100))
	_ = svc.RecordBid(ctx, newAcceptedBid("bid-2", "a1", "u2", 200))

	if err := svc.MarkWinnerBid(ctx, "a1", "u2"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	bids, err := svc.GetAuctionBids(ctx, "a1")
	if err != nil {
		t.Fatalf("get auction bids: %v", err)
	}
	statusByID := map[string]string{}
	for _, b := range bids {
		statusByID[b.BidID] = b.Status
	}
	if statusByID["bid-2"] != "WON" {
		t.Fatalf("expected bid-2 WON, got %s", statusByID["bid-2"])
	}
}

func TestMarkWinnerBid_Error(t *testing.T) {
	repo := newMockRepo()
	repo.markWonErr = errors.New("boom")
	svc := newTestService(repo)

	err := svc.MarkWinnerBid(context.Background(), "a1", "u1")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "mark winner bid") {
		t.Fatalf("expected wrapped error, got %v", err)
	}
}

// --- query helpers ---

func TestGetAuctionBids(t *testing.T) {
	repo := newMockRepo()
	svc := newTestService(repo)
	ctx := context.Background()

	_ = svc.RecordBid(ctx, newAcceptedBid("b1", "a1", "u1", 100))
	_ = svc.RecordBid(ctx, newAcceptedBid("b2", "a1", "u2", 200))
	_ = svc.RecordBid(ctx, newAcceptedBid("b3", "a2", "u1", 50))

	bids, err := svc.GetAuctionBids(ctx, "a1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(bids) != 2 {
		t.Fatalf("expected 2 bids for auction a1, got %d", len(bids))
	}
}

func TestGetAuctionBids_Error(t *testing.T) {
	repo := newMockRepo()
	repo.getByAuctionErr = errors.New("boom")
	svc := newTestService(repo)

	_, err := svc.GetAuctionBids(context.Background(), "a1")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "get auction bids") {
		t.Fatalf("expected wrapped error, got %v", err)
	}
}

func TestGetUserBids(t *testing.T) {
	repo := newMockRepo()
	svc := newTestService(repo)
	ctx := context.Background()

	_ = svc.RecordBid(ctx, newAcceptedBid("b1", "a1", "u1", 100))
	_ = svc.RecordBid(ctx, newAcceptedBid("b2", "a2", "u1", 200))

	bids, err := svc.GetUserBids(ctx, "u1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(bids) != 2 {
		t.Fatalf("expected 2 bids for user u1, got %d", len(bids))
	}
}

func TestGetUserBids_Empty(t *testing.T) {
	svc := newTestService(newMockRepo())

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

func TestGetUserBids_Error(t *testing.T) {
	repo := newMockRepo()
	repo.getByUserErr = errors.New("boom")
	svc := newTestService(repo)

	_, err := svc.GetUserBids(context.Background(), "u1")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "get user bids") {
		t.Fatalf("expected wrapped error, got %v", err)
	}
}

// --- sentinel errors ---

func TestErrNotFound(t *testing.T) {
	if !errors.Is(bid.ErrNotFound, bid.ErrNotFound) {
		t.Fatal("ErrNotFound should match itself")
	}
}

var _ bid.Repo = (*mockRepo)(nil)
