package shop_test

import (
	"context"
	"errors"
	"testing"

	"github.com/surplus-auction/platform/internal/shop"
)

// --- mock repo ---

type mockRepo struct {
	shops map[string]*shop.Shop
	items map[string]*shop.Item
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		shops: make(map[string]*shop.Shop),
		items: make(map[string]*shop.Item),
	}
}

func (m *mockRepo) SaveShop(_ context.Context, s shop.Shop) error {
	m.shops[s.ShopID] = &s
	return nil
}

func (m *mockRepo) FindShopByID(_ context.Context, shopID string) (*shop.Shop, error) {
	s, ok := m.shops[shopID]
	if !ok {
		return nil, errors.New("shop not found")
	}
	return s, nil
}

func (m *mockRepo) SaveItem(_ context.Context, item shop.Item) error {
	m.items[item.ItemID] = &item
	return nil
}

func (m *mockRepo) FindItemsByShop(_ context.Context, shopID string) ([]shop.Item, error) {
	var result []shop.Item
	for _, it := range m.items {
		if it.ShopID == shopID {
			result = append(result, *it)
		}
	}
	return result, nil
}

func (m *mockRepo) FindItemByID(_ context.Context, itemID string) (*shop.Item, error) {
	it, ok := m.items[itemID]
	if !ok {
		return nil, errors.New("item not found")
	}
	return it, nil
}

// --- tests ---

func TestCreateShop_Success(t *testing.T) {
	svc := shop.NewService(newMockRepo())
	s, err := svc.CreateShop(context.Background(), shop.CreateShopRequest{
		Name:     "My Shop",
		Location: "Boston",
	}, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.ShopID == "" {
		t.Fatal("expected non-empty shop_id")
	}
	if s.OwnerID != "user-1" {
		t.Fatalf("owner mismatch: got %s", s.OwnerID)
	}
}

func TestCreateShop_WithLogoURL(t *testing.T) {
	svc := shop.NewService(newMockRepo())
	s, err := svc.CreateShop(context.Background(), shop.CreateShopRequest{
		Name:     "Logo Shop",
		Location: "Boston",
		LogoURL:  "https://example.com/logo.png",
	}, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.LogoURL != "https://example.com/logo.png" {
		t.Fatalf("logo_url mismatch: got %s", s.LogoURL)
	}
}

func TestGetShop_NotFound(t *testing.T) {
	svc := shop.NewService(newMockRepo())
	_, err := svc.GetShop(context.Background(), "no-such-shop")
	if !errors.Is(err, shop.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestCreateItem_Success(t *testing.T) {
	svc := shop.NewService(newMockRepo())
	s, _ := svc.CreateShop(context.Background(), shop.CreateShopRequest{Name: "Store", Location: "NYC"}, "owner-1")

	item, err := svc.CreateItem(context.Background(), s.ShopID, shop.CreateItemRequest{
		Title:       "Vintage Chair",
		Description: "Very old",
		RetailValue: 5000,
		ImageURL:    "https://example.com/chair.png",
	}, "owner-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item.ItemID == "" {
		t.Fatal("expected non-empty item_id")
	}
	if item.RetailValue != 5000 {
		t.Fatalf("retail_value mismatch: got %d", item.RetailValue)
	}
	if item.ImageURL != "https://example.com/chair.png" {
		t.Fatalf("image_url mismatch: got %s", item.ImageURL)
	}
}

func TestCreateItem_Forbidden(t *testing.T) {
	svc := shop.NewService(newMockRepo())
	s, _ := svc.CreateShop(context.Background(), shop.CreateShopRequest{Name: "Store", Location: "NYC"}, "owner-1")

	_, err := svc.CreateItem(context.Background(), s.ShopID, shop.CreateItemRequest{Title: "Chair", RetailValue: 100}, "other-user")
	if !errors.Is(err, shop.ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func TestCreateItem_ShopNotFound(t *testing.T) {
	svc := shop.NewService(newMockRepo())
	_, err := svc.CreateItem(context.Background(), "ghost-shop", shop.CreateItemRequest{Title: "X", RetailValue: 100}, "u1")
	if !errors.Is(err, shop.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestCreateItem_ZeroRetailValue(t *testing.T) {
	svc := shop.NewService(newMockRepo())
	s, _ := svc.CreateShop(context.Background(), shop.CreateShopRequest{Name: "Store", Location: "NYC"}, "owner-1")

	_, err := svc.CreateItem(context.Background(), s.ShopID, shop.CreateItemRequest{Title: "Chair", RetailValue: 0}, "owner-1")
	if !errors.Is(err, shop.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestCreateItem_NegativeRetailValue(t *testing.T) {
	svc := shop.NewService(newMockRepo())
	s, _ := svc.CreateShop(context.Background(), shop.CreateShopRequest{Name: "Store", Location: "NYC"}, "owner-1")

	_, err := svc.CreateItem(context.Background(), s.ShopID, shop.CreateItemRequest{Title: "Chair", RetailValue: -50}, "owner-1")
	if !errors.Is(err, shop.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestGetItem_Success(t *testing.T) {
	svc := shop.NewService(newMockRepo())
	s, _ := svc.CreateShop(context.Background(), shop.CreateShopRequest{Name: "Store", Location: "NYC"}, "owner-1")
	created, _ := svc.CreateItem(context.Background(), s.ShopID, shop.CreateItemRequest{
		Title:       "Table",
		RetailValue: 9999,
	}, "owner-1")

	item, err := svc.GetItem(context.Background(), created.ItemID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item.ItemID != created.ItemID {
		t.Fatalf("item_id mismatch: got %s", item.ItemID)
	}
	if item.RetailValue != 9999 {
		t.Fatalf("retail_value mismatch: got %d", item.RetailValue)
	}
}

func TestGetItem_NotFound(t *testing.T) {
	svc := shop.NewService(newMockRepo())
	_, err := svc.GetItem(context.Background(), "no-such-item")
	if !errors.Is(err, shop.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestListItems(t *testing.T) {
	svc := shop.NewService(newMockRepo())
	s, _ := svc.CreateShop(context.Background(), shop.CreateShopRequest{Name: "Store", Location: "LA"}, "owner-1")
	svc.CreateItem(context.Background(), s.ShopID, shop.CreateItemRequest{Title: "A", RetailValue: 100}, "owner-1")
	svc.CreateItem(context.Background(), s.ShopID, shop.CreateItemRequest{Title: "B", RetailValue: 200}, "owner-1")

	items, err := svc.ListItems(context.Background(), s.ShopID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
}
