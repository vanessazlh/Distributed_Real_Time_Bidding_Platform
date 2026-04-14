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
	repo      paymentRepo
	publisher paymentPublisher
}

type paymentRepo interface {
	Create(ctx context.Context, p *Payment) error
	GetByID(ctx context.Context, paymentID string) (*Payment, error)
	GetByAuctionID(ctx context.Context, auctionID string) (*Payment, error)
	GetAllByAuctionID(ctx context.Context, auctionID string) (map[string]bool, error)
	GetByUserID(ctx context.Context, userID string) ([]*Payment, error)
	UpdateStatus(ctx context.Context, paymentID, status, failReason string) error
	SetGatewayDecision(ctx context.Context, paymentID, decision string) error
}

type paymentPublisher interface {
	PublishPaymentProcessed(ctx context.Context, event events.PaymentProcessedEvent) error
	PublishPaymentFailed(ctx context.Context, event events.PaymentFailedEvent) error
	PublishRefundProcessed(ctx context.Context, event events.RefundProcessedEvent) error
}

// NewService creates a new Service.
func NewService(repo paymentRepo, publisher paymentPublisher) *Service {
	return &Service{repo: repo, publisher: publisher}
}

// InitiatePayment creates a pending payment for the winning bidder(s) and processes them.
// Implements events.PaymentInitiator — called by the event consumer.
//
// For single-winner auctions: creates one payment using WinnerID/WinningBid.
// For multi-winner auctions (Quantity > 1): creates one payment per entry in Winners map.
func (s *Service) InitiatePayment(ctx context.Context, event events.AuctionClosedEvent) error {
	// Build the list of winners to process
	winners := make(map[string]int64)
	if len(event.Winners) > 0 {
		// Multi-winner auction
		for bidder, amount := range event.Winners {
			winners[bidder] = amount
		}
	} else if event.WinnerID != "" {
		// Single-winner (backwards compatible)
		winners[event.WinnerID] = event.WinningBid
	}

	if len(winners) == 0 {
		log.Printf("payment: auction %s has no winner, skipping", event.AuctionID)
		return nil
	}

	// Idempotency: check which winners already have a payment.
	// For multi-winner auctions this prevents partial-failure retries from
	// duplicating payments that were already created successfully.
	alreadyPaid, err := s.repo.GetAllByAuctionID(ctx, event.AuctionID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return fmt.Errorf("check existing payments: %w", err)
	}

	// Filter out winners who already have a payment
	pending := make(map[string]int64, len(winners))
	for bidder, amount := range winners {
		if !alreadyPaid[bidder] {
			pending[bidder] = amount
		}
	}
	if len(pending) == 0 {
		log.Printf("payment: auction %s — all %d winner(s) already have payments, skipping", event.AuctionID, len(winners))
		return nil
	}

	now := time.Now().UTC().Format(time.RFC3339)
	var firstErr error

	for bidderID, amount := range pending {
		p := &Payment{
			PaymentID: uuid.New().String(),
			AuctionID: event.AuctionID,
			UserID:    bidderID,
			ItemID:    event.ItemID,
			ShopID:    event.ShopID,
			Amount:    amount,
			Status:    StatusPending,
			CreatedAt: now,
			UpdatedAt: now,
		}

		if err := s.repo.Create(ctx, p); err != nil {
			log.Printf("payment: create payment for bidder %s on auction %s failed: %v", bidderID, event.AuctionID, err)
			if firstErr == nil {
				firstErr = fmt.Errorf("create payment record: %w", err)
			}
			continue
		}
		log.Printf("payment: created payment %s for auction %s bidder %s (amount=%d)", p.PaymentID, event.AuctionID, bidderID, p.Amount)

		// Immediately attempt to process each payment
		if err := s.ProcessPayment(ctx, p.PaymentID); err != nil {
			log.Printf("payment: process payment %s failed: %v", p.PaymentID, err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}

	return firstErr
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
