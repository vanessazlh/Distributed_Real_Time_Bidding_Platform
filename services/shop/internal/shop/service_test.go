// Package shop_test exercises the exported shop service behavior through
// lightweight in-memory test doubles instead of real DynamoDB or S3 clients.
package shop_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"rtb/services/shop/internal/shop"
)

// mockRepo is an in-memory implementation of shop.Repo used by unit tests.
// It lets the service be exercised without DynamoDB or network calls.
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
	cp := s
	m.shops[s.ShopID] = &cp
	return nil
}

func (m *mockRepo) FindShopByID(_ context.Context, shopID string) (*shop.Shop, error) {
	s, ok := m.shops[shopID]
	if !ok {
		return nil, errors.New("shop not found")
	}
	cp := *s
	return &cp, nil
}

func (m *mockRepo) FindShopsByOwnerID(_ context.Context, ownerID string) ([]shop.Shop, error) {
	var result []shop.Shop
	for _, s := range m.shops {
		if s.OwnerID == ownerID {
			result = append(result, *s)
		}
	}
	return result, nil
}

func (m *mockRepo) SaveItem(_ context.Context, item shop.Item) error {
	cp := item
	m.items[item.ItemID] = &cp
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
	cp := *it
	return &cp, nil
}

func (m *mockRepo) SaveReview(_ context.Context, _ shop.Review) error { return nil }

func (m *mockRepo) FindReviewsByShop(_ context.Context, _ string) ([]shop.Review, error) {
	return []shop.Review{}, nil
}

func (m *mockRepo) FindReviewByAuctionAndReviewer(_ context.Context, _, _ string) (*shop.Review, error) {
	return nil, errors.New("not found")
}

func (m *mockRepo) FindReviewByID(_ context.Context, _ string) (*shop.Review, error) {
	return nil, errors.New("not found")
}

func (m *mockRepo) UpdateReviewReply(_ context.Context, _, _, _ string) error { return nil }

// mockUploader records upload inputs so UploadImage tests can verify storage
// behavior without talking to S3.
type mockUploader struct {
	lastKey         string
	lastContentType string
	lastSize        int64
	uploadErr       error
}

func (m *mockUploader) Upload(_ context.Context, key string, contentType string, _ io.Reader, size int64) error {
	if m.uploadErr != nil {
		return m.uploadErr
	}
	m.lastKey = key
	m.lastContentType = contentType
	m.lastSize = size
	return nil
}

// newTestService builds a shop.Service with an in-memory repo and no uploader.
// Most service tests exercise CRUD logic only, so upload support can stay nil.
func newTestService() *shop.Service {
	return shop.NewService(newMockRepo(), nil, "")
}

// --- shop creation / lookup ---

