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
	"github.com/surplus-auction/platform/api"
	shopPkg "github.com/surplus-auction/platform/internal/shop"
	userPkg "github.com/surplus-auction/platform/internal/user"
)

func main() {
	db := newDynamoClient()

	// Wire user layer
	userRepo := userPkg.NewRepository(db)
	userSvc := userPkg.NewService(userRepo)
	userHandler := userPkg.NewHandler(userSvc)

	// Wire shop layer
	shopRepo := shopPkg.NewRepository(db)
	shopSvc := shopPkg.NewService(shopRepo)
	shopHandler := shopPkg.NewHandler(shopSvc)

	router := api.NewRouter(userHandler, shopHandler)

	addr := envOr("SERVER_ADDR", ":8080")
	srv := &http.Server{Addr: addr, Handler: router}

	go func() {
		log.Printf("server listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("shutdown: %v", err)
	}
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

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
