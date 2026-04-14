// Package payment_test exercises exported payment service behavior through
// in-memory test doubles instead of real DynamoDB or Redis clients.
package payment_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	paymentEvents "rtb/services/payment/internal/events"
	"rtb/services/payment/internal/payment"
)

// mockRepo is an in-memory implementation of the payment service's repository
// dependency. It supports targeted failures so tests can verify error wrapping.
type mockRepo struct {
	mu       sync.Mutex
	payments map[string]*payment.Payment

	defaultGatewayDecision string

	createErr          error
	createErrForUser   map[string]error
	getByIDErr         error
	getByAuctionErr    error
	getAllByAuctionErr error
	getByUserErr       error
	setGatewayErr      error
	updateStatusErr    map[string]error

	setGatewayDecisionCalls int
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		payments:        make(map[string]*payment.Payment),
		createErrForUser: make(map[string]error),
		updateStatusErr: make(map[string]error),
	}
}

func (m *mockRepo) Create(_ context.Context, p *payment.Payment) error {
	if m.createErr != nil {
		return m.createErr
	}
	if err := m.createErrForUser[p.UserID]; err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *p
	if cp.GatewayDecision == "" && m.defaultGatewayDecision != "" {
		cp.GatewayDecision = m.defaultGatewayDecision
	}
	m.payments[p.PaymentID] = &cp
	return nil
}

