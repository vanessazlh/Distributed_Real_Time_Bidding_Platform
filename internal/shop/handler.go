package shop

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler holds HTTP handlers for the shop domain.
type Handler struct {
	svc *Service
}

// NewHandler creates a new Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// callerID extracts the authenticated user ID set by the JWT middleware.
func callerID(c *gin.Context) string {
	v, _ := c.Get("user_id")
	id, _ := v.(string)
	return id
}

// CreateShop godoc
// POST /shops
func (h *Handler) CreateShop(c *gin.Context) {
	var req CreateShopRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ownerID := callerID(c)
	shop, err := h.svc.CreateShop(c.Request.Context(), req, ownerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusCreated, shop)
}

// GetShop godoc
// GET /shops/:shop_id
func (h *Handler) GetShop(c *gin.Context) {
	shopID := c.Param("shop_id")
	shop, err := h.svc.GetShop(c.Request.Context(), shopID)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "shop not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		}
		return
	}

	c.JSON(http.StatusOK, shop)
}

// CreateItem godoc
// POST /shops/:shop_id/items
func (h *Handler) CreateItem(c *gin.Context) {
	shopID := c.Param("shop_id")

	var req CreateItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	item, err := h.svc.CreateItem(c.Request.Context(), shopID, req, callerID(c))
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "shop not found"})
		case errors.Is(err, ErrForbidden):
			c.JSON(http.StatusForbidden, gin.H{"error": "only the shop owner can add items"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		}
		return
	}

	c.JSON(http.StatusCreated, item)
}

// ListItems godoc
// GET /shops/:shop_id/items
func (h *Handler) ListItems(c *gin.Context) {
	shopID := c.Param("shop_id")

	items, err := h.svc.ListItems(c.Request.Context(), shopID)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "shop not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"items": items})
}
