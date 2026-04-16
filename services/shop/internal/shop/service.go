package shop

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/google/uuid"
)

// ErrNotFound is returned when a shop or item cannot be found.
var ErrNotFound = errors.New("not found")

// ErrForbidden is returned when the caller is not the shop owner.
var ErrForbidden = errors.New("forbidden")

// ErrInvalidInput is returned when request data fails business validation.
var ErrInvalidInput = errors.New("invalid input")

// ErrUploadNotConfigured is returned when S3 is not configured.
var ErrUploadNotConfigured = errors.New("upload not configured")

// ErrAlreadyReviewed is returned when a buyer tries to review an auction they already reviewed.
var ErrAlreadyReviewed = errors.New("you have already reviewed this auction")

// ErrPaymentNotCompleted is returned when the buyer hasn't completed payment for the auction.
var ErrPaymentNotCompleted = errors.New("payment not completed for this auction")

// ErrFileTooLarge is returned when the upload exceeds the size limit.
var ErrFileTooLarge = errors.New("file exceeds 5MB limit")

// ErrInvalidFileType is returned when the upload has a disallowed MIME type.
var ErrInvalidFileType = errors.New("only JPEG, PNG, WebP, and GIF files are allowed")

// Uploader handles file storage operations.
type Uploader interface {
	Upload(ctx context.Context, key string, contentType string, body io.Reader, size int64) error
}

// Repo is the interface the service depends on (enables unit-testing with mocks).
type Repo interface {
	SaveShop(ctx context.Context, s Shop) error
	FindShopByID(ctx context.Context, shopID string) (*Shop, error)
	FindShopsByOwnerID(ctx context.Context, ownerID string) ([]Shop, error)
	SaveItem(ctx context.Context, item Item) error
	FindItemsByShop(ctx context.Context, shopID string) ([]Item, error)
	FindItemByID(ctx context.Context, itemID string) (*Item, error)
	SaveReview(ctx context.Context, rev Review) error
	FindReviewsByShop(ctx context.Context, shopID string) ([]Review, error)
	FindReviewByAuctionAndReviewer(ctx context.Context, auctionID, reviewerID string) (*Review, error)
	FindReviewByID(ctx context.Context, reviewID string) (*Review, error)
	UpdateReviewReply(ctx context.Context, reviewID, reply, updatedAt string) error
}

// Service contains business logic for the shop domain.
type Service struct {
	repo           Repo
	uploader       Uploader
	publicURL      string
	paymentSvcURL  string
}

// NewService creates a new Service. uploader may be nil if S3 is not configured.
func NewService(repo Repo, uploader Uploader, publicURL string) *Service {
	return &Service{repo: repo, uploader: uploader, publicURL: publicURL}
}

// WithPaymentServiceURL sets the payment service base URL for eligibility checks.
func (s *Service) WithPaymentServiceURL(url string) {
	s.paymentSvcURL = url
}

// CreateShop creates a new shop owned by ownerID.
func (s *Service) CreateShop(ctx context.Context, req CreateShopRequest, ownerID string) (*Shop, error) {
	if req.Lat == 0 && req.Lng == 0 {
		return nil, fmt.Errorf("%w: shop location coordinates are required — use the Pin my location button", ErrInvalidInput)
	}
	shop := Shop{
		ShopID:   uuid.NewString(),
		Name:     req.Name,
		Location: req.Location,
		OwnerID:  ownerID,
		LogoURL:  req.LogoURL,
		Lat:      req.Lat,
		Lng:      req.Lng,
	}
	if err := s.repo.SaveShop(ctx, shop); err != nil {
		return nil, fmt.Errorf("save shop: %w", err)
	}
	return &shop, nil
}

