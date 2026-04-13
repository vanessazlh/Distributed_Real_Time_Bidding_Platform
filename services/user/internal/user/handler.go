package user

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler holds HTTP handlers for the user domain.
type Handler struct {
	svc       *Service
	bidSvcURL string
}

// NewHandler creates a new Handler.
func NewHandler(svc *Service, bidSvcURL string) *Handler {
	return &Handler{svc: svc, bidSvcURL: bidSvcURL}
}

// Register godoc
// POST /users
func (h *Handler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, err := h.svc.Register(c.Request.Context(), req)
	if err != nil {
		var mismatch *UsernameMismatchError
		switch {
		case errors.As(err, &mismatch):
			c.JSON(http.StatusConflict, gin.H{
				"error":             "username_mismatch",
				"existing_username": mismatch.ExistingUsername,
			})
		case errors.Is(err, ErrEmailTaken):
			c.JSON(http.StatusConflict, gin.H{"error": "email already registered"})
		case errors.Is(err, ErrAlreadySeller):
			c.JSON(http.StatusConflict, gin.H{"error": "account is already a seller"})
		case errors.Is(err, ErrInvalidCredentials):
			c.JSON(http.StatusUnauthorized, gin.H{"error": "incorrect password for existing account"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		}
		return
	}

	c.JSON(http.StatusCreated, gin.H{"user_id": userID, "role": req.Role})
}

// Login godoc
// POST /auth/login
func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	token, err := h.svc.Login(c.Request.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidCredentials):
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": token})
}

// UpdateProfile godoc
// PUT /users/:user_id
func (h *Handler) UpdateProfile(c *gin.Context) {
	userID := c.Param("user_id")
	callerID, _ := c.Get("user_id")
	if userID != callerID {
		c.JSON(http.StatusForbidden, gin.H{"error": "cannot update another user's profile"})
		return
	}

	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.svc.UpdateProfile(c.Request.Context(), userID, req); err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// GetProfile godoc
// GET /users/:user_id
func (h *Handler) GetProfile(c *gin.Context) {
	userID := c.Param("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}

	u, err := h.svc.GetProfile(c.Request.Context(), userID)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		}
		return
	}

	c.JSON(http.StatusOK, u)
}

// GetBids godoc
// GET /users/:user_id/bids — proxied to the bid service
func (h *Handler) GetBids(c *gin.Context) {
	userID := c.Param("user_id")

	url := fmt.Sprintf("%s/users/%s/bids", h.bidSvcURL, userID)
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, url, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	// Forward the caller's JWT so the bid service can authenticate it.
	req.Header.Set("Authorization", c.GetHeader("Authorization"))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "bid service unavailable"})
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	var result json.RawMessage
	if err := json.Unmarshal(body, &result); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.Data(resp.StatusCode, "application/json", body)
}
