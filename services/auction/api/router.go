package api

import (
	"github.com/gin-gonic/gin"
	"rtb/services/auction/internal/auction"
	"rtb/shared/middleware"
)

// NewRouter wires auction routes and returns a configured *gin.Engine.
func NewRouter(auctionHandler *auction.Handler) *gin.Engine {
	r := gin.Default()

	// Public auction read routes
	r.GET("/auctions", auctionHandler.ListAuctions)
	r.GET("/auctions/:id", auctionHandler.GetAuction)

	// Protected routes
	protected := r.Group("/", middleware.Auth())
	{
		protected.POST("/auctions", auctionHandler.CreateAuction)
		protected.POST("/auctions/:id/bid", auctionHandler.PlaceBid)
		protected.POST("/auctions/:id/close", auctionHandler.CloseAuction)
	}

	// Admin routes (for experiment support)
	admin := r.Group("/admin")
	{
		admin.GET("/metrics", auctionHandler.GetMetrics)
		admin.POST("/metrics/reset", auctionHandler.ResetMetrics)
		admin.GET("/strategy", auctionHandler.GetStrategy)
		admin.PUT("/strategy", auctionHandler.SetStrategy)
	}

	return r
}
