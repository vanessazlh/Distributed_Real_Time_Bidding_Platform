package shop

// Shop represents a seller's shop.
type Shop struct {
	ShopID   string `dynamodbav:"shop_id" json:"shop_id"`
	Name     string `dynamodbav:"name" json:"name"`
	Location string `dynamodbav:"location" json:"location"`
	OwnerID  string `dynamodbav:"owner_id" json:"owner_id"`
}

// Item represents a product listed in a shop.
type Item struct {
	ItemID      string `dynamodbav:"item_id" json:"item_id"`
	ShopID      string `dynamodbav:"shop_id" json:"shop_id"`
	Title       string `dynamodbav:"title" json:"title"`
	Description string `dynamodbav:"description" json:"description"`
}

// CreateShopRequest is the payload for POST /shops.
type CreateShopRequest struct {
	Name     string `json:"name" binding:"required,min=2"`
	Location string `json:"location" binding:"required"`
}

// CreateItemRequest is the payload for POST /shops/:shop_id/items.
type CreateItemRequest struct {
	Title       string `json:"title" binding:"required,min=1"`
	Description string `json:"description"`
}
