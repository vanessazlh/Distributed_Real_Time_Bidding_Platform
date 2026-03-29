package auction

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"rtb/services/auction/internal/auction/concurrency"
	localEvents "rtb/services/auction/internal/events"
	"rtb/shared/events"
)

// ConcurrencyStrategy defines which concurrency control to use.
type ConcurrencyStrategy string

const (
	Optimistic  ConcurrencyStrategy = "optimistic"
	Pessimistic ConcurrencyStrategy = "pessimistic"
	Queue       ConcurrencyStrategy = "queue"
)

// ErrNotFound is returned when an auction cannot be found.
var ErrNotFound = errors.New("auction not found")

// ErrAuctionClosed is returned when bidding on a closed auction.
var ErrAuctionClosed = errors.New("auction is not open")

// ErrBidTooLow is returned when the bid is not higher than the current highest.
var ErrBidTooLow = errors.New("bid too low")

// Repo is the interface the service depends on.
type Repo interface {
	Create(ctx context.Context, a *Auction) error
	GetByID(ctx context.Context, auctionID string) (*Auction, error)
	List(ctx context.Context, status string) ([]*Auction, error)
	Close(ctx context.Context, auctionID string) error
}

// Service contains business logic for the auction domain.
type Service struct {
	repo      Repo
	publisher *localEvents.Publisher
	metrics   *Metrics

	strategy    ConcurrencyStrategy
	optimistic  *concurrency.Optimistic
	pessimistic *concurrency.Pessimistic
	queue       *concurrency.Queue
}

// NewService creates a new Service.
func NewService(repo Repo, publisher *localEvents.Publisher, rdb *redis.Client, strategy ConcurrencyStrategy) *Service {
	return &Service{
		repo:        repo,
		publisher:   publisher,
		metrics:     NewMetrics(),
		strategy:    strategy,
		optimistic:  concurrency.NewOptimistic(rdb),
		pessimistic: concurrency.NewPessimistic(rdb),
		queue:       concurrency.NewQueue(rdb),
	}
}

// GetStrategy returns the current concurrency strategy.
func (s *Service) GetStrategy() ConcurrencyStrategy {
	return s.strategy
}

// SetStrategy switches the concurrency strategy.
func (s *Service) SetStrategy(strategy ConcurrencyStrategy) {
	s.strategy = strategy
}

// GetMetrics returns the current bid metrics.
func (s *Service) GetMetrics() *BidMetrics {
	return s.metrics.Snapshot()
}

// ResetMetrics resets the metrics counters.
func (s *Service) ResetMetrics() {
	s.metrics.Reset()
}

// CreateAuction creates a new auction.
func (s *Service) CreateAuction(ctx context.Context, req CreateAuctionRequest) (*Auction, error) {
	now := time.Now().UTC()
	a := &Auction{
		AuctionID:      uuid.NewString(),
		ItemID:         req.ItemID,
		ItemTitle:      req.ItemTitle,
		ShopID:         req.ShopID,
		StartTime:      now,
		EndTime:        now.Add(time.Duration(req.Duration) * time.Minute),
		CurrentHighest: req.StartBid,
		HighestBidder:  "",
		Status:         "OPEN",
		Version:        0,
	}

	if err := s.repo.Create(ctx, a); err != nil {
		return nil, fmt.Errorf("create auction: %w", err)
	}
	return a, nil
}

// GetAuction returns an auction by ID.
func (s *Service) GetAuction(ctx context.Context, auctionID string) (*Auction, error) {
	a, err := s.repo.GetByID(ctx, auctionID)
	if err != nil {
		return nil, ErrNotFound
	}
	return a, nil
}

// ListAuctions returns auctions filtered by status.
func (s *Service) ListAuctions(ctx context.Context, status string) ([]*Auction, error) {
	auctions, err := s.repo.List(ctx, status)
	if err != nil {
		return nil, fmt.Errorf("list auctions: %w", err)
	}
	return auctions, nil
}

// PlaceBid places a bid on an auction using the current concurrency strategy.
// Bid history is recorded asynchronously by the Bid Service via the bid_placed event.
func (s *Service) PlaceBid(ctx context.Context, auctionID string, userID string, amount int64) (*BidResult, error) {
	start := time.Now()

	// Get current auction state for event publishing
	a, err := s.repo.GetByID(ctx, auctionID)
	if err != nil {
		s.metrics.RecordRejected()
		return nil, ErrNotFound
	}

	previousHighest := a.CurrentHighest
	previousBidder := a.HighestBidder

	// Use selected concurrency strategy to atomically update
	var newVersion int64
	switch s.strategy {
	case Optimistic:
		newVersion, err = s.optimistic.TryPlaceBid(ctx, auctionID, amount, userID)
	case Pessimistic:
		newVersion, err = s.pessimistic.TryPlaceBid(ctx, auctionID, amount, userID)
	case Queue:
		newVersion, err = s.queue.TryPlaceBid(ctx, auctionID, amount, userID)
	default:
		newVersion, err = s.optimistic.TryPlaceBid(ctx, auctionID, amount, userID)
	}

	latency := time.Since(start)

	if err != nil {
		s.metrics.RecordRejected()
		// Map concurrency errors to domain errors
		errMsg := err.Error()
		if contains(errMsg, "not open") {
			return nil, ErrAuctionClosed
		}
		if contains(errMsg, "must be higher") {
			return nil, ErrBidTooLow
		}
		return nil, fmt.Errorf("place bid: %w", err)
	}

	s.metrics.RecordSuccessful(latency)

	// Publish event — Bid Service consumes this to record bid history
	bidID := uuid.NewString()
	now := time.Now().UTC()
	_ = s.publisher.PublishBidPlaced(ctx, events.BidPlacedEvent{
		AuctionID:       auctionID,
		BidID:           bidID,
		ItemID:          a.ItemID,
		ItemTitle:       a.ItemTitle,
		UserID:          userID,
		Amount:          amount,
		PreviousHighest: previousHighest,
		PreviousBidder:  previousBidder,
		BidAcceptedAt:   now.Format(time.RFC3339Nano),
		Timestamp:       now.Format(time.RFC3339Nano),
	})

	_ = newVersion // used internally by concurrency strategies

	return &BidResult{
		BidID:     bidID,
		AuctionID: auctionID,
		Amount:    amount,
		Status:    "ACCEPTED",
	}, nil
}

// CloseAuction closes an auction.
func (s *Service) CloseAuction(ctx context.Context, auctionID string) error {
	a, err := s.repo.GetByID(ctx, auctionID)
	if err != nil {
		return ErrNotFound
	}

	if err := s.repo.Close(ctx, auctionID); err != nil {
		return fmt.Errorf("close auction: %w", err)
	}

	// Stop the queue processor if using queue strategy
	s.queue.Stop(auctionID)

	// Publish event
	_ = s.publisher.PublishAuctionClosed(ctx, events.AuctionClosedEvent{
		AuctionID:  auctionID,
		WinnerID:   a.HighestBidder,
		WinningBid: a.CurrentHighest,
		ItemID:     a.ItemID,
		ShopID:     a.ShopID,
		ClosedAt:   time.Now().UTC().Format(time.RFC3339Nano),
	})

	return nil
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
