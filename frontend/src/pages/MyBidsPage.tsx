import { useState, useEffect } from 'react'
import { Link } from 'react-router-dom'
import type { UserBid } from '@/types'
import { useAuth } from '@/context/AuthContext'
import { api } from '@/lib/api'
import { formatCurrency, timeAgo } from '@/lib/utils'
import { Card, Badge, StatCard, EmptyState, Button, Spinner } from '@/components/ui'
import { PageContainer } from '@/components/layout'
import { ChevronLeftIcon } from '@/components/icons'

export default function MyBidsPage() {
  const { user, token } = useAuth()

  const [bids,    setBids]    = useState<UserBid[]>([])
  const [loading, setLoading] = useState(true)
  const [error,   setError]   = useState<string | null>(null)

  useEffect(() => {
    if (!user || !token) { setLoading(false); return }
    api.users.bids(user.user_id, token)
      .then(setBids)
      .catch((err) => setError(err instanceof Error ? err.message : 'Failed to load bids'))
      .finally(() => setLoading(false))
  }, [user, token])

  if (!user) {
    return (
      <PageContainer narrow>
        <EmptyState
          message="Sign in to view your bids"
          action={<Button><Link to="/login">Sign In</Link></Button>}
        />
      </PageContainer>
    )
  }

  if (loading) {
    return (
      <PageContainer narrow>
        <Spinner className="py-20" />
      </PageContainer>
    )
  }

  if (error) {
    return (
      <PageContainer narrow>
        <EmptyState message={error} />
      </PageContainer>
    )
  }

  const stats = [
    { label: 'Active Bids', value: bids.filter((b) => b.status === 'WINNING').length },
    { label: 'Items Won',   value: bids.filter((b) => b.status === 'WON').length },
    {
      label: 'Total Spent',
      value: formatCurrency(
        bids.filter((b) => b.status !== 'OUTBID').reduce((s, b) => s + b.amount, 0)
      ),
    },
  ]

  return (
    <PageContainer narrow>
      <Link
        to="/"
        className="inline-flex items-center gap-1 text-text-secondary hover:text-brand text-sm font-medium transition-colors mb-8"
      >
        <ChevronLeftIcon /> All Auctions
      </Link>

      <h1 className="font-sans font-semibold text-3xl text-text-primary mb-8">My Bids</h1>

      {/* Stats strip */}
      <div className="flex gap-4 mb-8">
        {stats.map((s) => (
          <StatCard key={s.label} label={s.label} value={s.value} />
        ))}
      </div>

      {bids.length === 0 ? (
        <EmptyState message="You haven't placed any bids yet." />
      ) : (
        <Card>
          {bids.map((bid, i) => (
            <div
              key={bid.bid_id}
              className={`p-6 flex items-center justify-between ${i !== 0 ? 'border-t border-border' : ''}`}
            >
              <div>
                <p className="text-brand text-xs font-semibold mb-1">{bid.shop_name}</p>
                <p className="font-sans font-medium text-lg text-text-primary mb-1">{bid.item_title}</p>
                <p className="text-text-secondary text-sm">{timeAgo(bid.timestamp)}</p>
              </div>
              <div className="flex flex-col items-end gap-2">
                <p className="font-display text-2xl text-text-primary">{formatCurrency(bid.amount)}</p>
                <Badge status={bid.status} />
                {bid.status === 'OUTBID' && (
                  <Link
                    to={`/auction/${bid.auction_id}`}
                    className="text-brand text-xs font-medium hover:underline"
                  >
                    Bid Again →
                  </Link>
                )}
                {bid.status === 'WON' && (
                  <Link
                    to={`/payment/auction/${bid.auction_id}`}
                    className="text-brand text-xs font-medium hover:underline"
                  >
                    View Payment →
                  </Link>
                )}
              </div>
            </div>
          ))}
        </Card>
      )}
    </PageContainer>
  )
}