// UpdateShop updates an existing shop. The caller must be the owner.
func (s *Service) UpdateShop(ctx context.Context, shopID string, req UpdateShopRequest, callerID string) (*Shop, error) {
	shop, err := s.repo.FindShopByID(ctx, shopID)
	if err != nil {
		return nil, ErrNotFound
	}
	if shop.OwnerID != callerID {
		return nil, ErrForbidden
	}

	if req.Name != "" {
		shop.Name = req.Name
	}
	if req.Location != "" {
		shop.Location = req.Location
	}
	// LogoURL can be set to empty to clear it, so always apply if present in request
	shop.LogoURL = req.LogoURL
	if req.Lat != 0 || req.Lng != 0 {
		shop.Lat = req.Lat
		shop.Lng = req.Lng
	}

	if err := s.repo.SaveShop(ctx, *shop); err != nil {
		return nil, fmt.Errorf("update shop: %w", err)
	}
	return shop, nil
}

// GetShop returns a shop by ID.
func (s *Service) GetShop(ctx context.Context, shopID string) (*Shop, error) {
	shop, err := s.repo.FindShopByID(ctx, shopID)
	if err != nil {
		return nil, ErrNotFound
	}
	return shop, nil
}

// GetItem returns a single item by its ID.
func (s *Service) GetItem(ctx context.Context, itemID string) (*Item, error) {
	item, err := s.repo.FindItemByID(ctx, itemID)
	if err != nil {
		return nil, ErrNotFound
	}
	return item, nil
}

// CreateItem adds an item to the shop. The caller must be the shop owner.
func (s *Service) CreateItem(ctx context.Context, shopID string, req CreateItemRequest, callerID string) (*Item, error) {
	if req.RetailValue <= 0 {
		return nil, fmt.Errorf("%w: retail_value must be greater than 0", ErrInvalidInput)
	}

	shop, err := s.repo.FindShopByID(ctx, shopID)
	if err != nil {
		return nil, ErrNotFound
	}
	if shop.OwnerID != callerID {
		return nil, ErrForbidden
	}

	item := Item{
		ItemID:      uuid.NewString(),
		ShopID:      shopID,
		Title:       req.Title,
		Description: req.Description,
		RetailValue: req.RetailValue,
		ImageURL:    req.ImageURL,
		Category:    req.Category,
	}
	if err := s.repo.SaveItem(ctx, item); err != nil {
		return nil, fmt.Errorf("save item: %w", err)
	}
	return &item, nil
}

// ListSellerShops returns all shops owned by the given seller.
func (s *Service) ListSellerShops(ctx context.Context, ownerID string) ([]Shop, error) {
	shops, err := s.repo.FindShopsByOwnerID(ctx, ownerID)
	if err != nil {
		return nil, fmt.Errorf("list seller shops: %w", err)
	}
	return shops, nil
}

// ListItems returns all items for a shop.
func (s *Service) ListItems(ctx context.Context, shopID string) ([]Item, error) {
	// Verify shop exists first
	if _, err := s.repo.FindShopByID(ctx, shopID); err != nil {
		return nil, ErrNotFound
	}
	items, err := s.repo.FindItemsByShop(ctx, shopID)
	if err != nil {
		return nil, fmt.Errorf("list items: %w", err)
	}
	return items, nil
}

var allowedMIMETypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
	"image/gif":  ".gif",
}

// UploadImage validates and stores an image file, returning its public URL.
func (s *Service) UploadImage(ctx context.Context, contentType string, body io.Reader, size int64) (*UploadResponse, error) {
	if s.uploader == nil {
		return nil, ErrUploadNotConfigured
	}

	const maxSize = 5 * 1024 * 1024
	if size > maxSize {
		return nil, ErrFileTooLarge
	}

	ext, ok := allowedMIMETypes[contentType]
	if !ok {
		return nil, ErrInvalidFileType
	}

	key := "images/" + uuid.NewString() + ext

	if err := s.uploader.Upload(ctx, key, contentType, body, size); err != nil {
		return nil, fmt.Errorf("upload file: %w", err)
	}

	return &UploadResponse{URL: s.publicURL + "/" + key}, nil
}
