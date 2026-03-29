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
	"github.com/gin-gonic/gin"
	"rtb/services/shop/internal/shop"
	"rtb/shared/middleware"
)

func main() {
	db := newDynamoClient()

	repo := shop.NewRepository(db)
	svc := shop.NewService(repo)
	h := shop.NewHandler(svc)

	r := gin.Default()

	r.GET("/shops/:shop_id", h.GetShop)
	r.GET("/shops/:shop_id/items", h.ListItems)
	r.GET("/items/:item_id", h.GetItem)

	protected := r.Group("/", middleware.Auth())
	{
		protected.POST("/shops", h.CreateShop)
		protected.POST("/shops/:shop_id/items", h.CreateItem)
	}

	addr := envOr("SERVER_ADDR", ":8082")
	srv := &http.Server{Addr: addr, Handler: r}

	go func() {
		log.Printf("shop service listening on %s", addr)
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
