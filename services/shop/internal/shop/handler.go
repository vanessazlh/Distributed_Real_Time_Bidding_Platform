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

// callerRole extracts the role claim set by the JWT middleware.
func callerRole(c *gin.Context) string {
	v, _ := c.Get("role")
	r, _ := v.(string)
	return r
}

// callerUsername extracts the username claim set by the JWT middleware.
func callerUsername(c *gin.Context) string {
	v, _ := c.Get("username")
	u, _ := v.(string)
	return u
}

// CreateShop godoc
// POST /shops
func (h *Handler) CreateShop(c *gin.Context) {
	if callerRole(c) != "seller" {
		c.JSON(http.StatusForbidden, gin.H{"error": "only sellers can create shops"})
		return
	}

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

// UpdateShop godoc
// PUT /shops/:shop_id
func (h *Handler) UpdateShop(c *gin.Context) {
	if callerRole(c) != "seller" {
		c.JSON(http.StatusForbidden, gin.H{"error": "only sellers can update shops"})
		return
	}

	shopID := c.Param("shop_id")

	var req UpdateShopRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	shop, err := h.svc.UpdateShop(c.Request.Context(), shopID, req, callerID(c))
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "shop not found"})
		case errors.Is(err, ErrForbidden):
			c.JSON(http.StatusForbidden, gin.H{"error": "only the shop owner can update this shop"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		}
		return
	}

	c.JSON(http.StatusOK, shop)
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

// GetItem godoc
// GET /items/:item_id
func (h *Handler) GetItem(c *gin.Context) {
	itemID := c.Param("item_id")
	item, err := h.svc.GetItem(c.Request.Context(), itemID)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "item not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		}
		return
	}

	c.JSON(http.StatusOK, item)
}

// CreateItem godoc
// POST /shops/:shop_id/items
func (h *Handler) CreateItem(c *gin.Context) {
	if callerRole(c) != "seller" {
		c.JSON(http.StatusForbidden, gin.H{"error": "only sellers can add items"})
		return
	}

	shopID := c.Param("shop_id")

	var req CreateItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	item, err := h.svc.CreateItem(c.Request.Context(), shopID, req, callerID(c))
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidInput):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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

// ListSellerShops godoc
// GET /sellers/:user_id/shops
func (h *Handler) ListSellerShops(c *gin.Context) {
	ownerID := c.Param("user_id")
	shops, err := h.svc.ListSellerShops(c.Request.Context(), ownerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"shops": shops})
}

// UploadImage godoc
// POST /uploads
func (h *Handler) UploadImage(c *gin.Context) {
	if callerRole(c) != "seller" {
		c.JSON(http.StatusForbidden, gin.H{"error": "only sellers can upload images"})
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 6*1024*1024)

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}
	defer file.Close()

	resp, err := h.svc.UploadImage(c.Request.Context(), header.Header.Get("Content-Type"), file, header.Size)
	if err != nil {
		switch {
		case errors.Is(err, ErrFileTooLarge):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, ErrInvalidFileType):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, ErrUploadNotConfigured):
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "image upload is not available"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "upload failed"})
		}
		return
	}

	c.JSON(http.StatusOK, resp)
}

// CreateReview godoc
// POST /shops/:shop_id/reviews
func (h *Handler) CreateReview(c *gin.Context) {
	shopID := c.Param("shop_id")

	var req CreateReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	reviewerID := callerID(c)
	reviewerUsername := callerUsername(c)

	rev, err := h.svc.CreateReview(c.Request.Context(), shopID, req, reviewerID, reviewerUsername)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "shop not found"})
		case errors.Is(err, ErrAlreadyReviewed):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		case errors.Is(err, ErrPaymentNotCompleted):
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		}
		return
	}

	c.JSON(http.StatusCreated, rev)
}

// ListReviews godoc
// GET /shops/:shop_id/reviews
func (h *Handler) ListReviews(c *gin.Context) {
	shopID := c.Param("shop_id")
	resp, err := h.svc.ListReviews(c.Request.Context(), shopID)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "shop not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		}
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ReplyToReview godoc
// POST /shops/:shop_id/reviews/:review_id/reply
func (h *Handler) ReplyToReview(c *gin.Context) {
	if callerRole(c) != "seller" {
		c.JSON(http.StatusForbidden, gin.H{"error": "only sellers can reply to reviews"})
		return
	}

	shopID := c.Param("shop_id")
	reviewID := c.Param("review_id")

	var req ReplyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	rev, err := h.svc.ReplyToReview(c.Request.Context(), shopID, reviewID, req.Reply, callerID(c))
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "review not found"})
		case errors.Is(err, ErrForbidden):
			c.JSON(http.StatusForbidden, gin.H{"error": "only the shop owner can reply to reviews"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		}
		return
	}

	c.JSON(http.StatusOK, rev)
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
