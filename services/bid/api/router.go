package api

import (
	"github.com/gin-gonic/gin"
	"rtb/services/bid/internal/bid"
	"rtb/shared/middleware"
)

// NewRouter wires bid routes and returns a configured *gin.Engine.
func NewRouter(bidHandler *bid.Handler) *gin.Engine {
	r := gin.Default()

	// Public bid read routes
	r.GET("/auctions/:id/bids", bidHandler.GetAuctionBids)

	// Protected routes
	protected := r.Group("/", middleware.Auth())
	{
		protected.GET("/users/:user_id/bids", bidHandler.GetUserBids)
	}

	return r
}
