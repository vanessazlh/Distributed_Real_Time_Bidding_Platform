package shop

// Shop represents a seller's shop.
type Shop struct {
	ShopID   string  `dynamodbav:"shop_id" json:"shop_id"`
	Name     string  `dynamodbav:"name" json:"name"`
	Location string  `dynamodbav:"location" json:"location"`
	OwnerID  string  `dynamodbav:"owner_id" json:"owner_id"`
	LogoURL  string  `dynamodbav:"logo_url,omitempty" json:"logo_url,omitempty"`
	Lat      float64 `dynamodbav:"lat,omitempty" json:"lat,omitempty"`
	Lng      float64 `dynamodbav:"lng,omitempty" json:"lng,omitempty"`
}

// Item represents a product listed in a shop.
type Item struct {
	ItemID      string `dynamodbav:"item_id" json:"item_id"`
	ShopID      string `dynamodbav:"shop_id" json:"shop_id"`
	Title       string `dynamodbav:"title" json:"title"`
	Description string `dynamodbav:"description" json:"description"`
	RetailValue int64  `dynamodbav:"retail_value" json:"retail_value"`
	ImageURL    string `dynamodbav:"image_url,omitempty" json:"image_url,omitempty"`
	Category    string `dynamodbav:"category,omitempty" json:"category,omitempty"`
}

// CreateShopRequest is the payload for POST /shops.
type CreateShopRequest struct {
	Name     string  `json:"name" binding:"required,min=2"`
	Location string  `json:"location" binding:"required"`
	LogoURL  string  `json:"logo_url"`
	Lat      float64 `json:"lat"`
	Lng      float64 `json:"lng"`
}

// UpdateShopRequest is the payload for PUT /shops/:shop_id.
type UpdateShopRequest struct {
	Name     string  `json:"name"`
	Location string  `json:"location"`
	LogoURL  string  `json:"logo_url"`
	Lat      float64 `json:"lat"`
	Lng      float64 `json:"lng"`
}

// CreateItemRequest is the payload for POST /shops/:shop_id/items.
type CreateItemRequest struct {
	Title       string `json:"title" binding:"required,min=1"`
	Description string `json:"description"`
	RetailValue int64  `json:"retail_value"`
	ImageURL    string `json:"image_url"`
	Category    string `json:"category"`
}

// UploadResponse is returned by the upload endpoint.
type UploadResponse struct {
	URL string `json:"url"`
}

// Review represents a buyer's rating of a shop after a completed auction.
type Review struct {
	ReviewID         string `dynamodbav:"review_id" json:"review_id"`
	ShopID           string `dynamodbav:"shop_id" json:"shop_id"`
	ReviewerID       string `dynamodbav:"reviewer_id" json:"reviewer_id"`
	ReviewerUsername string `dynamodbav:"reviewer_username" json:"reviewer_username"`
	AuctionID        string `dynamodbav:"auction_id" json:"auction_id"`
	Rating           int    `dynamodbav:"rating" json:"rating"`
	Comment          string `dynamodbav:"comment,omitempty" json:"comment,omitempty"`
	SellerReply      string `dynamodbav:"seller_reply,omitempty" json:"seller_reply,omitempty"`
	CreatedAt        string `dynamodbav:"created_at" json:"created_at"`
	UpdatedAt        string `dynamodbav:"updated_at" json:"updated_at"`
}

// CreateReviewRequest is the payload for POST /shops/:shop_id/reviews.
type CreateReviewRequest struct {
	AuctionID string `json:"auction_id" binding:"required"`
	Rating    int    `json:"rating" binding:"required,min=1,max=5"`
	Comment   string `json:"comment"`
}

// ReplyRequest is the payload for POST /shops/:shop_id/reviews/:review_id/reply.
type ReplyRequest struct {
	Reply string `json:"reply" binding:"required,min=1"`
}

// ReviewsResponse is returned by GET /shops/:shop_id/reviews.
type ReviewsResponse struct {
	Reviews       []Review `json:"reviews"`
	AverageRating float64  `json:"average_rating"`
	TotalReviews  int      `json:"total_reviews"`
}
