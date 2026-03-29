package auction

import (
	"context"
	"log"
	"time"
)

// Closer periodically checks for expired auctions and closes them.
type Closer struct {
	svc    *Service
	ticker *time.Ticker
	done   chan struct{}
}

// NewCloser creates a new Closer.
func NewCloser(svc *Service) *Closer {
	return &Closer{
		svc:  svc,
		done: make(chan struct{}),
	}
}

// Start begins the background goroutine that checks for expired auctions.
func (c *Closer) Start() {
	c.ticker = time.NewTicker(1 * time.Second)
	go func() {
		for {
			select {
			case <-c.ticker.C:
				c.checkExpired()
			case <-c.done:
				return
			}
		}
	}()
	log.Println("auction closer started")
}

// Stop stops the background goroutine.
func (c *Closer) Stop() {
	c.ticker.Stop()
	close(c.done)
	log.Println("auction closer stopped")
}

func (c *Closer) checkExpired() {
	ctx := context.Background()
	auctions, err := c.svc.ListAuctions(ctx, "OPEN")
	if err != nil {
		return
	}

	now := time.Now().UTC()
	for _, a := range auctions {
		if now.After(a.EndTime) {
			if err := c.svc.CloseAuction(ctx, a.AuctionID); err != nil {
				log.Printf("failed to auto-close auction %s: %v", a.AuctionID, err)
			} else {
				log.Printf("auto-closed auction %s", a.AuctionID)
			}
		}
	}
}
