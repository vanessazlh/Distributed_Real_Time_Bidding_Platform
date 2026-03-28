package bid

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler holds HTTP handlers for the bid domain.
type Handler struct {
	svc *Service
}

// NewHandler creates a new Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// GetAuctionBids godoc
// GET /auctions/:id/bids
func (h *Handler) GetAuctionBids(c *gin.Context) {
	auctionID := c.Param("id")

	bids, err := h.svc.GetAuctionBids(c.Request.Context(), auctionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"bids": bids})
}

// GetUserBids godoc
// GET /users/:user_id/bids
func (h *Handler) GetUserBids(c *gin.Context) {
	userID := c.Param("user_id")

	bids, err := h.svc.GetUserBids(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"bids": bids})
}
