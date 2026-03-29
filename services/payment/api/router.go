package api

import (
	"github.com/gin-gonic/gin"
	"rtb/shared/middleware"
	"rtb/services/payment/internal/payment"
)

// NewRouter wires all routes and returns a configured *gin.Engine.
func NewRouter(paymentHandler *payment.Handler) *gin.Engine {
	r := gin.Default()

	// Protected payment routes — require JWT
	protected := r.Group("/", middleware.Auth())
	{
		protected.GET("/payments/:id", paymentHandler.GetPayment)
		protected.GET("/users/:user_id/payments", paymentHandler.GetUserPayments)
		protected.GET("/auctions/:auction_id/payment", paymentHandler.GetAuctionPayment)
	}

	// Admin routes — no auth for simplicity (add auth in production)
	admin := r.Group("/admin")
	{
		admin.POST("/payments/:id/process", paymentHandler.ProcessPayment)
		admin.POST("/payments/:id/refund", paymentHandler.RefundPayment)
	}

	return r
}
