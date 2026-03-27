package shop

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

const (
	shopsTable = "Shops"
	itemsTable = "Items"
)

// Repository handles DynamoDB operations for shops and items.
type Repository struct {
	db *dynamodb.Client
}

// NewRepository creates a new Repository.
func NewRepository(db *dynamodb.Client) *Repository {
	return &Repository{db: db}
}

// SaveShop persists a new shop.
func (r *Repository) SaveShop(ctx context.Context, s Shop) error {
	item, err := attributevalue.MarshalMap(s)
	if err != nil {
		return fmt.Errorf("marshal shop: %w", err)
	}
	_, err = r.db.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(shopsTable),
		Item:      item,
	})
	return err
}

// FindShopByID retrieves a shop by primary key.
func (r *Repository) FindShopByID(ctx context.Context, shopID string) (*Shop, error) {
	out, err := r.db.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(shopsTable),
		Key: map[string]types.AttributeValue{
			"shop_id": &types.AttributeValueMemberS{Value: shopID},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("get shop: %w", err)
	}
	if out.Item == nil {
		return nil, errors.New("shop not found")
	}
	var s Shop
	if err := attributevalue.UnmarshalMap(out.Item, &s); err != nil {
		return nil, fmt.Errorf("unmarshal shop: %w", err)
	}
	return &s, nil
}

// SaveItem persists a new item.
func (r *Repository) SaveItem(ctx context.Context, item Item) error {
	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return fmt.Errorf("marshal item: %w", err)
	}
	_, err = r.db.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(itemsTable),
		Item:      av,
	})
	return err
}

// FindItemsByShop queries the shop_id GSI on Items.
func (r *Repository) FindItemsByShop(ctx context.Context, shopID string) ([]Item, error) {
	out, err := r.db.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(itemsTable),
		IndexName:              aws.String("shop_id-index"),
		KeyConditionExpression: aws.String("shop_id = :sid"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":sid": &types.AttributeValueMemberS{Value: shopID},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("query items by shop: %w", err)
	}

	items := make([]Item, 0, len(out.Items))
	for _, av := range out.Items {
		var it Item
		if err := attributevalue.UnmarshalMap(av, &it); err != nil {
			return nil, fmt.Errorf("unmarshal item: %w", err)
		}
		items = append(items, it)
	}
	return items, nil
}
