package auction

import "time"

// Auction represents an active or closed auction.
type Auction struct {
	AuctionID     string    `json:"auction_id"`
	ItemID        string    `json:"item_id"`
	ShopID        string    `json:"shop_id"`
	StartTime     time.Time `json:"start_time"`
	EndTime       time.Time `json:"end_time"`
	CurrentHighest int64    `json:"current_highest_bid"`
	HighestBidder string    `json:"highest_bidder"`
	Status        string    `json:"status"`  // OPEN, CLOSED
	Version       int64     `json:"version"` // for optimistic locking
}

// CreateAuctionRequest is the payload for POST /auctions.
type CreateAuctionRequest struct {
	ItemID    string `json:"item_id" binding:"required"`
	ShopID    string `json:"shop_id" binding:"required"`
	Duration  int    `json:"duration_minutes" binding:"required,min=1"` // duration in minutes
	StartBid  int64  `json:"start_bid"`
}

// PlaceBidRequest is the payload for POST /auctions/:id/bid.
type PlaceBidRequest struct {
	Amount int64 `json:"amount" binding:"required,min=1"`
}

// BidResult is the response for a successful bid.
type BidResult struct {
	BidID     string `json:"bid_id"`
	AuctionID string `json:"auction_id"`
	Amount    int64  `json:"amount"`
	Status    string `json:"status"`
}
