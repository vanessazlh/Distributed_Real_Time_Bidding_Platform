package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"rtb/services/bid/api"
	bidPkg "rtb/services/bid/internal/bid"
	"rtb/services/bid/internal/events"
)

func main() {
	rdb := newRedisClient()

	// Wire bid layer
	bidRepo := bidPkg.NewRepository(rdb)
	bidSvc := bidPkg.NewService(bidRepo)
	bidHandler := bidPkg.NewHandler(bidSvc)

	// Start event consumer — listens for bid_placed events from Auction Service
	consumer := events.NewConsumer(rdb, bidSvc)
	consumer.Start()

	router := api.NewRouter(bidHandler)

	addr := envOr("SERVER_ADDR", ":8082")
	srv := &http.Server{Addr: addr, Handler: router}

	go func() {
		log.Printf("bid service listening on %s", addr)
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

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