func (m *mockRepo) GetByID(_ context.Context, paymentID string) (*payment.Payment, error) {
	if m.getByIDErr != nil {
		return nil, m.getByIDErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.payments[paymentID]
	if !ok {
		return nil, payment.ErrNotFound
	}
	cp := *p
	return &cp, nil
}

func (m *mockRepo) GetByAuctionID(_ context.Context, auctionID string) (*payment.Payment, error) {
	if m.getByAuctionErr != nil {
		return nil, m.getByAuctionErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, p := range m.payments {
		if p.AuctionID == auctionID {
			cp := *p
			return &cp, nil
		}
	}
	return nil, payment.ErrNotFound
}

func (m *mockRepo) GetAllByAuctionID(_ context.Context, auctionID string) (map[string]bool, error) {
	if m.getAllByAuctionErr != nil {
		return nil, m.getAllByAuctionErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	existing := make(map[string]bool)
	for _, p := range m.payments {
		if p.AuctionID == auctionID {
			existing[p.UserID] = true
		}
	}
	if len(existing) == 0 {
		return nil, payment.ErrNotFound
	}
	return existing, nil
}

func (m *mockRepo) GetByUserID(_ context.Context, userID string) ([]*payment.Payment, error) {
	if m.getByUserErr != nil {
		return nil, m.getByUserErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*payment.Payment
	for _, p := range m.payments {
		if p.UserID == userID {
			cp := *p
			result = append(result, &cp)
		}
	}
	return result, nil
}

func (m *mockRepo) UpdateStatus(_ context.Context, paymentID, status, failReason string) error {
	if err := m.updateStatusErr[status]; err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.payments[paymentID]
	if !ok {
		return payment.ErrNotFound
	}
	p.Status = status
	p.FailReason = failReason
	p.UpdatedAt = "updated"
	return nil
}

func (m *mockRepo) SetGatewayDecision(_ context.Context, paymentID, decision string) error {
	if m.setGatewayErr != nil {
		return m.setGatewayErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.payments[paymentID]
	if !ok {
		return payment.ErrNotFound
	}
	p.GatewayDecision = decision
	m.setGatewayDecisionCalls++
	return nil
}

// mockPublisher records domain events emitted by the service.
type mockPublisher struct {
	processed []paymentEvents.PaymentProcessedEvent
	failed    []paymentEvents.PaymentFailedEvent
	refunded  []paymentEvents.RefundProcessedEvent
}

func (m *mockPublisher) PublishPaymentProcessed(_ context.Context, event paymentEvents.PaymentProcessedEvent) error {
	m.processed = append(m.processed, event)
	return nil
}

func (m *mockPublisher) PublishPaymentFailed(_ context.Context, event paymentEvents.PaymentFailedEvent) error {
	m.failed = append(m.failed, event)
	return nil
}

func (m *mockPublisher) PublishRefundProcessed(_ context.Context, event paymentEvents.RefundProcessedEvent) error {
	m.refunded = append(m.refunded, event)
	return nil
}

func newTestService(repo *mockRepo, publisher *mockPublisher) *payment.Service {
	return payment.NewService(repo, publisher)
}

func newStoredPayment(paymentID, auctionID, userID, status string) *payment.Payment {
	return &payment.Payment{
		PaymentID: paymentID,
		AuctionID: auctionID,
		UserID:    userID,
		ItemID:    "item-1",
		ShopID:    "shop-1",
		Amount:    500,
		Status:    status,
		CreatedAt: "created",
		UpdatedAt: "created",
	}
}

// --- payment initiation / idempotency ---

func TestInitiatePayment_NoWinnerSkips(t *testing.T) {
	repo := newMockRepo()
	pub := &mockPublisher{}
	svc := newTestService(repo, pub)

	err := svc.InitiatePayment(context.Background(), paymentEvents.AuctionClosedEvent{
		AuctionID: "auction-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.payments) != 0 {
		t.Fatalf("expected no payments, got %d", len(repo.payments))
	}
}

func TestInitiatePayment_MultiWinnerSkipsAlreadyPaidUsers(t *testing.T) {
	repo := newMockRepo()
	repo.defaultGatewayDecision = "success"
	repo.payments["existing"] = &payment.Payment{
		PaymentID:        "existing",
		AuctionID:        "auction-1",
		UserID:           "user-1",
		ItemID:           "item-1",
		ShopID:           "shop-1",
		Amount:           100,
		Status:           payment.StatusCompleted,
		GatewayDecision:  "success",
		CreatedAt:        "created",
		UpdatedAt:        "created",
	}
	pub := &mockPublisher{}
	svc := newTestService(repo, pub)

	err := svc.InitiatePayment(context.Background(), paymentEvents.AuctionClosedEvent{
		AuctionID: "auction-1",
		ItemID:    "item-1",
		ShopID:    "shop-1",
		Winners: map[string]int64{
			"user-1": 100,
			"user-2": 200,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.payments) != 2 {
		t.Fatalf("expected 2 total payments, got %d", len(repo.payments))
	}

	var newPayment *payment.Payment
	for _, p := range repo.payments {
		if p.UserID == "user-2" {
			newPayment = p
			break
		}
	}
	if newPayment == nil {
		t.Fatal("expected payment for user-2")
	}
	if newPayment.Status != payment.StatusCompleted {
		t.Fatalf("expected completed payment for user-2, got %s", newPayment.Status)
	}
	if len(pub.processed) != 1 || pub.processed[0].UserID != "user-2" {
		t.Fatalf("expected one processed event for user-2, got %+v", pub.processed)
	}
}

func TestInitiatePayment_CheckExistingPaymentsError(t *testing.T) {
	repo := newMockRepo()
	repo.getAllByAuctionErr = errors.New("boom")
	pub := &mockPublisher{}
	svc := newTestService(repo, pub)

	err := svc.InitiatePayment(context.Background(), paymentEvents.AuctionClosedEvent{
		AuctionID:  "auction-1",
		WinnerID:   "user-1",
		WinningBid: 100,
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "check existing payments") {
		t.Fatalf("expected wrapped error, got %v", err)
	}
}

func TestInitiatePayment_ReturnsFirstCreateErrorButContinues(t *testing.T) {
	repo := newMockRepo()
	repo.defaultGatewayDecision = "success"
	repo.createErrForUser["user-1"] = errors.New("create failed")
	pub := &mockPublisher{}
	svc := newTestService(repo, pub)

	err := svc.InitiatePayment(context.Background(), paymentEvents.AuctionClosedEvent{
		AuctionID: "auction-1",
		ItemID:    "item-1",
		ShopID:    "shop-1",
		Winners: map[string]int64{
			"user-1": 100,
			"user-2": 200,
		},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "create payment record") {
		t.Fatalf("expected wrapped error, got %v", err)
	}
	if len(repo.payments) != 1 {
		t.Fatalf("expected one successful payment creation, got %d", len(repo.payments))
	}
	if len(pub.processed) != 1 || pub.processed[0].UserID != "user-2" {
		t.Fatalf("expected one processed event for user-2, got %+v", pub.processed)
	}
}

// --- payment processing ---

func TestProcessPayment_SuccessWithStoredGatewayDecision(t *testing.T) {
	repo := newMockRepo()
	repo.payments["p1"] = &payment.Payment{
		PaymentID:       "p1",
		AuctionID:       "auction-1",
		UserID:          "user-1",
		ItemID:          "item-1",
		ShopID:          "shop-1",
		Amount:          500,
		Status:          payment.StatusPending,
		GatewayDecision: "success",
		CreatedAt:       "created",
		UpdatedAt:       "created",
	}
	pub := &mockPublisher{}
	svc := newTestService(repo, pub)

	if err := svc.ProcessPayment(context.Background(), "p1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.payments["p1"].Status != payment.StatusCompleted {
		t.Fatalf("expected completed, got %s", repo.payments["p1"].Status)
	}
	if repo.setGatewayDecisionCalls != 0 {
		t.Fatalf("expected no new gateway decision write, got %d", repo.setGatewayDecisionCalls)
	}
	if len(pub.processed) != 1 || pub.processed[0].PaymentID != "p1" {
		t.Fatalf("expected processed event for p1, got %+v", pub.processed)
	}
}

func TestProcessPayment_ProcessingStatusReusesStoredFailureDecision(t *testing.T) {
	repo := newMockRepo()
	repo.payments["p1"] = &payment.Payment{
		PaymentID:       "p1",
		AuctionID:       "auction-1",
		UserID:          "user-1",
		ItemID:          "item-1",
		ShopID:          "shop-1",
		Amount:          500,
		Status:          payment.StatusProcessing,
		GatewayDecision: "failed",
		CreatedAt:       "created",
		UpdatedAt:       "created",
	}
	pub := &mockPublisher{}
	svc := newTestService(repo, pub)

	if err := svc.ProcessPayment(context.Background(), "p1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.payments["p1"].Status != payment.StatusFailed {
		t.Fatalf("expected failed, got %s", repo.payments["p1"].Status)
	}
	if repo.payments["p1"].FailReason == "" {
		t.Fatal("expected fail reason to be recorded")
	}
	if repo.setGatewayDecisionCalls != 0 {
		t.Fatalf("expected no new gateway decision write, got %d", repo.setGatewayDecisionCalls)
	}
	if len(pub.failed) != 1 || pub.failed[0].PaymentID != "p1" {
		t.Fatalf("expected failed event for p1, got %+v", pub.failed)
	}
}

func TestProcessPayment_InvalidStatus(t *testing.T) {
	repo := newMockRepo()
	repo.payments["p1"] = newStoredPayment("p1", "auction-1", "user-1", payment.StatusCompleted)
	pub := &mockPublisher{}
	svc := newTestService(repo, pub)

	err := svc.ProcessPayment(context.Background(), "p1")
	if !errors.Is(err, payment.ErrInvalidStatus) {
		t.Fatalf("expected ErrInvalidStatus, got %v", err)
	}
}

func TestProcessPayment_MarkProcessingError(t *testing.T) {
	repo := newMockRepo()
	repo.payments["p1"] = newStoredPayment("p1", "auction-1", "user-1", payment.StatusPending)
	repo.updateStatusErr[payment.StatusProcessing] = errors.New("boom")
	pub := &mockPublisher{}
	svc := newTestService(repo, pub)

	err := svc.ProcessPayment(context.Background(), "p1")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "mark processing") {
		t.Fatalf("expected wrapped error, got %v", err)
	}
}

func TestProcessPayment_SetGatewayDecisionError(t *testing.T) {
	repo := newMockRepo()
	repo.payments["p1"] = newStoredPayment("p1", "auction-1", "user-1", payment.StatusPending)
	repo.setGatewayErr = errors.New("boom")
	pub := &mockPublisher{}
	svc := newTestService(repo, pub)

	err := svc.ProcessPayment(context.Background(), "p1")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "record gateway decision") {
		t.Fatalf("expected wrapped error, got %v", err)
	}
}

// --- refund / abandon ---

func TestRefundPayment_Success(t *testing.T) {
	repo := newMockRepo()
	repo.payments["p1"] = newStoredPayment("p1", "auction-1", "user-1", payment.StatusCompleted)
	pub := &mockPublisher{}
	svc := newTestService(repo, pub)

	if err := svc.RefundPayment(context.Background(), "p1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.payments["p1"].Status != payment.StatusRefunded {
		t.Fatalf("expected refunded, got %s", repo.payments["p1"].Status)
	}
	if len(pub.refunded) != 1 || pub.refunded[0].PaymentID != "p1" {
		t.Fatalf("expected refunded event for p1, got %+v", pub.refunded)
	}
}

func TestRefundPayment_InvalidStatus(t *testing.T) {
	repo := newMockRepo()
	repo.payments["p1"] = newStoredPayment("p1", "auction-1", "user-1", payment.StatusPending)
	pub := &mockPublisher{}
	svc := newTestService(repo, pub)

	err := svc.RefundPayment(context.Background(), "p1")
	if !errors.Is(err, payment.ErrInvalidStatus) {
		t.Fatalf("expected ErrInvalidStatus, got %v", err)
	}
}

func TestAbandonPayment_Success(t *testing.T) {
	repo := newMockRepo()
	repo.payments["p1"] = newStoredPayment("p1", "auction-1", "user-1", payment.StatusProcessing)
	pub := &mockPublisher{}
	svc := newTestService(repo, pub)

	if err := svc.AbandonPayment(context.Background(), "p1", "too many retries"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.payments["p1"].Status != payment.StatusFailed {
		t.Fatalf("expected failed, got %s", repo.payments["p1"].Status)
	}
	if repo.payments["p1"].FailReason != "too many retries" {
		t.Fatalf("unexpected fail reason: %s", repo.payments["p1"].FailReason)
	}
	if len(pub.failed) != 1 || pub.failed[0].Reason != "too many retries" {
		t.Fatalf("expected failed event with abandon reason, got %+v", pub.failed)
	}
}

// --- payment lookup / response shaping ---

func TestGetPayment_ReturnsResponse(t *testing.T) {
	repo := newMockRepo()
	repo.payments["p1"] = newStoredPayment("p1", "auction-1", "user-1", payment.StatusCompleted)
	pub := &mockPublisher{}
	svc := newTestService(repo, pub)

	resp, err := svc.GetPayment(context.Background(), "p1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.PaymentID != "p1" || resp.Status != payment.StatusCompleted {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestGetPaymentByAuction_ReturnsResponse(t *testing.T) {
	repo := newMockRepo()
	repo.payments["p1"] = newStoredPayment("p1", "auction-1", "user-1", payment.StatusCompleted)
	pub := &mockPublisher{}
	svc := newTestService(repo, pub)

	resp, err := svc.GetPaymentByAuction(context.Background(), "auction-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.AuctionID != "auction-1" || resp.PaymentID != "p1" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestGetUserPayments_ReturnsResponses(t *testing.T) {
	repo := newMockRepo()
	repo.payments["p1"] = newStoredPayment("p1", "auction-1", "user-1", payment.StatusCompleted)
	repo.payments["p2"] = newStoredPayment("p2", "auction-2", "user-1", payment.StatusFailed)
	pub := &mockPublisher{}
	svc := newTestService(repo, pub)

	resp, err := svc.GetUserPayments(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp) != 2 {
		t.Fatalf("expected 2 payments, got %d", len(resp))
	}
}
