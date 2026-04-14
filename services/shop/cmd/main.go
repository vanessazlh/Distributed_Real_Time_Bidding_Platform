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
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
	"rtb/services/shop/internal/shop"
	"rtb/shared/middleware"
)

func main() {
	db := newDynamoClient()
	repo := shop.NewRepository(db)

	var uploader shop.Uploader
	var publicURL string
	if s3Client := newS3Client(); s3Client != nil {
		bucket := envOr("S3_BUCKET", "uploads")
		s3up := shop.NewS3Uploader(s3Client, bucket)
		if err := s3up.EnsureBucket(context.Background()); err != nil {
			log.Fatalf("ensure S3 bucket: %v", err)
		}
		uploader = s3up
		publicURL = envOr("S3_PUBLIC_URL", "http://localhost:3000/uploads")
		log.Printf("image uploads enabled (bucket=%s)", bucket)
	}

	svc := shop.NewService(repo, uploader, publicURL)
	h := shop.NewHandler(svc)

	r := gin.Default()

	r.GET("/shops/:shop_id", h.GetShop)
	r.GET("/shops/:shop_id/items", h.ListItems)
	r.GET("/items/:item_id", h.GetItem)

	protected := r.Group("/", middleware.Auth())
	{
		protected.POST("/shops", h.CreateShop)
		protected.PUT("/shops/:shop_id", h.UpdateShop)
		protected.POST("/shops/:shop_id/items", h.CreateItem)
		protected.GET("/sellers/:user_id/shops", h.ListSellerShops)
		protected.POST("/uploads", h.UploadImage)
	}

	addr := envOr("SERVER_ADDR", ":8083")
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
	endpoint := os.Getenv("DYNAMODB_ENDPOINT")
	region := envOr("AWS_REGION", "us-east-1")

	opts := []func(*config.LoadOptions) error{config.WithRegion(region)}
	if endpoint != "" {
		// Local dev: custom endpoint (DynamoDB Local) + static credentials
		opts = append(opts,
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("local", "local", "")),
			config.WithEndpointResolverWithOptions(
				aws.EndpointResolverWithOptionsFunc(func(service, reg string, _ ...interface{}) (aws.Endpoint, error) {
					return aws.Endpoint{URL: endpoint}, nil
				}),
			),
		)
	}
	// Production (ECS): endpoint unset → SDK uses ECS task role credentials + real DynamoDB

	cfg, err := config.LoadDefaultConfig(context.Background(), opts...)
	if err != nil {
		log.Fatalf("load aws config: %v", err)
	}
	return dynamodb.NewFromConfig(cfg)
}

func newS3Client() *s3.Client {
	region := envOr("AWS_REGION", "us-east-1")
	endpoint := os.Getenv("S3_ENDPOINT")

	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(region))
	if err != nil {
		log.Fatalf("load s3 config: %v", err)
	}

	if endpoint != "" {
		// Local dev: MinIO with custom endpoint + static credentials
		cfg, err = config.LoadDefaultConfig(context.Background(),
			config.WithRegion(region),
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
				envOr("S3_ACCESS_KEY", "minioadmin"),
				envOr("S3_SECRET_KEY", "minioadmin"),
				"",
			)),
		)
		if err != nil {
			log.Fatalf("load s3 config: %v", err)
		}
		return s3.NewFromConfig(cfg, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(endpoint)
			o.UsePathStyle = true
		})
	}
	// Production (ECS): endpoint unset → real S3 with ECS task role credentials
	return s3.NewFromConfig(cfg)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
