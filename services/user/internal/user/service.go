package user

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// ErrNotFound is returned when a user cannot be found.
var ErrNotFound = errors.New("user not found")

// ErrInvalidCredentials is returned when login credentials are wrong.
var ErrInvalidCredentials = errors.New("invalid credentials")

// ErrEmailTaken is returned when the email already exists.
var ErrEmailTaken = errors.New("email already taken")

// Repo is the interface the service depends on (enables unit-testing with mocks).
type Repo interface {
	Save(ctx context.Context, u User) error
	FindByID(ctx context.Context, userID string) (*User, error)
	FindByEmail(ctx context.Context, email string) (*User, error)
}

// Service contains business logic for the user domain.
type Service struct {
	repo Repo
}

// NewService creates a new Service.
func NewService(repo Repo) *Service {
	return &Service{repo: repo}
}

// Register creates a new user account.
func (s *Service) Register(ctx context.Context, req RegisterRequest) (string, error) {
	// Check email uniqueness
	existing, err := s.repo.FindByEmail(ctx, req.Email)
	if err == nil && existing != nil {
		return "", ErrEmailTaken
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}

	role := req.Role
	if role != "seller" {
		role = "buyer"
	}

	u := User{
		UserID:       uuid.NewString(),
		Email:        req.Email,
		PasswordHash: string(hash),
		Username:     req.Username,
		Role:         role,
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
	}
	if err := s.repo.Save(ctx, u); err != nil {
		return "", fmt.Errorf("save user: %w", err)
	}
	return u.UserID, nil
}

// Login verifies credentials and returns a signed JWT.
func (s *Service) Login(ctx context.Context, req LoginRequest) (string, error) {
	u, err := s.repo.FindByEmail(ctx, req.Email)
	if err != nil {
		return "", ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password)); err != nil {
		return "", ErrInvalidCredentials
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  u.UserID,
		"role": u.Role,
		"exp":  time.Now().Add(24 * time.Hour).Unix(),
	})
	signed, err := token.SignedString(jwtSecret())
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}
	return signed, nil
}

// GetProfile returns the user profile for the given ID.
func (s *Service) GetProfile(ctx context.Context, userID string) (*User, error) {
	u, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return nil, ErrNotFound
	}
	return u, nil
}

func jwtSecret() []byte {
	if s := os.Getenv("JWT_SECRET"); s != "" {
		return []byte(s)
	}
	return []byte("dev-secret-change-in-production")
}
