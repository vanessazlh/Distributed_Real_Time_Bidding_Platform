package notification

import (
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Allow all origins during development; tighten in production.
	CheckOrigin: func(r *http.Request) bool { return true },
}

// WSClient wraps a gorilla WebSocket connection and implements Client.
// A mutex guards writes because gorilla/websocket allows only one concurrent writer.
type WSClient struct {
	mu   sync.Mutex
	conn *websocket.Conn
}

func (c *WSClient) Send(msg []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn.WriteMessage(websocket.TextMessage, msg)
}

func (c *WSClient) ClientType() string { return "websocket" }

// ServeWS upgrades an HTTP connection to WebSocket and registers it with the hub.
// Route: GET /auctions/{auction_id}/subscribe
//
// The connection is kept open until the client disconnects; all messages
// sent by the client are drained and discarded (the client is read-only for now).
func ServeWS(hub *Hub, w http.ResponseWriter, r *http.Request) {
	auctionID := r.PathValue("auction_id")
	if auctionID == "" {
		http.Error(w, "missing auction_id", http.StatusBadRequest)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws: upgrade failed for auction %s: %v", auctionID, err)
		return
	}

	client := &WSClient{conn: conn}
	hub.Register(auctionID, client)
	log.Printf("ws: client connected to auction %s", auctionID)

	defer func() {
		hub.Unregister(auctionID, client)
		conn.Close()
		log.Printf("ws: client disconnected from auction %s", auctionID)
	}()

	// Drain incoming frames (ping/pong/close) to keep the connection alive.
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}
}