func TestCreateShop_Success(t *testing.T) {
	svc := newTestService()

	s, err := svc.CreateShop(context.Background(), shop.CreateShopRequest{
		Name:     "My Shop",
		Location: "Boston",
		Lat:      49.2827,
		Lng:      -123.1207,
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
	svc := newTestService()

	s, err := svc.CreateShop(context.Background(), shop.CreateShopRequest{
		Name:     "Logo Shop",
		Location: "Boston",
		LogoURL:  "https://example.com/logo.png",
		Lat:      49.2827,
		Lng:      -123.1207,
	}, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.LogoURL != "https://example.com/logo.png" {
		t.Fatalf("logo_url mismatch: got %s", s.LogoURL)
	}
}

func TestGetShop_NotFound(t *testing.T) {
	svc := newTestService()

	_, err := svc.GetShop(context.Background(), "no-such-shop")
	if !errors.Is(err, shop.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// --- item creation / lookup / validation ---

func TestCreateItem_Success(t *testing.T) {
	svc := newTestService()
	s, _ := svc.CreateShop(context.Background(), shop.CreateShopRequest{
		Name:     "Store",
		Location: "NYC",
		Lat:      49.2827,
		Lng:      -123.1207,
	}, "owner-1")

	item, err := svc.CreateItem(context.Background(), s.ShopID, shop.CreateItemRequest{
		Title:       "Vintage Chair",
		Description: "Very old",
		RetailValue: 5000,
		ImageURL:    "https://example.com/chair.png",
		Category:    "furniture",
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
	if item.Category != "furniture" {
		t.Fatalf("category mismatch: got %s", item.Category)
	}
}

func TestCreateItem_Forbidden(t *testing.T) {
	svc := newTestService()
	s, _ := svc.CreateShop(context.Background(), shop.CreateShopRequest{
		Name:     "Store",
		Location: "NYC",
		Lat:      49.2827,
		Lng:      -123.1207,
	}, "owner-1")

	_, err := svc.CreateItem(context.Background(), s.ShopID, shop.CreateItemRequest{
		Title:       "Chair",
		RetailValue: 100,
	}, "other-user")
	if !errors.Is(err, shop.ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func TestCreateItem_ShopNotFound(t *testing.T) {
	svc := newTestService()

	_, err := svc.CreateItem(context.Background(), "ghost-shop", shop.CreateItemRequest{
		Title:       "X",
		RetailValue: 100,
	}, "u1")
	if !errors.Is(err, shop.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestCreateItem_ZeroRetailValue(t *testing.T) {
	svc := newTestService()
	s, _ := svc.CreateShop(context.Background(), shop.CreateShopRequest{
		Name:     "Store",
		Location: "NYC",
		Lat:      49.2827,
		Lng:      -123.1207,
	}, "owner-1")

	_, err := svc.CreateItem(context.Background(), s.ShopID, shop.CreateItemRequest{
		Title:       "Chair",
		RetailValue: 0,
	}, "owner-1")
	if !errors.Is(err, shop.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestCreateItem_NegativeRetailValue(t *testing.T) {
	svc := newTestService()
	s, _ := svc.CreateShop(context.Background(), shop.CreateShopRequest{
		Name:     "Store",
		Location: "NYC",
		Lat:      49.2827,
		Lng:      -123.1207,
	}, "owner-1")

	_, err := svc.CreateItem(context.Background(), s.ShopID, shop.CreateItemRequest{
		Title:       "Chair",
		RetailValue: -50,
	}, "owner-1")
	if !errors.Is(err, shop.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestGetItem_Success(t *testing.T) {
	svc := newTestService()
	s, _ := svc.CreateShop(context.Background(), shop.CreateShopRequest{
		Name:     "Store",
		Location: "NYC",
		Lat:      49.2827,
		Lng:      -123.1207,
	}, "owner-1")
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
	svc := newTestService()

	_, err := svc.GetItem(context.Background(), "no-such-item")
	if !errors.Is(err, shop.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestListItems(t *testing.T) {
	svc := newTestService()
	s, _ := svc.CreateShop(context.Background(), shop.CreateShopRequest{
		Name:     "Store",
		Location: "LA",
		Lat:      49.2827,
		Lng:      -123.1207,
	}, "owner-1")
	_, _ = svc.CreateItem(context.Background(), s.ShopID, shop.CreateItemRequest{
		Title:       "A",
		RetailValue: 100,
	}, "owner-1")
	_, _ = svc.CreateItem(context.Background(), s.ShopID, shop.CreateItemRequest{
		Title:       "B",
		RetailValue: 200,
	}, "owner-1")

	items, err := svc.ListItems(context.Background(), s.ShopID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
}

func TestListItems_ShopNotFound(t *testing.T) {
	svc := newTestService()

	_, err := svc.ListItems(context.Background(), "missing-shop")
	if !errors.Is(err, shop.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// --- seller-owned shop listing / updates ---

func TestListSellerShops(t *testing.T) {
	svc := newTestService()
	_, _ = svc.CreateShop(context.Background(), shop.CreateShopRequest{
		Name:     "A",
		Location: "Boston",
		Lat:      49.2827,
		Lng:      -123.1207,
	}, "seller-1")
	_, _ = svc.CreateShop(context.Background(), shop.CreateShopRequest{
		Name:     "B",
		Location: "NYC",
		Lat:      49.2827,
		Lng:      -123.1207,
	}, "seller-1")
	_, _ = svc.CreateShop(context.Background(), shop.CreateShopRequest{
		Name:     "C",
		Location: "LA",
		Lat:      49.2827,
		Lng:      -123.1207,
	}, "seller-2")

	shops, err := svc.ListSellerShops(context.Background(), "seller-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(shops) != 2 {
		t.Fatalf("expected 2 shops, got %d", len(shops))
	}
}

func TestUpdateShop_Success(t *testing.T) {
	svc := newTestService()
	created, _ := svc.CreateShop(context.Background(), shop.CreateShopRequest{
		Name:     "Old Name",
		Location: "Old Location",
		Lat:      49.2827,
		Lng:      -123.1207,
	}, "owner-1")

	updated, err := svc.UpdateShop(context.Background(), created.ShopID, shop.UpdateShopRequest{
		Name:     "New Name",
		Location: "New Location",
		LogoURL:  "https://example.com/logo.png",
	}, "owner-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Name != "New Name" {
		t.Fatalf("name mismatch: got %s", updated.Name)
	}
	if updated.Location != "New Location" {
		t.Fatalf("location mismatch: got %s", updated.Location)
	}
	if updated.LogoURL != "https://example.com/logo.png" {
		t.Fatalf("logo_url mismatch: got %s", updated.LogoURL)
	}
}

func TestUpdateShop_Forbidden(t *testing.T) {
	svc := newTestService()
	created, _ := svc.CreateShop(context.Background(), shop.CreateShopRequest{
		Name:     "Store",
		Location: "Boston",
		Lat:      49.2827,
		Lng:      -123.1207,
	}, "owner-1")

	_, err := svc.UpdateShop(context.Background(), created.ShopID, shop.UpdateShopRequest{
		Name: "Hack",
	}, "other-user")
	if !errors.Is(err, shop.ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func TestUpdateShop_NotFound(t *testing.T) {
	svc := newTestService()

	_, err := svc.UpdateShop(context.Background(), "missing-shop", shop.UpdateShopRequest{
		Name: "X",
	}, "owner-1")
	if !errors.Is(err, shop.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// --- upload validation / storage ---

func TestUploadImage_NotConfigured(t *testing.T) {
	svc := shop.NewService(newMockRepo(), nil, "")

	_, err := svc.UploadImage(context.Background(), "image/png", bytes.NewReader([]byte("abc")), 3)
	if !errors.Is(err, shop.ErrUploadNotConfigured) {
		t.Fatalf("expected ErrUploadNotConfigured, got %v", err)
	}
}

func TestUploadImage_InvalidFileType(t *testing.T) {
	uploader := &mockUploader{}
	svc := shop.NewService(newMockRepo(), uploader, "http://localhost:3000/uploads")

	_, err := svc.UploadImage(context.Background(), "text/plain", bytes.NewReader([]byte("abc")), 3)
	if !errors.Is(err, shop.ErrInvalidFileType) {
		t.Fatalf("expected ErrInvalidFileType, got %v", err)
	}
}

func TestUploadImage_FileTooLarge(t *testing.T) {
	uploader := &mockUploader{}
	svc := shop.NewService(newMockRepo(), uploader, "http://localhost:3000/uploads")

	_, err := svc.UploadImage(context.Background(), "image/png", bytes.NewReader([]byte("abc")), 6*1024*1024)
	if !errors.Is(err, shop.ErrFileTooLarge) {
		t.Fatalf("expected ErrFileTooLarge, got %v", err)
	}
}

func TestUploadImage_Success(t *testing.T) {
	uploader := &mockUploader{}
	svc := shop.NewService(newMockRepo(), uploader, "http://localhost:3000/uploads")

	resp, err := svc.UploadImage(context.Background(), "image/png", bytes.NewReader([]byte("pngdata")), 7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil || resp.URL == "" {
		t.Fatal("expected non-empty upload URL")
	}
	if uploader.lastContentType != "image/png" {
		t.Fatalf("content type mismatch: got %s", uploader.lastContentType)
	}
	if uploader.lastSize != 7 {
		t.Fatalf("size mismatch: got %d", uploader.lastSize)
	}
	if uploader.lastKey == "" {
		t.Fatal("expected uploader to receive a generated key")
	}
}
