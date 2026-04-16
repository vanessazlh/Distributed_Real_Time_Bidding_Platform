package auction

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"rtb/services/auction/internal/auction/concurrency"
	"rtb/services/auction/internal/auction/concurrency/experimental"
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

// ErrBidExceedsMax is returned when the bid exceeds the auction's max price.
var ErrBidExceedsMax = errors.New("bid exceeds max price")

// ErrBidIncrementTooSmall is returned when the bid doesn't meet the minimum increment.
var ErrBidIncrementTooSmall = errors.New("bid increment too small")

// ErrSelfBid is returned when a seller tries to bid on their own auction.
var ErrSelfBid = errors.New("sellers cannot bid on their own auction")

// ErrValidation is returned when auction creation input is invalid.
var ErrValidation = errors.New("validation error")

// WinnerPersister is an optional interface for persisting winner data to DynamoDB.
// Implemented by *Repository when DynamoDB is configured.
type WinnerPersister interface {
	UpdateDynamoWinner(ctx context.Context, auctionID, bidderID string, amount int64)
}

// Repo is the interface the service depends on.
type Repo interface {
	Create(ctx context.Context, a *Auction) error
	GetByID(ctx context.Context, auctionID string) (*Auction, error)
	List(ctx context.Context, status string) ([]*Auction, error)
	ListByShop(ctx context.Context, shopID string) ([]*Auction, error)
	GetShopIDsNear(ctx context.Context, lat, lng, radiusKm float64) ([]string, error)
	Open(ctx context.Context, auctionID string) error
	Close(ctx context.Context, auctionID string) error // Deprecated: use the atomic close methods below

	// Reliable close sequence
	AtomicCloseAndReadWinner(ctx context.Context, auctionID string) (winnerID string, winningBid int64, err error)
	AtomicCloseAndReadWinners(ctx context.Context, auctionID string) (winners map[string]int64, err error)
	RollbackClose(ctx context.Context, auctionID string) error
	PersistClosedState(ctx context.Context, auctionID string) error
	CleanupRedis(ctx context.Context, auctionID string) error
	GetDynamoWinners(ctx context.Context, auctionID string) (winnerID string, winningBid int64, err error)
	GetDynamoAllWinners(ctx context.Context, auctionID string) (winners map[string]int64, err error)
}

// Service contains business logic for the auction domain.
type Service struct {
	repo      Repo
	publisher *localEvents.Publisher
	metrics   *Metrics

	strategy    ConcurrencyStrategy
	pessimistic *concurrency.Pessimistic

	// Experimental strategies — kept for benchmarking and single-node dev.
	// See concurrency/experimental/ for deployment limitations.
	optimistic *experimental.Optimistic
	queue      *experimental.Queue
}

