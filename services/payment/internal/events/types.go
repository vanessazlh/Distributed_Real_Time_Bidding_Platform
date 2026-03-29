package events

import "time"

// AuctionClosedEvent is published by the auction service when an auction ends.
type AuctionClosedEvent struct {
	AuctionID  string    `json:"auction_id"`
	WinnerID   string    `json:"winner_id"`
	WinningBid int64     `json:"winning_bid"`
	ItemID     string    `json:"item_id"`
	ShopID     string    `json:"shop_id"`
	ClosedAt   time.Time `json:"closed_at"`
}

// PaymentProcessedEvent is published when a payment completes successfully.
type PaymentProcessedEvent struct {
	PaymentID string    `json:"payment_id"`
	AuctionID string    `json:"auction_id"`
	UserID    string    `json:"user_id"`
	Amount    int64     `json:"amount"`
	ProcessedAt time.Time `json:"processed_at"`
}

// PaymentFailedEvent is published when a payment fails.
type PaymentFailedEvent struct {
	PaymentID  string    `json:"payment_id"`
	AuctionID  string    `json:"auction_id"`
	UserID     string    `json:"user_id"`
	Amount     int64     `json:"amount"`
	Reason     string    `json:"reason"`
	FailedAt   time.Time `json:"failed_at"`
}

// RefundProcessedEvent is published when a refund completes.
type RefundProcessedEvent struct {
	PaymentID   string    `json:"payment_id"`
	AuctionID   string    `json:"auction_id"`
	UserID      string    `json:"user_id"`
	Amount      int64     `json:"amount"`
	RefundedAt  time.Time `json:"refunded_at"`
}
