package notification

import (
	"fmt"
	"log"
	"net/http"
	"sync"
)

// SSEClient writes Server-Sent Events to an HTTP response writer.
// A mutex guards writes because broadcasts can arrive concurrently.
type SSEClient struct {
	mu      sync.Mutex
	w       http.ResponseWriter
	flusher http.Flusher
}

func (c *SSEClient) Send(msg []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, err := fmt.Fprintf(c.w, "data: %s\n\n", msg); err != nil {
		return err
	}
	c.flusher.Flush()
	return nil
}

func (c *SSEClient) ClientType() string { return "sse" }

// ServeSSE handles Server-Sent Events connections and registers them with the hub.
// Route: GET /auctions/{auction_id}/subscribe/sse
//
// The handler blocks until the client disconnects (r.Context().Done()), at which
// point the deferred Unregister cleans up the hub entry.
func ServeSSE(hub *Hub, w http.ResponseWriter, r *http.Request) {
	auctionID := r.PathValue("auction_id")
	if auctionID == "" {
		http.Error(w, "missing auction_id", http.StatusBadRequest)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported by this server", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	client := &SSEClient{w: w, flusher: flusher}
	hub.Register(auctionID, client)
	log.Printf("sse: client connected to auction %s", auctionID)

	defer func() {
		hub.Unregister(auctionID, client)
		log.Printf("sse: client disconnected from auction %s", auctionID)
	}()

	// Send an initial "connected" event so the client knows the stream is live.
	fmt.Fprintf(w, "event: connected\ndata: {\"auction_id\":\"%s\"}\n\n", auctionID)
	flusher.Flush()

	// Block until the client closes the connection.
	<-r.Context().Done()
}
