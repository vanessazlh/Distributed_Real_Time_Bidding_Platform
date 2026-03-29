package events

// BidPlacedEvent matches the Redis "bid_placed" channel payload
// published by the Auction Service.
// All services that subscribe to this channel should import this type.
type BidPlacedEvent struct {
	AuctionID       string `json:"auction_id"`
	BidID           string `json:"bid_id"`
	ItemID          string `json:"item_id"`
	ItemTitle       string `json:"item_title"`
	UserID          string `json:"user_id"`          // new highest bidder
	Amount          int64  `json:"amount"`            // cents
	PreviousHighest int64  `json:"previous_highest"`  // cents
	PreviousBidder  string `json:"previous_bidder"`   // who was outbid; "" on first bid
	BidAcceptedAt   string `json:"bid_accepted_at"`   // ISO timestamp for latency measurement
	Timestamp       string `json:"timestamp"`
}

// AuctionClosedEvent is published by the Auction Service when an auction ends.
// Consumed by Payment Service and Notification Service.
type AuctionClosedEvent struct {
	AuctionID  string `json:"auction_id"`
	WinnerID   string `json:"winner_id"`    // who to charge; "" if no bids placed
	WinningBid int64  `json:"winning_bid"`  // cents; equals start bid if no bids
	ItemID     string `json:"item_id"`
	ShopID     string `json:"shop_id"`      // seller — for payment routing
	ClosedAt   string `json:"closed_at"`
}

// PaymentProcessedEvent is published by the Payment Service on successful payment.
type PaymentProcessedEvent struct {
	PaymentID   string `json:"payment_id"`
	AuctionID   string `json:"auction_id"`
	UserID      string `json:"user_id"`
	Amount      int64  `json:"amount"`       // cents
	ProcessedAt string `json:"processed_at"`
}

// PaymentFailedEvent is published by the Payment Service on payment failure.
type PaymentFailedEvent struct {
	PaymentID string `json:"payment_id"`
	AuctionID string `json:"auction_id"`
	UserID    string `json:"user_id"`
	Amount    int64  `json:"amount"`    // cents
	Reason    string `json:"reason"`
	FailedAt  string `json:"failed_at"`
}

// RefundProcessedEvent is published by the Payment Service on successful refund.
type RefundProcessedEvent struct {
	PaymentID  string `json:"payment_id"`
	AuctionID  string `json:"auction_id"`
	UserID     string `json:"user_id"`
	Amount     int64  `json:"amount"`      // cents
	RefundedAt string `json:"refunded_at"`
}
