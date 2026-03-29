package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/redis/go-redis/v9"
	"github.com/surplus-auction/platform/api"
	"github.com/surplus-auction/platform/internal/events"
	paymentPkg "github.com/surplus-auction/platform/internal/payment"
)

func main() {
	db := newDynamoClient()
	rdb := newRedisClient()

	ensureTable(db)

	// Wire payment layer
	paymentRepo := paymentPkg.NewRepository(db)
	publisher := events.NewPublisher(rdb)
	paymentSvc := paymentPkg.NewService(paymentRepo, publisher)
	paymentHandler := paymentPkg.NewHandler(paymentSvc)

	// Start auction_closed event consumer
	consumer := events.NewConsumer(rdb, paymentSvc)
	consumer.Start()

	router := api.NewRouter(paymentHandler)

	addr := envOr("SERVER_ADDR", ":8081")
	srv := &http.Server{Addr: addr, Handler: router}

	go func() {
		log.Printf("payment service listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("shutting down...")

	consumer.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("shutdown: %v", err)
	}
}

// ensureTable creates the payments DynamoDB table if it doesn't exist.
func ensureTable(db *dynamodb.Client) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := db.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName: aws.String("payments"),
		AttributeDefinitions: []types.AttributeDefinition{
			{AttributeName: aws.String("payment_id"), AttributeType: types.ScalarAttributeTypeS},
			{AttributeName: aws.String("auction_id"), AttributeType: types.ScalarAttributeTypeS},
			{AttributeName: aws.String("user_id"), AttributeType: types.ScalarAttributeTypeS},
			{AttributeName: aws.String("created_at"), AttributeType: types.ScalarAttributeTypeS},
		},
		KeySchema: []types.KeySchemaElement{
			{AttributeName: aws.String("payment_id"), KeyType: types.KeyTypeHash},
		},
		GlobalSecondaryIndexes: []types.GlobalSecondaryIndex{
			{
				IndexName: aws.String("auction-index"),
				KeySchema: []types.KeySchemaElement{
					{AttributeName: aws.String("auction_id"), KeyType: types.KeyTypeHash},
				},
				Projection: &types.Projection{ProjectionType: types.ProjectionTypeAll},
				ProvisionedThroughput: &types.ProvisionedThroughput{
					ReadCapacityUnits:  aws.Int64(5),
					WriteCapacityUnits: aws.Int64(5),
				},
			},
			{
				IndexName: aws.String("user-index"),
				KeySchema: []types.KeySchemaElement{
					{AttributeName: aws.String("user_id"), KeyType: types.KeyTypeHash},
					{AttributeName: aws.String("created_at"), KeyType: types.KeyTypeRange},
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
	if err != nil {
		// Table likely already exists — that's fine.
		log.Printf("ensureTable: %v (may already exist)", err)
		return
	}
	log.Println("payments table created")
}

func newDynamoClient() *dynamodb.Client {
	endpoint := envOr("DYNAMODB_ENDPOINT", "http://localhost:8000")
	region := envOr("AWS_REGION", "us-east-1")

	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("local", "local", "")),
		config.WithEndpointResolverWithOptions(
			aws.EndpointResolverWithOptionsFunc(func(service, reg string, _ ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{URL: endpoint}, nil
			}),
		),
	)
	if err != nil {
		log.Fatalf("load aws config: %v", err)
	}
	return dynamodb.NewFromConfig(cfg)
}

func newRedisClient() *redis.Client {
	addr := envOr("REDIS_ADDR", "localhost:6379")
	rdb := redis.NewClient(&redis.Options{Addr: addr})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("connect redis: %v", err)
	}
	log.Printf("connected to redis at %s", addr)
	return rdb
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
