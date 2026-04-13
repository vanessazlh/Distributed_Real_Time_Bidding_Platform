package auction

import (
	"context"
	"log"
	"time"
)

// Closer periodically checks for expired auctions and closes them.
// It also transitions PENDING auctions to OPEN when their start_time arrives.
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

// Start begins the background goroutine that checks for expired auctions
// and opens scheduled auctions.
func (c *Closer) Start() {
	c.ticker = time.NewTicker(1 * time.Second)
	go func() {
		for {
			select {
			case <-c.ticker.C:
				c.checkExpired()
				c.checkPending()
			case <-c.done:
				return
			}
		}
	}()
	log.Println("auction closer started (close + open)")
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

func (c *Closer) checkPending() {
	ctx := context.Background()
	auctions, err := c.svc.ListAuctions(ctx, "PENDING")
	if err != nil {
		return
	}

	now := time.Now().UTC()
	for _, a := range auctions {
		if now.After(a.StartTime) || now.Equal(a.StartTime) {
			if err := c.svc.OpenAuction(ctx, a.AuctionID); err != nil {
				log.Printf("failed to auto-open auction %s: %v", a.AuctionID, err)
			} else {
				log.Printf("auto-opened auction %s (was PENDING since %s)", a.AuctionID, a.StartTime.Format(time.RFC3339))
			}
		}
	}
}
