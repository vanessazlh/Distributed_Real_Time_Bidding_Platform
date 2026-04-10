package user_test

import (
	"context"
	"errors"
	"testing"

	"rtb/services/user/internal/user"
)

// --- mock repo ---

type mockRepo struct {
	users map[string]*user.User // keyed by user_id
}

func newMockRepo() *mockRepo { return &mockRepo{users: make(map[string]*user.User)} }

func (m *mockRepo) Save(_ context.Context, u user.User) error {
	if _, exists := m.users[u.UserID]; exists {
		return errors.New("duplicate user_id")
	}
	m.users[u.UserID] = &u
	return nil
}

func (m *mockRepo) FindByID(_ context.Context, userID string) (*user.User, error) {
	u, ok := m.users[userID]
	if !ok {
		return nil, errors.New("user not found")
	}
	return u, nil
}

func (m *mockRepo) FindByEmail(_ context.Context, email string) (*user.User, error) {
	for _, u := range m.users {
		if u.Email == email {
			return u, nil
		}
	}
	return nil, errors.New("user not found")
}

func (m *mockRepo) UpdateProfile(_ context.Context, userID, username, avatarURL string) error {
	u, ok := m.users[userID]
	if !ok {
		return errors.New("user not found")
	}
	u.Username = username
	if avatarURL != "" {
		u.AvatarURL = avatarURL
	}
	return nil
}

func (m *mockRepo) UpdateRole(_ context.Context, userID, role string) error {
	u, ok := m.users[userID]
	if !ok {
		return errors.New("user not found")
	}
	u.Role = role
	return nil
}

// --- tests ---

