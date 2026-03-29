package payment

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler exposes payment endpoints.
type Handler struct {
	svc *Service
}

// NewHandler creates a new Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// GetPayment handles GET /payments/:id
func (h *Handler) GetPayment(c *gin.Context) {
	resp, err := h.svc.GetPayment(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// GetUserPayments handles GET /users/:user_id/payments
func (h *Handler) GetUserPayments(c *gin.Context) {
	callerID, _ := c.Get("user_id")
	requestedID := c.Param("user_id")
	if callerID != requestedID {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	resp, err := h.svc.GetUserPayments(c.Request.Context(), requestedID)
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// GetAuctionPayment handles GET /auctions/:auction_id/payment
func (h *Handler) GetAuctionPayment(c *gin.Context) {
	resp, err := h.svc.GetPaymentByAuction(c.Request.Context(), c.Param("auction_id"))
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ProcessPayment handles POST /payments/:id/process (admin)
func (h *Handler) ProcessPayment(c *gin.Context) {
	if err := h.svc.ProcessPayment(c.Request.Context(), c.Param("id")); err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "payment processing initiated"})
}

// RefundPayment handles POST /payments/:id/refund (admin)
func (h *Handler) RefundPayment(c *gin.Context) {
	if err := h.svc.RefundPayment(c.Request.Context(), c.Param("id")); err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "refund processed"})
}

func (h *Handler) writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, ErrAlreadyExists):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, ErrInvalidStatus):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
