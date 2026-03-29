package shop

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// ErrNotFound is returned when a shop or item cannot be found.
var ErrNotFound = errors.New("not found")

// ErrForbidden is returned when the caller is not the shop owner.
var ErrForbidden = errors.New("forbidden")

// ErrInvalidInput is returned when request data fails business validation.
var ErrInvalidInput = errors.New("invalid input")

// Repo is the interface the service depends on (enables unit-testing with mocks).
type Repo interface {
	SaveShop(ctx context.Context, s Shop) error
	FindShopByID(ctx context.Context, shopID string) (*Shop, error)
	SaveItem(ctx context.Context, item Item) error
	FindItemsByShop(ctx context.Context, shopID string) ([]Item, error)
	FindItemByID(ctx context.Context, itemID string) (*Item, error)
}

// Service contains business logic for the shop domain.
type Service struct {
	repo Repo
}

// NewService creates a new Service.
func NewService(repo Repo) *Service {
	return &Service{repo: repo}
}

// CreateShop creates a new shop owned by ownerID.
func (s *Service) CreateShop(ctx context.Context, req CreateShopRequest, ownerID string) (*Shop, error) {
	shop := Shop{
		ShopID:   uuid.NewString(),
		Name:     req.Name,
		Location: req.Location,
		OwnerID:  ownerID,
		LogoURL:  req.LogoURL,
	}
	if err := s.repo.SaveShop(ctx, shop); err != nil {
		return nil, fmt.Errorf("save shop: %w", err)
	}
	return &shop, nil
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
	}
	if err := s.repo.SaveItem(ctx, item); err != nil {
		return nil, fmt.Errorf("save item: %w", err)
	}
	return &item, nil
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