// NewService creates a new Service.
func NewService(repo Repo, publisher *localEvents.Publisher, rdb *redis.Client, strategy ConcurrencyStrategy) *Service {
	return &Service{
		repo:        repo,
		publisher:   publisher,
		metrics:     NewMetrics(),
		strategy:    strategy,
		pessimistic: concurrency.NewPessimistic(rdb),
		optimistic:  experimental.NewOptimistic(rdb),
		queue:       experimental.NewQueue(rdb),
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

// CreateAuction creates a new auction with input validation and optional scheduling.
func (s *Service) CreateAuction(ctx context.Context, req CreateAuctionRequest, sellerID string) (*Auction, error) {
	now := time.Now().UTC()

	// ── Input validation ──
	if req.StartBid < 0 {
		return nil, fmt.Errorf("%w: start_bid must be >= 0", ErrValidation)
	}
	if req.Duration < 1 {
		return nil, fmt.Errorf("%w: duration_minutes must be >= 1", ErrValidation)
	}
	if req.Duration > 10080 { // 7 days max
		return nil, fmt.Errorf("%w: duration_minutes must be <= 10080 (7 days)", ErrValidation)
	}
	if req.MaxPrice < 0 {
		return nil, fmt.Errorf("%w: max_price must be >= 0", ErrValidation)
	}
	if req.MaxPrice > 0 && req.MaxPrice <= req.StartBid {
		return nil, fmt.Errorf("%w: max_price must be greater than start_bid", ErrValidation)
	}
	if req.Quantity < 0 {
		return nil, fmt.Errorf("%w: quantity must be >= 0", ErrValidation)
	}
	if req.MinIncrement < 0 {
		return nil, fmt.Errorf("%w: min_increment must be >= 0", ErrValidation)
	}

	// Determine start time and status
	startTime := now
	status := "OPEN"

	if req.ScheduledStart != "" {
		parsed, err := time.Parse(time.RFC3339, req.ScheduledStart)
		if err != nil {
			return nil, fmt.Errorf("%w: scheduled_start must be a valid RFC3339 timestamp", ErrValidation)
		}
		if parsed.Before(now) {
			return nil, fmt.Errorf("%w: scheduled_start must be in the future", ErrValidation)
		}
		startTime = parsed.UTC()
		status = "PENDING"
	}

	endTime := startTime.Add(time.Duration(req.Duration) * time.Minute)

	// Parse required pickup window
	if req.PickupStart == "" || req.PickupEnd == "" {
		return nil, fmt.Errorf("%w: pickup_start and pickup_end are required", ErrValidation)
	}
	pickupStart, err := time.Parse(time.RFC3339, req.PickupStart)
	if err != nil {
		return nil, fmt.Errorf("%w: pickup_start must be a valid RFC3339 timestamp", ErrValidation)
	}
	pickupStart = pickupStart.UTC()
	pickupEnd, err := time.Parse(time.RFC3339, req.PickupEnd)
	if err != nil {
		return nil, fmt.Errorf("%w: pickup_end must be a valid RFC3339 timestamp", ErrValidation)
	}
	pickupEnd = pickupEnd.UTC()
	if !pickupEnd.After(pickupStart) {
		return nil, fmt.Errorf("%w: pickup_end must be after pickup_start", ErrValidation)
	}

	qty := req.Quantity
	if qty < 1 {
		qty = 1
	}

	a := &Auction{
		AuctionID:      uuid.NewString(),
		SellerID:       sellerID,
		ItemID:         req.ItemID,
		ItemTitle:      req.ItemTitle,
		ShopID:         req.ShopID,
		ShopName:       req.ShopName,
		ShopLat:        req.ShopLat,
		ShopLng:        req.ShopLng,
		RetailPrice:    req.RetailPrice,
		MaxPrice:       req.MaxPrice,
		MinIncrement:   req.MinIncrement,
		Quantity:       qty,
		ImageURL:       req.ImageURL,
		ShopLogoURL:    req.ShopLogoURL,
		Description:    req.Description,
		Category:       req.Category,
		PickupStart:    pickupStart,
		PickupEnd:      pickupEnd,
		StartTime:      startTime,
		EndTime:        endTime,
		CurrentHighest: req.StartBid,
		BidCount:       0,
		HighestBidder:  "",
		Status:         status,
		Version:        0,
	}

	if err := s.repo.Create(ctx, a); err != nil {
		return nil, fmt.Errorf("create auction: %w", err)
	}
	return a, nil
}

// OpenAuction transitions a PENDING auction to OPEN.
func (s *Service) OpenAuction(ctx context.Context, auctionID string) error {
	if err := s.repo.Open(ctx, auctionID); err != nil {
		return fmt.Errorf("open auction: %w", err)
	}
	return nil
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

// ListAuctionsByShop returns all auctions for a given shop.
func (s *Service) ListAuctionsByShop(ctx context.Context, shopID string) ([]*Auction, error) {
	auctions, err := s.repo.ListByShop(ctx, shopID)
	if err != nil {
		return nil, fmt.Errorf("list auctions by shop: %w", err)
	}
	return auctions, nil
}

// ListAuctionsNear returns active auctions whose shop is within radiusKm of (lat, lng).
func (s *Service) ListAuctionsNear(ctx context.Context, lat, lng, radiusKm float64) ([]*Auction, error) {
	shopIDs, err := s.repo.GetShopIDsNear(ctx, lat, lng, radiusKm)
	if err != nil {
		return nil, fmt.Errorf("geo search: %w", err)
	}
	if len(shopIDs) == 0 {
		return []*Auction{}, nil
	}
	allowed := make(map[string]bool, len(shopIDs))
	for _, id := range shopIDs {
		allowed[id] = true
	}
	all, err := s.repo.List(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("list auctions: %w", err)
	}
	result := make([]*Auction, 0, len(all))
	for _, a := range all {
		if allowed[a.ShopID] {
			result = append(result, a)
		}
	}
	return result, nil
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

	if a.SellerID != "" && a.SellerID == userID {
		s.metrics.RecordRejected()
		return nil, ErrSelfBid
	}

	previousHighest := a.CurrentHighest
	previousBidder := a.HighestBidder

	// Use selected concurrency strategy to atomically update
	var evictedBidder string
	switch s.strategy {
	case Pessimistic:
		placement, placeErr := s.pessimistic.TryPlaceBid(ctx, auctionID, amount, userID)
		if placeErr != nil {
			err = placeErr
		} else {
			evictedBidder = placement.EvictedBidder
		}
	case Optimistic:
		_, err = s.optimistic.TryPlaceBid(ctx, auctionID, amount, userID)
	case Queue:
		_, err = s.queue.TryPlaceBid(ctx, auctionID, amount, userID)
	default:
		placement, placeErr := s.pessimistic.TryPlaceBid(ctx, auctionID, amount, userID)
		if placeErr != nil {
			err = placeErr
		} else {
			evictedBidder = placement.EvictedBidder
		}
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
		if contains(errMsg, "exceeds max price") {
			return nil, ErrBidExceedsMax
		}
		if contains(errMsg, "increment too small") {
			return nil, ErrBidIncrementTooSmall
		}
		return nil, fmt.Errorf("place bid: %w", err)
	}

	s.metrics.RecordSuccessful(latency)

	// For quantity>1, the evicted bidder is the "outbid" party.
	// For quantity=1, fall back to the previous highest bidder from the hash.
	outbidBidder := previousBidder
	if evictedBidder != "" {
		outbidBidder = evictedBidder
	}

	// Publish event — Bid Service consumes this to record bid history
	bidID := uuid.NewString()
	now := time.Now().UTC()
	_ = s.publisher.PublishBidPlaced(ctx, events.BidPlacedEvent{
		AuctionID:       auctionID,
		BidID:           bidID,
		ItemID:          a.ItemID,
		ItemTitle:       a.ItemTitle,
		ShopName:        a.ShopName,
		UserID:          userID,
		Amount:          amount,
		PreviousHighest: previousHighest,
		PreviousBidder:  outbidBidder,
		BidAcceptedAt:   now.Format(time.RFC3339Nano),
		Timestamp:       now.Format(time.RFC3339Nano),
	})

	// Persist winner to DynamoDB asynchronously (fallback for Redis failure at close time)
	if wp, ok := s.repo.(WinnerPersister); ok {
		go wp.UpdateDynamoWinner(context.Background(), auctionID, userID, amount)
	}

	return &BidResult{
		BidID:         bidID,
		AuctionID:     auctionID,
		Amount:        amount,
		NewHighestBid: amount,
		Status:        "ACCEPTED",
	}, nil
}

// CloseAuction closes an auction using a reliable sequence:
// 1. Read metadata  2. Atomic close + read winner(s)  3. DynamoDB fallback
// 4. Publish event (rollback on failure)  5. Persist + cleanup
func (s *Service) CloseAuction(ctx context.Context, auctionID string) error {
	// Step 1: Read auction metadata (needed for event fields)
	a, err := s.repo.GetByID(ctx, auctionID)
	if err != nil {
		return ErrNotFound
	}

	qty := a.Quantity
	if qty < 1 {
		qty = 1
	}

	var winnerID string
	var winningBid int64
	var allWinners map[string]int64

	if qty == 1 {
		// Step 2a: Single-winner — original path
		winnerID, winningBid, err = s.repo.AtomicCloseAndReadWinner(ctx, auctionID)
		if err != nil {
			return fmt.Errorf("close auction: %w", err)
		}

		// Step 3a: DynamoDB fallback
		if winnerID == "" {
			if dynWinner, dynBid, dynErr := s.repo.GetDynamoWinners(ctx, auctionID); dynErr == nil && dynWinner != "" {
				winnerID = dynWinner
				winningBid = dynBid
			}
		}
	} else {
		// Step 2b: Multi-winner — read all N winners atomically
		allWinners, err = s.repo.AtomicCloseAndReadWinners(ctx, auctionID)
		if err != nil {
			return fmt.Errorf("close auction: %w", err)
		}

		// Step 3b: DynamoDB fallback for multi-winner
		if len(allWinners) == 0 {
			if dynWinners, dynErr := s.repo.GetDynamoAllWinners(ctx, auctionID); dynErr == nil && len(dynWinners) > 0 {
				allWinners = dynWinners
			}
		}

		// Set top winner for backward compatibility
		for id, amt := range allWinners {
			if amt > winningBid {
				winnerID = id
				winningBid = amt
			}
		}
	}

	// Step 4: Publish event — if this fails, rollback to OPEN so closer retries
	closedEvent := events.AuctionClosedEvent{
		AuctionID:  auctionID,
		WinnerID:   winnerID,
		WinningBid: winningBid,
		Quantity:   qty,
		ItemID:     a.ItemID,
		ItemTitle:  a.ItemTitle,
		ShopID:     a.ShopID,
		ClosedAt:   time.Now().UTC().Format(time.RFC3339Nano),
	}
	if qty > 1 && len(allWinners) > 0 {
		closedEvent.Winners = allWinners
	}

	err = s.publisher.PublishAuctionClosed(ctx, closedEvent)
	if err != nil {
		if rbErr := s.repo.RollbackClose(ctx, auctionID); rbErr != nil {
			log.Printf("CRITICAL: rollback close failed for %s after publish failure: %v (publish err: %v)",
				auctionID, rbErr, err)
		}
		return fmt.Errorf("close auction: publish event failed: %w", err)
	}

	// Step 5: Stop the queue processor
	s.queue.Stop(auctionID)

	// Step 6: Persist final CLOSED state to DynamoDB (best-effort; event already sent)
	if err := s.repo.PersistClosedState(ctx, auctionID); err != nil {
		log.Printf("WARN: persist closed state failed for %s: %v", auctionID, err)
	}

	// Step 7: Cleanup Redis keys (best-effort)
	if err := s.repo.CleanupRedis(ctx, auctionID); err != nil {
		log.Printf("WARN: redis cleanup failed for %s: %v", auctionID, err)
	}

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