func TestRegister_Success(t *testing.T) {
	svc := user.NewService(newMockRepo())
	id, err := svc.Register(context.Background(), user.RegisterRequest{
		Email:    "alice@example.com",
		Password: "secret123",
		Username: "alice",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty user_id")
	}
}

func TestRegister_DuplicateEmail(t *testing.T) {
	svc := user.NewService(newMockRepo())
	req := user.RegisterRequest{Email: "bob@example.com", Password: "pass123", Username: "bob"}

	if _, err := svc.Register(context.Background(), req); err != nil {
		t.Fatalf("first register failed: %v", err)
	}

	_, err := svc.Register(context.Background(), req)
	if !errors.Is(err, user.ErrEmailTaken) {
		t.Fatalf("expected ErrEmailTaken, got %v", err)
	}
}

func TestRegister_BuyerToSellerUpgrade(t *testing.T) {
	svc := user.NewService(newMockRepo())

	// Register as buyer first
	buyerID, err := svc.Register(context.Background(), user.RegisterRequest{
		Email: "carol@example.com", Password: "mypassword", Username: "carol",
	})
	if err != nil {
		t.Fatalf("buyer register: %v", err)
	}

	// Upgrade to seller with same password
	sellerID, err := svc.Register(context.Background(), user.RegisterRequest{
		Email: "carol@example.com", Password: "mypassword", Username: "carol", Role: "seller",
	})
	if err != nil {
		t.Fatalf("seller upgrade: %v", err)
	}
	if sellerID != buyerID {
		t.Fatalf("expected same user_id after upgrade, got buyer=%s seller=%s", buyerID, sellerID)
	}

	// Login should return seller role
	token, err := svc.Login(context.Background(), user.LoginRequest{
		Email: "carol@example.com", Password: "mypassword",
	})
	if err != nil {
		t.Fatalf("login after upgrade: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
}

func TestRegister_BuyerToSellerWrongPassword(t *testing.T) {
	svc := user.NewService(newMockRepo())

	if _, err := svc.Register(context.Background(), user.RegisterRequest{
		Email: "dan@example.com", Password: "correct", Username: "dan",
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	_, err := svc.Register(context.Background(), user.RegisterRequest{
		Email: "dan@example.com", Password: "wrong", Username: "dan", Role: "seller",
	})
	if !errors.Is(err, user.ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestRegister_AlreadySeller(t *testing.T) {
	svc := user.NewService(newMockRepo())

	if _, err := svc.Register(context.Background(), user.RegisterRequest{
		Email: "eve@example.com", Password: "pass123", Username: "eve", Role: "seller",
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	_, err := svc.Register(context.Background(), user.RegisterRequest{
		Email: "eve@example.com", Password: "pass123", Username: "eve", Role: "seller",
	})
	if !errors.Is(err, user.ErrAlreadySeller) {
		t.Fatalf("expected ErrAlreadySeller, got %v", err)
	}
}

func TestLogin_Success(t *testing.T) {
	svc := user.NewService(newMockRepo())
	if _, err := svc.Register(context.Background(), user.RegisterRequest{
		Email: "frank@example.com", Password: "mypassword", Username: "frank",
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	token, err := svc.Login(context.Background(), user.LoginRequest{
		Email: "frank@example.com", Password: "mypassword",
	})
	if err != nil {
		t.Fatalf("login error: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	svc := user.NewService(newMockRepo())
	if _, err := svc.Register(context.Background(), user.RegisterRequest{
		Email: "grace@example.com", Password: "correct", Username: "grace",
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	_, err := svc.Login(context.Background(), user.LoginRequest{
		Email: "grace@example.com", Password: "wrong",
	})
	if !errors.Is(err, user.ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestLogin_UnknownEmail(t *testing.T) {
	svc := user.NewService(newMockRepo())
	_, err := svc.Login(context.Background(), user.LoginRequest{
		Email: "nobody@example.com", Password: "pass",
	})
	if !errors.Is(err, user.ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestRegister_BuyerToSellerUsernameMismatch(t *testing.T) {
	svc := user.NewService(newMockRepo())

	if _, err := svc.Register(context.Background(), user.RegisterRequest{
		Email: "fay@example.com", Password: "pass123", Username: "fay_buyer",
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	// Attempt upgrade with different username — should get mismatch error
	_, err := svc.Register(context.Background(), user.RegisterRequest{
		Email: "fay@example.com", Password: "pass123", Username: "fay_seller", Role: "seller",
	})
	var mismatch *user.UsernameMismatchError
	if !errors.As(err, &mismatch) {
		t.Fatalf("expected UsernameMismatchError, got %v", err)
	}
	if mismatch.ExistingUsername != "fay_buyer" {
		t.Fatalf("expected existing username fay_buyer, got %s", mismatch.ExistingUsername)
	}
}

func TestRegister_BuyerToSellerConfirmNewUsername(t *testing.T) {
	repo := newMockRepo()
	svc := user.NewService(repo)

	buyerID, err := svc.Register(context.Background(), user.RegisterRequest{
		Email: "gina@example.com", Password: "pass123", Username: "gina_buyer",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	// Confirm upgrade with new username
	sellerID, err := svc.Register(context.Background(), user.RegisterRequest{
		Email: "gina@example.com", Password: "pass123", Username: "gina_seller",
		Role: "seller", ConfirmUpgrade: true,
	})
	if err != nil {
		t.Fatalf("confirmed upgrade: %v", err)
	}
	if sellerID != buyerID {
		t.Fatalf("expected same user_id, got buyer=%s seller=%s", buyerID, sellerID)
	}

	// Verify username was updated
	u, err := svc.GetProfile(context.Background(), sellerID)
	if err != nil {
		t.Fatalf("get profile: %v", err)
	}
	if u.Username != "gina_seller" {
		t.Fatalf("expected username gina_seller, got %s", u.Username)
	}
	if u.Role != "seller" {
		t.Fatalf("expected role seller, got %s", u.Role)
	}
}

func TestGetProfile_NotFound(t *testing.T) {
	svc := user.NewService(newMockRepo())
	_, err := svc.GetProfile(context.Background(), "nonexistent-id")
	if !errors.Is(err, user.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
