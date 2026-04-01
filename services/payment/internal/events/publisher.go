package events

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
)

const (
	StreamPaymentProcessed = "payment:processed"
	StreamPaymentFailed    = "payment:failed"
	StreamRefundProcessed  = "refund:processed"
)

// Publisher publishes domain events to Redis Streams.
type Publisher struct {
	rdb *redis.Client
}

// NewPublisher creates a new Publisher.
func NewPublisher(rdb *redis.Client) *Publisher {
	return &Publisher{rdb: rdb}
}

// PublishPaymentProcessed publishes a payment-processed event.
func (p *Publisher) PublishPaymentProcessed(ctx context.Context, event PaymentProcessedEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal payment processed event: %w", err)
	}
	return p.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: StreamPaymentProcessed,
		Values: map[string]interface{}{"payload": string(data)},
	}).Err()
}

// PublishPaymentFailed publishes a payment-failed event.
func (p *Publisher) PublishPaymentFailed(ctx context.Context, event PaymentFailedEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal payment failed event: %w", err)
	}
	return p.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: StreamPaymentFailed,
		Values: map[string]interface{}{"payload": string(data)},
	}).Err()
}

// PublishRefundProcessed publishes a refund-processed event.
func (p *Publisher) PublishRefundProcessed(ctx context.Context, event RefundProcessedEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal refund processed event: %w", err)
	}
	return p.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: StreamRefundProcessed,
		Values: map[string]interface{}{"payload": string(data)},
	}).Err()
}
