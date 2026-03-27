package api

import (
	"github.com/gin-gonic/gin"
	"github.com/surplus-auction/platform/internal/middleware"
	"github.com/surplus-auction/platform/internal/shop"
	"github.com/surplus-auction/platform/internal/user"
)

// NewRouter wires all routes and returns a configured *gin.Engine.
func NewRouter(userHandler *user.Handler, shopHandler *shop.Handler) *gin.Engine {
	r := gin.Default()

	// Public routes
	r.POST("/users", userHandler.Register)
	r.POST("/auth/login", userHandler.Login)
	r.GET("/shops/:shop_id", shopHandler.GetShop)
	r.GET("/shops/:shop_id/items", shopHandler.ListItems)

	// Protected routes
	protected := r.Group("/", middleware.Auth())
	{
		protected.GET("/users/:user_id", userHandler.GetProfile)
		protected.GET("/users/:user_id/bids", userHandler.GetBids)
		protected.POST("/shops", shopHandler.CreateShop)
		protected.POST("/shops/:shop_id/items", shopHandler.CreateItem)
	}

	return r
}
