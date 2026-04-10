import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import type { Auction } from '@/types'
import { CATEGORIES } from '@/types'
import { api } from '@/lib/api'
import { getPickupHour } from '@/lib/utils'
import { useAuth } from '@/context/AuthContext'
import { AuctionCard } from '@/components/auction'
import { PageContainer } from '@/components/layout'
import { Spinner, EmptyState } from '@/components/ui'

const TABS = ['All', ...CATEGORIES] as const
type TabFilter = typeof TABS[number]

const PICKUP_FILTERS = ['Any Time', 'Morning', 'Afternoon', 'Evening'] as const
type PickupFilter = typeof PICKUP_FILTERS[number]

export default function HomePage() {
  const { isSeller } = useAuth()
  const navigate = useNavigate()
  const [auctions, setAuctions] = useState<Auction[]>([])
  const [loading,  setLoading]  = useState(true)
  const [error,    setError]    = useState<string | null>(null)
  const [filter,       setFilter]       = useState<TabFilter>('All')
  const [pickupFilter, setPickupFilter] = useState<PickupFilter>('Any Time')

  useEffect(() => {
    if (isSeller) { navigate('/seller/dashboard', { replace: true }); return }
    api.auctions.list()
      .then(setAuctions)
      .catch((err) => setError(err instanceof Error ? err.message : 'Failed to load auctions'))
      .finally(() => setLoading(false))
  }, [isSeller, navigate])

  const byCategory = filter === 'All'
    ? auctions
    : auctions.filter((a) => a.category === filter)

  const visible = pickupFilter === 'Any Time'
    ? byCategory
    : byCategory.filter((a) => {
        if (!a.pickup_start) return false
        const hour = getPickupHour(a.pickup_start)
        if (pickupFilter === 'Morning')   return hour < 12
        if (pickupFilter === 'Afternoon') return hour >= 12 && hour < 17
        return hour >= 17 // Evening
      })

  return (
    <PageContainer>
      {/* Hero */}
      <div className="py-12 text-center max-w-2xl mx-auto">
        <h1 className="font-sans font-semibold text-4xl text-text-primary mb-4">
          Rescue today's surplus.<br />5 minutes to bid.
        </h1>
        <p className="text-text-secondary text-lg">
          Premium unsold goods from local shops, auctioned at deep discounts to prevent food waste.
        </p>
      </div>

      {/* Category tabs */}
      <div className="flex justify-center gap-8 border-b border-border">
        {TABS.map((tab) => (
          <button
            key={tab}
            onClick={() => setFilter(tab)}
            className={[
              'pb-3 font-sans font-medium text-lg border-b-2 transition-colors',
              filter === tab
                ? 'border-brand text-brand'
                : 'border-transparent text-text-secondary hover:text-text-primary',
            ].join(' ')}
          >
            {tab}
          </button>
        ))}
      </div>

      {/* Pickup time filter */}
      <div className="flex justify-center gap-3 mt-4 mb-8">
        {PICKUP_FILTERS.map((pf) => (
          <button
            key={pf}
            onClick={() => setPickupFilter(pf)}
            className={[
              'px-4 py-1.5 rounded-full text-sm font-medium transition-colors',
              pickupFilter === pf
                ? 'bg-brand text-white'
                : 'bg-surface text-text-secondary hover:bg-surface-alt hover:text-text-primary',
            ].join(' ')}
          >
            {pf}
          </button>
        ))}
      </div>

      {loading && <Spinner className="py-20" />}

      {!loading && error && (
        <EmptyState message={error} />
      )}

      {!loading && !error && visible.length === 0 && (
        <EmptyState message="No auctions in this category right now." />
      )}

      {!loading && !error && visible.length > 0 && (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {visible.map((auction) => (
            <AuctionCard key={auction.auction_id} auction={auction} />
          ))}
        </div>
      )}
    </PageContainer>
  )
}
