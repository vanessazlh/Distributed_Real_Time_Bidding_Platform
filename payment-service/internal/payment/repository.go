package payment

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

const tableName = "payments"

// Repository handles DynamoDB operations for payments.
type Repository struct {
	db *dynamodb.Client
}

// NewRepository creates a new Repository.
func NewRepository(db *dynamodb.Client) *Repository {
	return &Repository{db: db}
}

// Create stores a new payment record.
func (r *Repository) Create(ctx context.Context, p *Payment) error {
	item, err := attributevalue.MarshalMap(p)
	if err != nil {
		return fmt.Errorf("marshal payment: %w", err)
	}

	_, err = r.db.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String(tableName),
		Item:                item,
		ConditionExpression: aws.String("attribute_not_exists(payment_id)"),
	})
	if err != nil {
		return fmt.Errorf("create payment: %w", err)
	}
	return nil
}

// GetByID retrieves a payment by its ID.
func (r *Repository) GetByID(ctx context.Context, paymentID string) (*Payment, error) {
	out, err := r.db.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"payment_id": &types.AttributeValueMemberS{Value: paymentID},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("get payment: %w", err)
	}
	if out.Item == nil {
		return nil, ErrNotFound
	}

	var p Payment
	if err := attributevalue.UnmarshalMap(out.Item, &p); err != nil {
		return nil, fmt.Errorf("unmarshal payment: %w", err)
	}
	return &p, nil
}

// GetByAuctionID retrieves the payment for a given auction (via GSI).
func (r *Repository) GetByAuctionID(ctx context.Context, auctionID string) (*Payment, error) {
	out, err := r.db.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		IndexName:              aws.String("auction-index"),
		KeyConditionExpression: aws.String("auction_id = :aid"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":aid": &types.AttributeValueMemberS{Value: auctionID},
		},
		Limit: aws.Int32(1),
	})
	if err != nil {
		return nil, fmt.Errorf("query payment by auction: %w", err)
	}
	if len(out.Items) == 0 {
		return nil, ErrNotFound
	}

	var p Payment
	if err := attributevalue.UnmarshalMap(out.Items[0], &p); err != nil {
		return nil, fmt.Errorf("unmarshal payment: %w", err)
	}
	return &p, nil
}

// GetByUserID retrieves all payments for a given user (via GSI).
func (r *Repository) GetByUserID(ctx context.Context, userID string) ([]*Payment, error) {
	out, err := r.db.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		IndexName:              aws.String("user-index"),
		KeyConditionExpression: aws.String("user_id = :uid"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":uid": &types.AttributeValueMemberS{Value: userID},
		},
		ScanIndexForward: aws.Bool(false), // newest first
	})
	if err != nil {
		return nil, fmt.Errorf("query payments by user: %w", err)
	}

	payments := make([]*Payment, 0, len(out.Items))
	for _, item := range out.Items {
		var p Payment
		if err := attributevalue.UnmarshalMap(item, &p); err != nil {
			continue
		}
		payments = append(payments, &p)
	}
	return payments, nil
}

// UpdateStatus updates the status (and optional fail_reason) of a payment.
func (r *Repository) UpdateStatus(ctx context.Context, paymentID, status, failReason string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := r.db.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"payment_id": &types.AttributeValueMemberS{Value: paymentID},
		},
		UpdateExpression: aws.String("SET #s = :s, updated_at = :ua, fail_reason = :fr"),
		ExpressionAttributeNames: map[string]string{
			"#s": "status",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":s":  &types.AttributeValueMemberS{Value: status},
			":ua": &types.AttributeValueMemberS{Value: now},
			":fr": &types.AttributeValueMemberS{Value: failReason},
		},
	})
	if err != nil {
		return fmt.Errorf("update payment status: %w", err)
	}
	return nil
}
