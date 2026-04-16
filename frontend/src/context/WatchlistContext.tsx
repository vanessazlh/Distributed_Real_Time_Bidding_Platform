import { createContext, useContext, useState, useEffect, useCallback } from 'react'
import type { ReactNode } from 'react'
import { useAuth } from '@/context/AuthContext'
import { api } from '@/lib/api'

interface WatchlistContextValue {
  watchlist: Set<string>
  toggle: (auctionId: string) => Promise<void>
  isWatched: (auctionId: string) => boolean
}

const WatchlistContext = createContext<WatchlistContextValue | null>(null)

export function WatchlistProvider({ children }: { children: ReactNode }) {
  const { user, token } = useAuth()
  const [watchlist, setWatchlist] = useState<Set<string>>(new Set())

  // Load watchlist when the user logs in
  useEffect(() => {
    if (!user || !token) {
      setWatchlist(new Set())
      return
    }
    api.users.getWatchlist(user.user_id, token)
      .then((ids) => setWatchlist(new Set(ids)))
      .catch(() => { /* silently ignore — non-critical */ })
  }, [user?.user_id, token])

  const toggle = useCallback(async (auctionId: string) => {
    if (!user || !token) return
    const isCurrentlyWatched = watchlist.has(auctionId)
    // Optimistic update
    setWatchlist((prev) => {
      const next = new Set(prev)
      if (isCurrentlyWatched) {
        next.delete(auctionId)
      } else {
        next.add(auctionId)
      }
      return next
    })
    try {
      if (isCurrentlyWatched) {
        await api.users.removeFromWatchlist(user.user_id, auctionId, token)
      } else {
        await api.users.addToWatchlist(user.user_id, auctionId, token)
      }
    } catch {
      // Roll back on failure
      setWatchlist((prev) => {
        const next = new Set(prev)
        if (isCurrentlyWatched) {
          next.add(auctionId)
        } else {
          next.delete(auctionId)
        }
        return next
      })
    }
  }, [user, token, watchlist])

  const isWatched = useCallback((auctionId: string) => watchlist.has(auctionId), [watchlist])

  return (
    <WatchlistContext.Provider value={{ watchlist, toggle, isWatched }}>
      {children}
    </WatchlistContext.Provider>
  )
}

export function useWatchlist(): WatchlistContextValue {
  const ctx = useContext(WatchlistContext)
  if (!ctx) throw new Error('useWatchlist must be used within <WatchlistProvider>')
  return ctx
}
