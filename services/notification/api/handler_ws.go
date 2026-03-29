package api

import (
	"encoding/json"
	"net/http"

	"rtb/services/notification/internal/notification"
)

// RegisterRoutes attaches all notification-service HTTP handlers to mux.
//
// Routes:
//
//	GET /auctions/{auction_id}/subscribe      — WebSocket (push)
//	GET /auctions/{auction_id}/subscribe/sse  — Server-Sent Events (push)
//	GET /metrics                              — hub statistics for Experiment 3
func RegisterRoutes(mux *http.ServeMux, hub *notification.Hub) {
	// WebSocket push endpoint.
	mux.HandleFunc("GET /auctions/{auction_id}/subscribe", func(w http.ResponseWriter, r *http.Request) {
		notification.ServeWS(hub, w, r)
	})

	// SSE push endpoint.
	// Note: registered before the broader /subscribe pattern so Go 1.22 routing
	// matches /subscribe/sse first (more specific wins).
	mux.HandleFunc("GET /auctions/{auction_id}/subscribe/sse", func(w http.ResponseWriter, r *http.Request) {
		notification.ServeSSE(hub, w, r)
	})

	// Metrics endpoint — used by Person 4 during Experiment 3 to measure
	// resource usage (active connections, broadcasts sent) as client count scales.
	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		json.NewEncoder(w).Encode(hub.GetMetrics())
	})
}
