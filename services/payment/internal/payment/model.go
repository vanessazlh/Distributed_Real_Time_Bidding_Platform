package payment

import "errors"

// Status values for a payment.
const (
	StatusPending    = "pending"
	StatusProcessing = "processing"
	StatusCompleted  = "completed"
	StatusFailed     = "failed"
	StatusRefunded   = "refunded"
)

var (
	ErrNotFound        = errors.New("payment not found")
	ErrAlreadyExists   = errors.New("payment already exists for this auction")
	ErrInvalidStatus   = errors.New("invalid payment status for this operation")
	ErrNoWinner        = errors.New("auction has no winner")
)

// Payment represents a single payment record stored in DynamoDB.
type Payment struct {
	PaymentID  string `dynamodbav:"payment_id"`  // PK
	AuctionID  string `dynamodbav:"auction_id"`  // GSI auction-index PK
	UserID     string `dynamodbav:"user_id"`     // GSI user-index PK
	ItemID     string `dynamodbav:"item_id"`
	ShopID     string `dynamodbav:"shop_id"`
	Amount     int64  `dynamodbav:"amount"`      // in cents
	Status     string `dynamodbav:"status"`
	FailReason string `dynamodbav:"fail_reason,omitempty"`
	// GatewayDecision records the mock gateway's outcome ("success" or "failed").
	// Written once before applying the result so retries of stuck PROCESSING records
	// use the same outcome instead of re-randomizing.
	//
	// NOTE: this field is a mock simplification. With a real payment gateway (e.g.
	// Stripe), the idempotency result is stored on the gateway's side and queried
	// via the idempotency key — the payment service would not need this field.
	GatewayDecision string `dynamodbav:"gateway_decision,omitempty"`
	// RetryCount tracks how many times the recovery job has retried this payment.
	// Used to enforce a retry cap: once RetryCount reaches maxRetries the payment
	// is abandoned (marked FAILED) rather than retried again.
	RetryCount int `dynamodbav:"retry_count,omitempty"`
	CreatedAt  string `dynamodbav:"created_at"`
	UpdatedAt  string `dynamodbav:"updated_at"`
}

// Response is the API representation of a Payment.
type Response struct {
	PaymentID  string `json:"payment_id"`
	AuctionID  string `json:"auction_id"`
	UserID     string `json:"user_id"`
	ItemID     string `json:"item_id"`
	ShopID     string `json:"shop_id"`
	Amount     int64  `json:"amount"`
	Status     string `json:"status"`
	FailReason string `json:"fail_reason,omitempty"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

func toResponse(p *Payment) Response {
	return Response{
		PaymentID:  p.PaymentID,
		AuctionID:  p.AuctionID,
		UserID:     p.UserID,
		ItemID:     p.ItemID,
		ShopID:     p.ShopID,
		Amount:     p.Amount,
		Status:     p.Status,
		FailReason: p.FailReason,
		CreatedAt:  p.CreatedAt,
		UpdatedAt:  p.UpdatedAt,
	}
}
