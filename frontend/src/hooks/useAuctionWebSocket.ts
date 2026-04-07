import { useEffect, useRef } from 'react'
import type { BidPlacedEvent, AuctionClosedEvent } from '@/types'

interface WebSocketCallbacks {
  onBidPlaced?: (event: BidPlacedEvent) => void
  onAuctionClosed?: (event: AuctionClosedEvent) => void
}

/**
 * Connects to the notification service WebSocket for a given auction.
 * Route: GET /auctions/:auctionId/subscribe
 *
 * Calls the appropriate callback for bid_placed and auction_closed events.
 * Silently no-ops if the server is unavailable.
 */
export function useAuctionWebSocket(
  auctionId: string,
  callbacks: WebSocketCallbacks,
): void {
  const cbRef = useRef(callbacks)
  useEffect(() => { cbRef.current = callbacks })

  useEffect(() => {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const ws = new WebSocket(`${protocol}//${window.location.host}/auctions/${auctionId}/subscribe`)

    ws.onmessage = (e: MessageEvent<string>) => {
      try {
        const msg = JSON.parse(e.data)
        if (msg.type === 'bid_placed' && cbRef.current.onBidPlaced) {
          cbRef.current.onBidPlaced(msg as BidPlacedEvent)
        }
        if (msg.type === 'auction_closed' && cbRef.current.onAuctionClosed) {
          cbRef.current.onAuctionClosed(msg as AuctionClosedEvent)
        }
      } catch {
        // Ignore malformed messages
      }
    }

    // Silence connection errors — dev server may not be running
    ws.onerror = () => { /* noop */ }

    return () => ws.close()
  }, [auctionId])
}
