package user

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

const tableName = "Users"

// Repository handles DynamoDB operations for users.
type Repository struct {
	db *dynamodb.Client
}

// NewRepository creates a new Repository.
func NewRepository(db *dynamodb.Client) *Repository {
	return &Repository{db: db}
}

// Save persists a new user to DynamoDB.
func (r *Repository) Save(ctx context.Context, u User) error {
	item, err := attributevalue.MarshalMap(u)
	if err != nil {
		return fmt.Errorf("marshal user: %w", err)
	}
	_, err = r.db.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String(tableName),
		Item:                item,
		ConditionExpression: aws.String("attribute_not_exists(user_id)"),
	})
	return err
}

// FindByID retrieves a user by primary key.
func (r *Repository) FindByID(ctx context.Context, userID string) (*User, error) {
	out, err := r.db.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"user_id": &types.AttributeValueMemberS{Value: userID},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("get item: %w", err)
	}
	if out.Item == nil {
		return nil, errors.New("user not found")
	}
	var u User
	if err := attributevalue.UnmarshalMap(out.Item, &u); err != nil {
		return nil, fmt.Errorf("unmarshal user: %w", err)
	}
	return &u, nil
}

// FindByEmail queries the email GSI to look up a user by email.
func (r *Repository) FindByEmail(ctx context.Context, email string) (*User, error) {
	out, err := r.db.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		IndexName:              aws.String("email-index"),
		KeyConditionExpression: aws.String("email = :email"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":email": &types.AttributeValueMemberS{Value: email},
		},
		Limit: aws.Int32(1),
	})
	if err != nil {
		return nil, fmt.Errorf("query email index: %w", err)
	}
	if out.Count == 0 {
		return nil, errors.New("user not found")
	}
	var u User
	if err := attributevalue.UnmarshalMap(out.Items[0], &u); err != nil {
		return nil, fmt.Errorf("unmarshal user: %w", err)
	}
	return &u, nil
}

// UpdateProfile updates the username and optionally the avatar_url of an existing user.
// If avatarURL is empty the avatar_url attribute is left unchanged.
func (r *Repository) UpdateProfile(ctx context.Context, userID, username, avatarURL string) error {
	input := &dynamodb.UpdateItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"user_id": &types.AttributeValueMemberS{Value: userID},
		},
		UpdateExpression: aws.String("SET username = :username"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":username": &types.AttributeValueMemberS{Value: username},
		},
	}
	if avatarURL != "" {
		input.UpdateExpression = aws.String("SET username = :username, avatar_url = :avatar_url")
		input.ExpressionAttributeValues[":avatar_url"] = &types.AttributeValueMemberS{Value: avatarURL}
	}
	_, err := r.db.UpdateItem(ctx, input)
	return err
}

// AddToWatchlist atomically adds an auction_id to the user's watchlist string-set.
func (r *Repository) AddToWatchlist(ctx context.Context, userID, auctionID string) error {
	_, err := r.db.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"user_id": &types.AttributeValueMemberS{Value: userID},
		},
		UpdateExpression: aws.String("ADD watchlist :ids"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":ids": &types.AttributeValueMemberSS{Value: []string{auctionID}},
		},
		ConditionExpression: aws.String("attribute_exists(user_id)"),
	})
	return err
}

// RemoveFromWatchlist atomically removes an auction_id from the user's watchlist string-set.
func (r *Repository) RemoveFromWatchlist(ctx context.Context, userID, auctionID string) error {
	_, err := r.db.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"user_id": &types.AttributeValueMemberS{Value: userID},
		},
		UpdateExpression: aws.String("DELETE watchlist :ids"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":ids": &types.AttributeValueMemberSS{Value: []string{auctionID}},
		},
	})
	return err
}

// GetWatchlist returns the auction IDs saved on the user's watchlist.
// Returns an empty slice when the attribute does not exist (never saved anything).
func (r *Repository) GetWatchlist(ctx context.Context, userID string) ([]string, error) {
	out, err := r.db.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"user_id": &types.AttributeValueMemberS{Value: userID},
		},
		ProjectionExpression: aws.String("watchlist"),
	})
	if err != nil {
		return nil, fmt.Errorf("get item: %w", err)
	}
	if out.Item == nil {
		return nil, errors.New("user not found")
	}
	attr, ok := out.Item["watchlist"]
	if !ok {
		return []string{}, nil
	}
	ss, ok := attr.(*types.AttributeValueMemberSS)
	if !ok {
		return []string{}, nil
	}
	return ss.Value, nil
}

// UpdateRole updates the role of an existing user (e.g. buyer → seller upgrade).
func (r *Repository) UpdateRole(ctx context.Context, userID, role string) error {
	_, err := r.db.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"user_id": &types.AttributeValueMemberS{Value: userID},
		},
		UpdateExpression: aws.String("SET #r = :role"),
		ExpressionAttributeNames: map[string]string{
			"#r": "role",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":role": &types.AttributeValueMemberS{Value: role},
		},
	})
	return err
}
