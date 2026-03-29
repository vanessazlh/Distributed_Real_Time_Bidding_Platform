package payment

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/google/uuid"
	"github.com/surplus-auction/platform/internal/events"
)

// Service handles payment business logic.
type Service struct {
	repo      *Repository
	publisher *events.Publisher
}

// NewService creates a new Service.
func NewService(repo *Repository, publisher *events.Publisher) *Service {
	return &Service{repo: repo, publisher: publisher}
}

// InitiatePayment creates a pending payment for the winning bidder and processes it.
// Implements events.PaymentInitiator — called by the event consumer.
func (s *Service) InitiatePayment(ctx context.Context, event events.AuctionClosedEvent) error {
	if event.WinnerID == "" {
		log.Printf("payment: auction %s has no winner, skipping", event.AuctionID)
		return nil
	}

	// Idempotency: skip if a payment already exists for this auction.
	existing, err := s.repo.GetByAuctionID(ctx, event.AuctionID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return fmt.Errorf("check existing payment: %w", err)
	}
	if existing != nil {
		log.Printf("payment: auction %s already has payment %s, skipping", event.AuctionID, existing.PaymentID)
		return nil
	}

	now := time.Now().UTC().Format(time.RFC3339)
	p := &Payment{
		PaymentID: uuid.New().String(),
		AuctionID: event.AuctionID,
		UserID:    event.WinnerID,
		ItemID:    event.ItemID,
		ShopID:    event.ShopID,
		Amount:    event.WinningBid,
		Status:    StatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.repo.Create(ctx, p); err != nil {
		return fmt.Errorf("create payment record: %w", err)
	}
	log.Printf("payment: created payment %s for auction %s (amount=%d)", p.PaymentID, p.AuctionID, p.Amount)

	// Immediately attempt to process.
	return s.ProcessPayment(ctx, p.PaymentID)
}

// ProcessPayment transitions a pending payment to completed or failed.
func (s *Service) ProcessPayment(ctx context.Context, paymentID string) error {
	p, err := s.repo.GetByID(ctx, paymentID)
	if err != nil {
		return err
	}
	if p.Status != StatusPending {
		return fmt.Errorf("%w: cannot process a payment in status %q", ErrInvalidStatus, p.Status)
	}

	// Mark as processing.
	if err := s.repo.UpdateStatus(ctx, paymentID, StatusProcessing, ""); err != nil {
		return fmt.Errorf("mark processing: %w", err)
	}

	// Simulate payment gateway: 90% success rate.
	success := rand.Intn(10) < 9
	if success {
		if err := s.repo.UpdateStatus(ctx, paymentID, StatusCompleted, ""); err != nil {
			return fmt.Errorf("mark completed: %w", err)
		}
		log.Printf("payment: %s completed", paymentID)
		_ = s.publisher.PublishPaymentProcessed(ctx, events.PaymentProcessedEvent{
			PaymentID:   paymentID,
			AuctionID:   p.AuctionID,
			UserID:      p.UserID,
			Amount:      p.Amount,
			ProcessedAt: time.Now().UTC(),
		})
	} else {
		reason := "payment gateway declined"
		if err := s.repo.UpdateStatus(ctx, paymentID, StatusFailed, reason); err != nil {
			return fmt.Errorf("mark failed: %w", err)
		}
		log.Printf("payment: %s failed (%s)", paymentID, reason)
		_ = s.publisher.PublishPaymentFailed(ctx, events.PaymentFailedEvent{
			PaymentID: paymentID,
			AuctionID: p.AuctionID,
			UserID:    p.UserID,
			Amount:    p.Amount,
			Reason:    reason,
			FailedAt:  time.Now().UTC(),
		})
	}
	return nil
}

// RefundPayment transitions a completed payment to refunded.
func (s *Service) RefundPayment(ctx context.Context, paymentID string) error {
	p, err := s.repo.GetByID(ctx, paymentID)
	if err != nil {
		return err
	}
	if p.Status != StatusCompleted && p.Status != StatusFailed {
		return fmt.Errorf("%w: cannot refund a payment in status %q", ErrInvalidStatus, p.Status)
	}

	if err := s.repo.UpdateStatus(ctx, paymentID, StatusRefunded, ""); err != nil {
		return fmt.Errorf("mark refunded: %w", err)
	}
	log.Printf("payment: %s refunded", paymentID)

	_ = s.publisher.PublishRefundProcessed(ctx, events.RefundProcessedEvent{
		PaymentID:  paymentID,
		AuctionID:  p.AuctionID,
		UserID:     p.UserID,
		Amount:     p.Amount,
		RefundedAt: time.Now().UTC(),
	})
	return nil
}

// GetPayment retrieves a payment by ID.
func (s *Service) GetPayment(ctx context.Context, paymentID string) (Response, error) {
	p, err := s.repo.GetByID(ctx, paymentID)
	if err != nil {
		return Response{}, err
	}
	return toResponse(p), nil
}

// GetPaymentByAuction retrieves the payment for a given auction.
func (s *Service) GetPaymentByAuction(ctx context.Context, auctionID string) (Response, error) {
	p, err := s.repo.GetByAuctionID(ctx, auctionID)
	if err != nil {
		return Response{}, err
	}
	return toResponse(p), nil
}

// GetUserPayments retrieves all payments for a given user.
func (s *Service) GetUserPayments(ctx context.Context, userID string) ([]Response, error) {
	payments, err := s.repo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	result := make([]Response, len(payments))
	for i, p := range payments {
		result[i] = toResponse(p)
	}
	return result, nil
}
