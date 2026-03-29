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

func TestLogin_Success(t *testing.T) {
	svc := user.NewService(newMockRepo())
	if _, err := svc.Register(context.Background(), user.RegisterRequest{
		Email: "carol@example.com", Password: "mypassword", Username: "carol",
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	token, err := svc.Login(context.Background(), user.LoginRequest{
		Email: "carol@example.com", Password: "mypassword",
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
		Email: "dan@example.com", Password: "correct", Username: "dan",
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	_, err := svc.Login(context.Background(), user.LoginRequest{
		Email: "dan@example.com", Password: "wrong",
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

func TestGetProfile_NotFound(t *testing.T) {
	svc := user.NewService(newMockRepo())
	_, err := svc.GetProfile(context.Background(), "nonexistent-id")
	if !errors.Is(err, user.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
