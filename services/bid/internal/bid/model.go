package bid

import "time"

// Bid represents a single bid placed on an auction.
type Bid struct {
	BidID     string    `json:"bid_id"`
	AuctionID string    `json:"auction_id"`
	UserID    string    `json:"user_id"`
	Amount    int64     `json:"amount"`
	Timestamp time.Time `json:"timestamp"`
	Status    string    `json:"status"` // ACCEPTED, OUTBID
}
