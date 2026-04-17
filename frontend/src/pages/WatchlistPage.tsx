import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import type { Auction } from '@/types'
import { useAuth } from '@/context/AuthContext'
import { useWatchlist } from '@/context/WatchlistContext'
import { api } from '@/lib/api'
import { EmptyState, Spinner, Button } from '@/components/ui'
import { AuctionCard } from '@/components/auction'
import { PageContainer } from '@/components/layout'
import { HeartIcon } from '@/components/icons'

export default function WatchlistPage() {
  const { user, token } = useAuth()
  const { watchlist }   = useWatchlist()
  const navigate        = useNavigate()

  const [auctions, setAuctions] = useState<Auction[]>([])
  const [loading,  setLoading]  = useState(true)
  const [error,    setError]    = useState<string | null>(null)

  useEffect(() => {
    if (!user || !token) {
      navigate('/login')
      return
    }
    if (watchlist.size === 0) {
      setLoading(false)
      setAuctions([])
      return
    }

    setLoading(true)
    Promise.all([...watchlist].map((id) => api.auctions.get(id).catch(() => null)))
      .then((results) => setAuctions(results.filter((a): a is Auction => a !== null)))
      .catch(() => setError('Failed to load watchlist'))
      .finally(() => setLoading(false))
  }, [watchlist, user, token])

  if (!user) return null

  return (
    <PageContainer>
      <div className="mb-8 flex flex-col gap-4">
        <Button variant="ghost" className="self-start -ml-2" onClick={() => navigate(-1)}>
          ← Back
        </Button>
        <div className="flex items-center gap-3">
          <HeartIcon filled width={24} height={24} className="text-red-500" />
          <h1 className="font-display font-bold text-3xl text-text-primary">My Watchlist</h1>
        </div>
      </div>

      {loading ? (
        <Spinner className="py-32" />
      ) : error ? (
        <EmptyState message={error} action={<Button onClick={() => navigate('/')}>Browse Auctions</Button>} />
      ) : auctions.length === 0 ? (
        <EmptyState
          message="You haven't saved any auctions yet."
          action={<Button onClick={() => navigate('/')}>Browse Auctions</Button>}
        />
      ) : (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-6">
          {auctions.map((auction) => (
            <AuctionCard key={auction.auction_id} auction={auction} />
          ))}
        </div>
      )}
    </PageContainer>
  )
}
