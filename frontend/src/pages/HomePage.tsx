import { useState, useEffect } from 'react'
import type { Auction } from '@/types'
import { api } from '@/lib/api'
import { AuctionCard } from '@/components/auction'
import { PageContainer } from '@/components/layout'
import { Spinner, EmptyState } from '@/components/ui'

const TABS = ['All', 'Bakery', 'Sushi', 'Other'] as const

function matchesFilter(auction: Auction, filter: string): boolean {
  if (filter === 'All') return true
  const haystack = `${auction.item.title} ${auction.item.shop_name}`.toLowerCase()
  const isBakery  = /bakery|bread|pastry|cake|croissant|sourdough/i.test(haystack)
  const isSushi   = /sushi|bento|nigiri|maki|roll/i.test(haystack)
  if (filter === 'Bakery') return isBakery
  if (filter === 'Sushi')  return isSushi
  if (filter === 'Other')  return !isBakery && !isSushi
  return true
}

export default function HomePage() {
  const [auctions, setAuctions] = useState<Auction[]>([])
  const [loading,  setLoading]  = useState(true)
  const [error,    setError]    = useState<string | null>(null)
  const [filter,   setFilter]   = useState<string>('All')

  useEffect(() => {
    api.auctions.list()
      .then(setAuctions)
      .catch((err) => setError(err instanceof Error ? err.message : 'Failed to load auctions'))
      .finally(() => setLoading(false))
  }, [])

  const visible = auctions.filter((a) => matchesFilter(a, filter))

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
      <div className="flex justify-center gap-8 mb-8 border-b border-border">
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
