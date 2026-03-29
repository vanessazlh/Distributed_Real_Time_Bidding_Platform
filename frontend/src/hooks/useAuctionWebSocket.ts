import { useEffect, useRef } from 'react'
import type { BidPlacedEvent } from '@/types'

/**
 * Connects to the notification service WebSocket for a given auction.
 * Route: GET /auctions/:auctionId/subscribe
 *
 * Calls onMessage for every valid bid_placed event received.
 * Silently no-ops if the server is unavailable (mock simulation handles the UI).
 */
export function useAuctionWebSocket(
  auctionId: string,
  onMessage: (event: BidPlacedEvent) => void,
): void {
  // Keep a stable ref to the callback so we don't reconnect on every render
  const onMessageRef = useRef(onMessage)
  useEffect(() => { onMessageRef.current = onMessage })

  useEffect(() => {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const ws = new WebSocket(`${protocol}//${window.location.host}/auctions/${auctionId}/subscribe`)

    ws.onmessage = (e: MessageEvent<string>) => {
      try {
        const msg = JSON.parse(e.data) as BidPlacedEvent
        if (msg.type === 'bid_placed') onMessageRef.current(msg)
      } catch {
        // Ignore malformed messages
      }
    }

    // Silence connection errors — dev server may not be running
    ws.onerror = () => { /* noop */ }

    return () => ws.close()
  }, [auctionId])
}
