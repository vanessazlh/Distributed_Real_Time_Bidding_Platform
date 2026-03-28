package events

import "time"

// BidPlacedEvent is published when a new highest bid is placed.
type BidPlacedEvent struct {
	AuctionID       string    `json:"auction_id"`
	BidID           string    `json:"bid_id"`
	UserID          string    `json:"user_id"`
	Amount          int64     `json:"amount"`
	PreviousHighest int64     `json:"previous_highest"`
	PreviousBidder  string    `json:"previous_bidder"`
	Timestamp       time.Time `json:"timestamp"`
}

// AuctionClosedEvent is published when an auction ends.
type AuctionClosedEvent struct {
	AuctionID  string    `json:"auction_id"`
	WinnerID   string    `json:"winner_id"`
	WinningBid int64     `json:"winning_bid"`
	ItemID     string    `json:"item_id"`
	ShopID     string    `json:"shop_id"`
	ClosedAt   time.Time `json:"closed_at"`
}
