import { useEffect, useRef } from 'react'
import type { UserNotificationEvent } from '@/types'

/**
 * Connects to the global user-level WebSocket for notifications.
 * Route: GET /notifications/subscribe?token=<jwt>
 *
 * Reconnects with exponential backoff (1s → 2s → 4s … → 30s max).
 * Silently no-ops when token is null (logged out).
 */
export function useUserWebSocket(
  token: string | null,
  onNotification: (event: UserNotificationEvent) => void,
): void {
  const cbRef = useRef(onNotification)
  useEffect(() => { cbRef.current = onNotification })

  useEffect(() => {
    if (!token) return

    let ws: WebSocket | null = null
    let retryDelay = 1000
    let unmounted = false

    function connect() {
      if (unmounted) return
      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
      ws = new WebSocket(`${protocol}//${window.location.host}/notifications/subscribe?token=${encodeURIComponent(token!)}`)

      ws.onmessage = (e: MessageEvent<string>) => {
        try {
          const msg = JSON.parse(e.data)
          if (msg.type === 'notification') {
            cbRef.current(msg as UserNotificationEvent)
          }
        } catch {
          // Ignore malformed messages
        }
      }

      ws.onopen = () => {
        retryDelay = 1000 // reset on successful connection
      }

      ws.onclose = () => {
        if (unmounted) return
        setTimeout(connect, retryDelay)
        retryDelay = Math.min(retryDelay * 2, 30000)
      }

      ws.onerror = () => { /* onclose will fire after this */ }
    }

    connect()

    return () => {
      unmounted = true
      ws?.close()
    }
  }, [token])
}
