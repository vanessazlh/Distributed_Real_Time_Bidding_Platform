package events

// BidPlacedEvent is published by the Auction Service to the "bid:placed" Redis Stream.
// Consumed by Bid Service and Notification Service via consumer groups.
type BidPlacedEvent struct {
	AuctionID       string `json:"auction_id"`
	BidID           string `json:"bid_id"`
	ItemID          string `json:"item_id"`
	ItemTitle       string `json:"item_title"`
	ShopName        string `json:"shop_name"`
	UserID          string `json:"user_id"`          // new highest bidder
	Amount          int64  `json:"amount"`            // cents
	PreviousHighest int64  `json:"previous_highest"`  // cents
	PreviousBidder  string `json:"previous_bidder"`   // who was outbid; "" on first bid
	BidAcceptedAt   string `json:"bid_accepted_at"`   // ISO timestamp for latency measurement
	Timestamp       string `json:"timestamp"`
}

// AuctionClosedEvent is published by the Auction Service to the "auction:closed" Redis Stream.
// Consumed by Payment Service, Bid Service, and Notification Service via consumer groups.
//
// For single-winner auctions (Quantity <= 1): WinnerID and WinningBid are set.
// For multi-winner auctions: Winners contains all winning bidder→amount pairs,
// and WinnerID/WinningBid point to the top bidder (backwards compatible).
type AuctionClosedEvent struct {
	AuctionID  string           `json:"auction_id"`
	WinnerID   string           `json:"winner_id"`    // top winner; "" if no bids placed
	WinningBid int64            `json:"winning_bid"`  // top winner's bid in cents
	Winners    map[string]int64 `json:"winners,omitempty"` // all winners: bidderID → amount (quantity>1)
	Quantity   int              `json:"quantity"`     // number of winning slots
	ItemID     string           `json:"item_id"`
	ItemTitle  string           `json:"item_title"`
	ShopID     string           `json:"shop_id"`      // seller — for payment routing
	ClosedAt   string           `json:"closed_at"`
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
