package api

import (
	"encoding/json"
	"net/http"

	"rtb/services/notification/internal/notification"
	"rtb/shared/middleware"
)

// RegisterRoutes attaches all notification-service HTTP handlers to mux.
//
// Routes:
//
//	GET  /auctions/{auction_id}/subscribe — per-auction WebSocket (push)
//	GET  /auctions/{auction_id}/subscribe/sse — per-auction SSE (push)
//	GET  /notifications/subscribe         — per-user WebSocket (global push, JWT in query)
//	GET  /notifications                   — list stored notifications (JWT in header)
//	POST /notifications/read              — mark all as read (JWT in header)
//	GET  /metrics                         — hub statistics
func RegisterRoutes(mux *http.ServeMux, hub *notification.Hub, store *notification.Store) {
	// Per-auction WebSocket.
	mux.HandleFunc("GET /auctions/{auction_id}/subscribe", func(w http.ResponseWriter, r *http.Request) {
		notification.ServeWS(hub, w, r)
	})

	// Per-auction Server-Sent Events.
	mux.HandleFunc("GET /auctions/{auction_id}/subscribe/sse", func(w http.ResponseWriter, r *http.Request) {
		notification.ServeSSE(hub, w, r)
	})

	// Per-user global WebSocket.
	mux.HandleFunc("GET /notifications/subscribe", func(w http.ResponseWriter, r *http.Request) {
		notification.ServeUserWS(hub, w, r)
	})

	// List stored notifications for the authenticated user.
	mux.HandleFunc("GET /notifications", func(w http.ResponseWriter, r *http.Request) {
		userID, err := extractUserID(r)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		notifications, err := store.List(r.Context(), userID)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		unread, _ := store.UnreadCount(r.Context(), userID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"notifications": notifications,
			"unread_count":  unread,
		})
	})

	// Mark all notifications as read.
	mux.HandleFunc("POST /notifications/read", func(w http.ResponseWriter, r *http.Request) {
		userID, err := extractUserID(r)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if err := store.MarkAllRead(r.Context(), userID); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
	})

	// Metrics endpoint.
	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		json.NewEncoder(w).Encode(hub.GetMetrics())
	})
}

// extractUserID parses the JWT from the Authorization header and returns the user ID.
func extractUserID(r *http.Request) (string, error) {
	header := r.Header.Get("Authorization")
	if len(header) < 8 || header[:7] != "Bearer " {
		return "", http.ErrNoCookie // reuse as generic error
	}
	return middleware.VerifyJWT(header[7:])
}
