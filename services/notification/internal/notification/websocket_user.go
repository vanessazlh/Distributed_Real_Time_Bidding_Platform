package notification

import (
	"log"
	"net/http"

	"rtb/shared/middleware"
)

// ServeUserWS upgrades an HTTP connection to WebSocket for user-level notifications.
// Route: GET /notifications/subscribe?token=<jwt>
//
// The JWT is passed as a query parameter because the browser WebSocket API
// does not support custom headers. The connection is kept open until the
// client disconnects; incoming frames are drained and discarded.
func ServeUserWS(hub *Hub, w http.ResponseWriter, r *http.Request) {
	tokenStr := r.URL.Query().Get("token")
	if tokenStr == "" {
		http.Error(w, "missing token", http.StatusUnauthorized)
		return
	}

	userID, err := middleware.VerifyJWT(tokenStr)
	if err != nil {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws-user: upgrade failed for user %s: %v", userID, err)
		return
	}

	client := &WSClient{conn: conn}
	hub.RegisterUser(userID, client)
	log.Printf("ws-user: client connected for user %s", userID)

	defer func() {
		hub.UnregisterUser(userID, client)
		conn.Close()
		log.Printf("ws-user: client disconnected for user %s", userID)
	}()

	// Drain incoming frames to keep the connection alive.
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}
}
