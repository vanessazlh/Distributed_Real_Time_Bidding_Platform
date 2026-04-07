package payment

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/google/uuid"
	"rtb/services/payment/internal/events"
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
// Also accepts PROCESSING to support retries of records stuck mid-flight
// (e.g. after a crash between status updates).
func (s *Service) ProcessPayment(ctx context.Context, paymentID string) error {
	p, err := s.repo.GetByID(ctx, paymentID)
	if err != nil {
		return err
	}
	if p.Status != StatusPending && p.Status != StatusProcessing {
		return fmt.Errorf("%w: cannot process a payment in status %q", ErrInvalidStatus, p.Status)
	}

	// Transition PENDING → PROCESSING (skip if already PROCESSING from a prior attempt).
	if p.Status == StatusPending {
		if err := s.repo.UpdateStatus(ctx, paymentID, StatusProcessing, ""); err != nil {
			return fmt.Errorf("mark processing: %w", err)
		}
	}

	// Determine the gateway outcome. If gateway_decision is already set, a prior
	// attempt reached the gateway and we reuse its result to avoid re-randomizing.
	//
	// NOTE: this is a mock simplification. A real gateway (e.g. Stripe) stores the
	// idempotency result on its own side; the caller would query the gateway using
	// paymentID as the idempotency key and get back the same outcome on retry.
	decision := p.GatewayDecision
	if decision == "" {
		if rand.Intn(10) < 9 {
			decision = "success"
		} else {
			decision = "failed"
		}
		if err := s.repo.SetGatewayDecision(ctx, paymentID, decision); err != nil {
			return fmt.Errorf("record gateway decision: %w", err)
		}
	}

	success := decision == "success"
	if success {
		if err := s.repo.UpdateStatus(ctx, paymentID, StatusCompleted, ""); err != nil {
			return fmt.Errorf("mark completed: %w", err)
		}
		log.Printf("payment: %s completed", paymentID)
		if err := s.publisher.PublishPaymentProcessed(ctx, events.PaymentProcessedEvent{
			PaymentID:   paymentID,
			AuctionID:   p.AuctionID,
			UserID:      p.UserID,
			Amount:      p.Amount,
			ProcessedAt: time.Now().UTC().Format(time.RFC3339),
		}); err != nil {
			log.Printf("payment: failed to publish payment_processed for %s: %v", paymentID, err)
		}
	} else {
		reason := "payment gateway declined"
		if err := s.repo.UpdateStatus(ctx, paymentID, StatusFailed, reason); err != nil {
			return fmt.Errorf("mark failed: %w", err)
		}
		log.Printf("payment: %s failed (%s)", paymentID, reason)
		if err := s.publisher.PublishPaymentFailed(ctx, events.PaymentFailedEvent{
			PaymentID: paymentID,
			AuctionID: p.AuctionID,
			UserID:    p.UserID,
			Amount:    p.Amount,
			Reason:    reason,
			FailedAt:  time.Now().UTC().Format(time.RFC3339),
		}); err != nil {
			log.Printf("payment: failed to publish payment_failed for %s: %v", paymentID, err)
		}
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

	if err := s.publisher.PublishRefundProcessed(ctx, events.RefundProcessedEvent{
		PaymentID:  paymentID,
		AuctionID:  p.AuctionID,
		UserID:     p.UserID,
		Amount:     p.Amount,
		RefundedAt: time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		log.Printf("payment: failed to publish refund_processed for %s: %v", paymentID, err)
	}
	return nil
}

// AbandonPayment marks a payment as FAILED and publishes a payment_failed event.
// Called by the recovery job when a stuck payment has exhausted its retry budget.
// The published event is what triggers downstream user notification.
func (s *Service) AbandonPayment(ctx context.Context, paymentID, reason string) error {
	p, err := s.repo.GetByID(ctx, paymentID)
	if err != nil {
		return err
	}
	if err := s.repo.UpdateStatus(ctx, paymentID, StatusFailed, reason); err != nil {
		return fmt.Errorf("abandon payment: %w", err)
	}
	log.Printf("payment: %s abandoned after max retries (%s)", paymentID, reason)
	if err := s.publisher.PublishPaymentFailed(ctx, events.PaymentFailedEvent{
		PaymentID: paymentID,
		AuctionID: p.AuctionID,
		UserID:    p.UserID,
		Amount:    p.Amount,
		Reason:    reason,
		FailedAt:  time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		log.Printf("payment: failed to publish payment_failed for abandoned payment %s: %v", paymentID, err)
	}
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
