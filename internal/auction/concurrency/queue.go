package concurrency

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"

	"github.com/redis/go-redis/v9"
)

// bidRequest represents a bid to be processed by the queue.
type bidRequest struct {
	AuctionID string
	Amount    int64
	BidderID  string
	Result    chan bidResponse
}

type bidResponse struct {
	NewVersion int64
	Err        error
}

// Queue implements serialized queue-based concurrency using Go channels.
type Queue struct {
	rdb      *redis.Client
	channels sync.Map // map[string]chan bidRequest
}

// NewQueue creates a new Queue controller.
func NewQueue(rdb *redis.Client) *Queue {
	return &Queue{rdb: rdb}
}

// TryPlaceBid sends a bid request to the auction's dedicated channel.
func (q *Queue) TryPlaceBid(ctx context.Context, auctionID string, amount int64, bidderID string) (int64, error) {
	ch := q.getOrCreateChannel(auctionID)

	req := bidRequest{
		AuctionID: auctionID,
		Amount:    amount,
		BidderID:  bidderID,
		Result:    make(chan bidResponse, 1),
	}

	select {
	case ch <- req:
	case <-ctx.Done():
		return 0, ctx.Err()
	}

	select {
	case resp := <-req.Result:
		return resp.NewVersion, resp.Err
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

func (q *Queue) getOrCreateChannel(auctionID string) chan bidRequest {
	if ch, ok := q.channels.Load(auctionID); ok {
		return ch.(chan bidRequest)
	}

	ch := make(chan bidRequest, 1000)
	actual, loaded := q.channels.LoadOrStore(auctionID, ch)
	if !loaded {
		go q.processQueue(auctionID, ch)
	}
	return actual.(chan bidRequest)
}

func (q *Queue) processQueue(auctionID string, ch chan bidRequest) {
	for req := range ch {
		newVersion, err := q.processBid(req)
		req.Result <- bidResponse{NewVersion: newVersion, Err: err}
	}
}

func (q *Queue) processBid(req bidRequest) (int64, error) {
	ctx := context.Background()
	key := "auction:" + req.AuctionID

	vals, err := q.rdb.HGetAll(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("get auction: %w", err)
	}
	if len(vals) == 0 {
		return 0, errors.New("auction not found")
	}
	if vals["status"] != "OPEN" {
		return 0, errors.New("auction is not open")
	}

	currentHighest, _ := strconv.ParseInt(vals["current_highest"], 10, 64)
	if req.Amount <= currentHighest {
		return 0, fmt.Errorf("bid amount %d must be higher than current highest %d", req.Amount, currentHighest)
	}

	version, _ := strconv.ParseInt(vals["version"], 10, 64)
	newVersion := version + 1

	err = q.rdb.HSet(ctx, key, map[string]interface{}{
		"current_highest": req.Amount,
		"highest_bidder":  req.BidderID,
		"version":         newVersion,
	}).Err()
	if err != nil {
		return 0, fmt.Errorf("update auction: %w", err)
	}

	return newVersion, nil
}

// Stop closes the channel for an auction (call when auction is closed).
func (q *Queue) Stop(auctionID string) {
	if ch, ok := q.channels.LoadAndDelete(auctionID); ok {
		close(ch.(chan bidRequest))
	}
}
