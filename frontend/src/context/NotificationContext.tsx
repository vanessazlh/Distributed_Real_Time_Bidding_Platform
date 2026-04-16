import { createContext, useContext, useState, useEffect, useCallback, useRef } from 'react'
import type { ReactNode } from 'react'
import type { StoredNotification, UserNotificationEvent } from '@/types'
import { useAuth } from '@/context/AuthContext'
import { useUserWebSocket } from '@/hooks/useUserWebSocket'
import { api } from '@/lib/api'

interface NotificationContextValue {
  notifications: StoredNotification[]
  unreadCount:   number
  latestToast:   StoredNotification | null
  markAllRead:   () => void
  markOneRead:   (id: string) => void
  dismissToast:  () => void
}

const NotificationContext = createContext<NotificationContextValue | null>(null)

export function NotificationProvider({ children }: { children: ReactNode }) {
  const { token } = useAuth()
  const [notifications, setNotifications] = useState<StoredNotification[]>([])
  const [unreadCount, setUnreadCount]     = useState(0)
  const [latestToast, setLatestToast]     = useState<StoredNotification | null>(null)
  const toastTimer = useRef<ReturnType<typeof setTimeout>>()

  // Fetch stored notifications on login
  useEffect(() => {
    if (!token) {
      setNotifications([])
      setUnreadCount(0)
      setLatestToast(null)
      return
    }
    api.notifications.list(token).then((r) => {
      setNotifications(r.notifications ?? [])
      setUnreadCount(r.unread_count ?? 0)
    }).catch(() => { /* silent */ })
  }, [token])

  // Handle incoming WS notification
  const handleNotification = useCallback((event: UserNotificationEvent) => {
    const n = event.notification
    setNotifications((prev) => {
      // Dedup by ID — replace if exists, otherwise prepend
      const filtered = prev.filter((p) => p.id !== n.id)
      return [n, ...filtered].slice(0, 20)
    })
    setUnreadCount(event.unread_count)

    // Show toast
    setLatestToast(n)
    clearTimeout(toastTimer.current)
    toastTimer.current = setTimeout(() => setLatestToast(null), 5000)
  }, [])

  useUserWebSocket(token, handleNotification)

  const markAllRead = useCallback(() => {
    if (!token) return
    setNotifications((prev) => prev.map((n) => ({ ...n, read: true })))
    setUnreadCount(0)
    api.notifications.markAllRead(token).catch(() => { /* silent */ })
  }, [token])

  const markOneRead = useCallback((id: string) => {
    if (!token) return
    setNotifications((prev) => prev.map((n) => n.id === id ? { ...n, read: true } : n))
    setUnreadCount((prev) => Math.max(0, prev - 1))
    // Backend only supports mark-all; fire it so the server stays in sync
    api.notifications.markAllRead(token).catch(() => { /* silent */ })
  }, [token])

  const dismissToast = useCallback(() => {
    setLatestToast(null)
    clearTimeout(toastTimer.current)
  }, [])

  return (
    <NotificationContext.Provider value={{ notifications, unreadCount, latestToast, markAllRead, markOneRead, dismissToast }}>
      {children}
    </NotificationContext.Provider>
  )
}

export function useNotifications(): NotificationContextValue {
  const ctx = useContext(NotificationContext)
  if (!ctx) throw new Error('useNotifications must be used within <NotificationProvider>')
  return ctx
}
