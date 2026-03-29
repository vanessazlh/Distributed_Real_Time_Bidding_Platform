//go:build ignore

// Run with: go run scripts/init_tables.go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func main() {
	db := newClient()
	ctx := context.Background()

	if err := createUsersTable(ctx, db); err != nil {
		log.Printf("Users table: %v", err)
	} else {
		fmt.Println("created Users table")
	}

	if err := createShopsTable(ctx, db); err != nil {
		log.Printf("Shops table: %v", err)
	} else {
		fmt.Println("created Shops table")
	}

	if err := createItemsTable(ctx, db); err != nil {
		log.Printf("Items table: %v", err)
	} else {
		fmt.Println("created Items table")
	}
}

func createUsersTable(ctx context.Context, db *dynamodb.Client) error {
	_, err := db.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName: aws.String("Users"),
		AttributeDefinitions: []types.AttributeDefinition{
			{AttributeName: aws.String("user_id"), AttributeType: types.ScalarAttributeTypeS},
			{AttributeName: aws.String("email"), AttributeType: types.ScalarAttributeTypeS},
		},
		KeySchema: []types.KeySchemaElement{
			{AttributeName: aws.String("user_id"), KeyType: types.KeyTypeHash},
		},
		GlobalSecondaryIndexes: []types.GlobalSecondaryIndex{
			{
				IndexName: aws.String("email-index"),
				KeySchema: []types.KeySchemaElement{
					{AttributeName: aws.String("email"), KeyType: types.KeyTypeHash},
				},
				Projection: &types.Projection{ProjectionType: types.ProjectionTypeAll},
				ProvisionedThroughput: &types.ProvisionedThroughput{
					ReadCapacityUnits:  aws.Int64(5),
					WriteCapacityUnits: aws.Int64(5),
				},
			},
		},
		ProvisionedThroughput: &types.ProvisionedThroughput{
			ReadCapacityUnits:  aws.Int64(5),
			WriteCapacityUnits: aws.Int64(5),
		},
	})
	return err
}

func createShopsTable(ctx context.Context, db *dynamodb.Client) error {
	_, err := db.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName: aws.String("Shops"),
		AttributeDefinitions: []types.AttributeDefinition{
			{AttributeName: aws.String("shop_id"), AttributeType: types.ScalarAttributeTypeS},
		},
		KeySchema: []types.KeySchemaElement{
			{AttributeName: aws.String("shop_id"), KeyType: types.KeyTypeHash},
		},
		ProvisionedThroughput: &types.ProvisionedThroughput{
			ReadCapacityUnits:  aws.Int64(5),
			WriteCapacityUnits: aws.Int64(5),
		},
	})
	return err
}

func createItemsTable(ctx context.Context, db *dynamodb.Client) error {
	_, err := db.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName: aws.String("Items"),
		AttributeDefinitions: []types.AttributeDefinition{
			{AttributeName: aws.String("item_id"), AttributeType: types.ScalarAttributeTypeS},
			{AttributeName: aws.String("shop_id"), AttributeType: types.ScalarAttributeTypeS},
		},
		KeySchema: []types.KeySchemaElement{
			{AttributeName: aws.String("item_id"), KeyType: types.KeyTypeHash},
		},
		GlobalSecondaryIndexes: []types.GlobalSecondaryIndex{
			{
				IndexName: aws.String("shop_id-index"),
				KeySchema: []types.KeySchemaElement{
					{AttributeName: aws.String("shop_id"), KeyType: types.KeyTypeHash},
				},
				Projection: &types.Projection{ProjectionType: types.ProjectionTypeAll},
				ProvisionedThroughput: &types.ProvisionedThroughput{
					ReadCapacityUnits:  aws.Int64(5),
					WriteCapacityUnits: aws.Int64(5),
				},
			},
		},
		ProvisionedThroughput: &types.ProvisionedThroughput{
			ReadCapacityUnits:  aws.Int64(5),
			WriteCapacityUnits: aws.Int64(5),
		},
	})
	return err
}

func newClient() *dynamodb.Client {
	endpoint := os.Getenv("DYNAMODB_ENDPOINT")
	if endpoint == "" {
		endpoint = "http://localhost:8000"
	}
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("local", "local", "")),
		config.WithEndpointResolverWithOptions(
			aws.EndpointResolverWithOptionsFunc(func(service, region string, _ ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{URL: endpoint}, nil
			}),
		),
	)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	return dynamodb.NewFromConfig(cfg)
}
