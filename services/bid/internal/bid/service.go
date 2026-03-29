package bid

import (
	"context"
	"errors"
	"fmt"
)

// ErrNotFound is returned when a bid cannot be found.
var ErrNotFound = errors.New("bid not found")

// Repo is the interface the service depends on.
type Repo interface {
	Create(ctx context.Context, b *Bid) error
	GetByAuction(ctx context.Context, auctionID string) ([]*Bid, error)
	GetByUser(ctx context.Context, userID string) ([]*Bid, error)
	MarkOutbid(ctx context.Context, auctionID string, excludeBidID string) error
}

// Service contains business logic for the bid domain.
type Service struct {
	repo Repo
}

// NewService creates a new Service.
func NewService(repo Repo) *Service {
	return &Service{repo: repo}
}

// RecordBid records a new bid and marks previous bids as outbid.
func (s *Service) RecordBid(ctx context.Context, b *Bid) error {
	if err := s.repo.Create(ctx, b); err != nil {
		return fmt.Errorf("record bid: %w", err)
	}
	if err := s.repo.MarkOutbid(ctx, b.AuctionID, b.BidID); err != nil {
		return fmt.Errorf("mark outbid: %w", err)
	}
	return nil
}

// GetAuctionBids returns all bids for an auction.
func (s *Service) GetAuctionBids(ctx context.Context, auctionID string) ([]*Bid, error) {
	bids, err := s.repo.GetByAuction(ctx, auctionID)
	if err != nil {
		return nil, fmt.Errorf("get auction bids: %w", err)
	}
	return bids, nil
}

// GetUserBids returns all bids placed by a user.
func (s *Service) GetUserBids(ctx context.Context, userID string) ([]*Bid, error) {
	bids, err := s.repo.GetByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user bids: %w", err)
	}
	return bids, nil
}
