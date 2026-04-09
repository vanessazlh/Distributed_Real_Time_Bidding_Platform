package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	awsCfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/redis/go-redis/v9"
	"rtb/services/auction/api"
	auctionPkg "rtb/services/auction/internal/auction"
	"rtb/services/auction/internal/events"
)

func main() {
	rdb := newRedisClient()
	db := newDynamoClient()

	// Wire event publisher
	publisher := events.NewPublisher(rdb)

	// Wire auction layer
	strategy := auctionPkg.ConcurrencyStrategy(envOr("CONCURRENCY_STRATEGY", "pessimistic"))
	var auctionRepo *auctionPkg.Repository
	if db != nil {
		auctionRepo = auctionPkg.NewRepositoryWithDynamo(rdb, db)
		log.Println("auction repository: Redis + DynamoDB backing store")
	} else {
		auctionRepo = auctionPkg.NewRepository(rdb)
		log.Println("auction repository: Redis only (no DynamoDB)")
	}
	auctionSvc := auctionPkg.NewService(auctionRepo, publisher, rdb, strategy)
	auctionHandler := auctionPkg.NewHandler(auctionSvc)

	// Start auction auto-closer
	closer := auctionPkg.NewCloser(auctionSvc)
	closer.Start()

	router := api.NewRouter(auctionHandler)

	addr := envOr("SERVER_ADDR", ":8081")
	srv := &http.Server{Addr: addr, Handler: router}

	go func() {
		log.Printf("auction service listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("shutting down...")

	closer.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("shutdown: %v", err)
	}
}

func newRedisClient() *redis.Client {
	addr := envOr("REDIS_ADDR", "localhost:6379")
	rdb := redis.NewClient(&redis.Options{
		Addr: addr,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("connect redis: %v", err)
	}
	log.Printf("connected to redis at %s", addr)
	return rdb
}

func newDynamoClient() *dynamodb.Client {
	endpoint := os.Getenv("DYNAMODB_ENDPOINT")
	if endpoint == "" {
		log.Println("DYNAMODB_ENDPOINT not set — DynamoDB backing store disabled")
		return nil
	}

	region := envOr("AWS_REGION", "us-east-1")
	cfg, err := awsCfg.LoadDefaultConfig(context.Background(),
		awsCfg.WithRegion(region),
		awsCfg.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("local", "local", "")),
		awsCfg.WithEndpointResolverWithOptions(
			aws.EndpointResolverWithOptionsFunc(func(service, reg string, _ ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{URL: endpoint}, nil
			}),
		),
	)
	if err != nil {
		log.Printf("WARN: failed to load AWS config — DynamoDB disabled: %v", err)
		return nil
	}
	log.Printf("connected to DynamoDB at %s", endpoint)
	return dynamodb.NewFromConfig(cfg)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
