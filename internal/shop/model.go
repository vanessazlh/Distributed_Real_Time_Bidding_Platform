package shop

// Shop represents a seller's shop.
type Shop struct {
	ShopID   string `dynamodbav:"shop_id" json:"shop_id"`
	Name     string `dynamodbav:"name" json:"name"`
	Location string `dynamodbav:"location" json:"location"`
	OwnerID  string `dynamodbav:"owner_id" json:"owner_id"`
	LogoURL  string `dynamodbav:"logo_url,omitempty" json:"logo_url,omitempty"`
}

// Item represents a product listed in a shop.
type Item struct {
	ItemID      string `dynamodbav:"item_id" json:"item_id"`
	ShopID      string `dynamodbav:"shop_id" json:"shop_id"`
	Title       string `dynamodbav:"title" json:"title"`
	Description string `dynamodbav:"description" json:"description"`
	RetailValue int64  `dynamodbav:"retail_value" json:"retail_value"`
	ImageURL    string `dynamodbav:"image_url,omitempty" json:"image_url,omitempty"`
}

// CreateShopRequest is the payload for POST /shops.
type CreateShopRequest struct {
	Name     string `json:"name" binding:"required,min=2"`
	Location string `json:"location" binding:"required"`
	LogoURL  string `json:"logo_url"`
}

// CreateItemRequest is the payload for POST /shops/:shop_id/items.
type CreateItemRequest struct {
	Title       string `json:"title" binding:"required,min=1"`
	Description string `json:"description"`
	RetailValue int64  `json:"retail_value"`
	ImageURL    string `json:"image_url"`
}
