package auction

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handler holds HTTP handlers for the auction domain.
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

// callerRole extracts the authenticated user role set by the JWT middleware.
func callerRole(c *gin.Context) string {
	v, _ := c.Get("role")
	role, _ := v.(string)
	return role
}

// CreateAuction godoc
// POST /auctions
func (h *Handler) CreateAuction(c *gin.Context) {
	if callerRole(c) != "seller" {
		c.JSON(http.StatusForbidden, gin.H{"error": "only sellers can create auctions"})
		return
	}

	var req CreateAuctionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	sellerID := callerID(c)
	auction, err := h.svc.CreateAuction(c.Request.Context(), req, sellerID)
	if err != nil {
		if errors.Is(err, ErrValidation) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusCreated, auction)
}

// GetAuction godoc
// GET /auctions/:id
func (h *Handler) GetAuction(c *gin.Context) {
	auctionID := c.Param("id")

	auction, err := h.svc.GetAuction(c.Request.Context(), auctionID)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "auction not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		}
		return
	}

	c.JSON(http.StatusOK, auction)
}

// ListAuctions godoc
// GET /auctions
// Optional geo filter: ?lat=<float>&lng=<float>&radius_km=<float>
func (h *Handler) ListAuctions(c *gin.Context) {
	latStr := c.Query("lat")
	lngStr := c.Query("lng")
	radiusStr := c.Query("radius_km")

	if latStr != "" && lngStr != "" && radiusStr != "" {
		lat, errLat := strconv.ParseFloat(latStr, 64)
		lng, errLng := strconv.ParseFloat(lngStr, 64)
		radius, errR := strconv.ParseFloat(radiusStr, 64)
		if errLat == nil && errLng == nil && errR == nil && radius > 0 {
			auctions, err := h.svc.ListAuctionsNear(c.Request.Context(), lat, lng, radius)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"auctions": auctions})
			return
		}
	}

	status := c.Query("status")
	auctions, err := h.svc.ListAuctions(c.Request.Context(), status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"auctions": auctions})
}

// ListShopAuctions godoc
// GET /shops/:shop_id/auctions
func (h *Handler) ListShopAuctions(c *gin.Context) {
	shopID := c.Param("shop_id")

	auctions, err := h.svc.ListAuctionsByShop(c.Request.Context(), shopID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"auctions": auctions})
}

// PlaceBid godoc
// POST /auctions/:id/bid
func (h *Handler) PlaceBid(c *gin.Context) {
	auctionID := c.Param("id")
	userID := callerID(c)

	var req PlaceBidRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.svc.PlaceBid(c.Request.Context(), auctionID, userID, req.Amount)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "auction not found"})
		case errors.Is(err, ErrAuctionClosed):
			c.JSON(http.StatusConflict, gin.H{"error": "auction is not open"})
		case errors.Is(err, ErrBidTooLow):
			c.JSON(http.StatusBadRequest, gin.H{"error": "bid must be higher than current highest"})
		case errors.Is(err, ErrBidExceedsMax):
			c.JSON(http.StatusBadRequest, gin.H{"error": "bid exceeds the maximum price for this auction"})
		case errors.Is(err, ErrBidIncrementTooSmall):
			c.JSON(http.StatusBadRequest, gin.H{"error": "bid does not meet the minimum increment for this auction"})
		case errors.Is(err, ErrSelfBid):
			c.JSON(http.StatusForbidden, gin.H{"error": "sellers cannot bid on their own auction"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		}
		return
	}

	c.JSON(http.StatusCreated, result)
}

// CloseAuction godoc
// POST /auctions/:id/close
func (h *Handler) CloseAuction(c *gin.Context) {
	auctionID := c.Param("id")
	userID := callerID(c)

	// Ownership check: only the seller who owns the shop can close the auction
	auction, err := h.svc.GetAuction(c.Request.Context(), auctionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "auction not found"})
		return
	}
	if auction.SellerID != "" && auction.SellerID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "you do not own this auction"})
		return
	}

	err = h.svc.CloseAuction(c.Request.Context(), auctionID)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "auction not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "auction closed"})
}

// GetMetrics godoc
// GET /admin/metrics
func (h *Handler) GetMetrics(c *gin.Context) {
	c.JSON(http.StatusOK, h.svc.GetMetrics())
}

// ResetMetrics godoc
// POST /admin/metrics/reset
func (h *Handler) ResetMetrics(c *gin.Context) {
	h.svc.ResetMetrics()
	c.JSON(http.StatusOK, gin.H{"message": "metrics reset"})
}

// GetStrategy godoc
// GET /admin/strategy
func (h *Handler) GetStrategy(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"strategy": string(h.svc.GetStrategy())})
}

// SetStrategy godoc
// PUT /admin/strategy
func (h *Handler) SetStrategy(c *gin.Context) {
	var req struct {
		Strategy string `json:"strategy" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	switch ConcurrencyStrategy(req.Strategy) {
	case Optimistic, Pessimistic, Queue:
		h.svc.SetStrategy(ConcurrencyStrategy(req.Strategy))
		c.JSON(http.StatusOK, gin.H{"strategy": req.Strategy})
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid strategy, must be: optimistic, pessimistic, or queue"})
	}
}
