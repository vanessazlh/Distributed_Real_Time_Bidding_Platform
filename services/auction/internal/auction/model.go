package auction

import "time"

// Auction represents an active or closed auction.
type Auction struct {
	AuctionID      string    `json:"auction_id"`
	ItemID         string    `json:"item_id"`
	ItemTitle      string    `json:"item_title"`
	SellerID       string    `json:"seller_id"`
	ShopID         string    `json:"shop_id"`
	ShopName       string    `json:"shop_name"`
	RetailPrice    int64     `json:"retail_price"`
	MaxPrice       int64     `json:"max_price,omitempty"` // bid ceiling; 0 = no limit
	Quantity       int       `json:"quantity"`            // number of winners; 1 = standard auction
	ImageURL       string    `json:"image_url"`
	ShopLogoURL    string    `json:"shop_logo_url"`
	Description    string    `json:"description"`
	Category       string    `json:"category,omitempty"`
	PickupStart    time.Time `json:"pickup_start"`
	PickupEnd      time.Time `json:"pickup_end"`
	StartTime      time.Time `json:"start_time"`
	EndTime        time.Time `json:"end_time"`
	CurrentHighest int64     `json:"current_highest_bid"`
	BidCount       int64     `json:"bid_count"`
	HighestBidder  string           `json:"highest_bidder"`
	Status         string           `json:"status"`  // PENDING, OPEN, CLOSED
	Version        int64            `json:"version"` // for optimistic locking
	Winners        map[string]int64 `json:"-"`        // DynamoDB-only; bidderID → amount
}

// CreateAuctionRequest is the payload for POST /auctions.
type CreateAuctionRequest struct {
	ItemID      string `json:"item_id" binding:"required"`
	ItemTitle   string `json:"item_title"`
	ShopID      string `json:"shop_id" binding:"required"`
	ShopName    string `json:"shop_name"`
	RetailPrice int64  `json:"retail_price"`
	MaxPrice    int64  `json:"max_price"`      // bid ceiling; 0 = no limit
	Quantity    int    `json:"quantity"`       // number of winners; default 1
	ImageURL    string `json:"image_url"`
	ShopLogoURL string `json:"shop_logo_url"`
	Description string `json:"description"`
	Category       string `json:"category"`
	Duration       int    `json:"duration_minutes" binding:"required,min=1"`
	StartBid       int64  `json:"start_bid"`
	ScheduledStart string `json:"scheduled_start"` // optional RFC3339; if set, auction starts as PENDING
	PickupStart    string `json:"pickup_start"`    // optional RFC3339; pickup window start
	PickupEnd      string `json:"pickup_end"`      // optional RFC3339; pickup window end
}

// PlaceBidRequest is the payload for POST /auctions/:id/bid.
type PlaceBidRequest struct {
	Amount int64 `json:"amount" binding:"required,min=1"`
}

// BidResult is the response for a successful bid.
type BidResult struct {
	BidID          string `json:"bid_id"`
	AuctionID      string `json:"auction_id"`
	Amount         int64  `json:"amount"`
	NewHighestBid  int64  `json:"new_highest_bid"`
	Status         string `json:"status"`
}
