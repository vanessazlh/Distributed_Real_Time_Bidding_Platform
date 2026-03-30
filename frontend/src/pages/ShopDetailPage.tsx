import { useState, useEffect } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import type { Auction, Shop } from '@/types'
import { useAuth } from '@/context/AuthContext'
import { api } from '@/lib/api'
import { Avatar, Card, Button, EmptyState, Spinner } from '@/components/ui'
import { AuctionCard } from '@/components/auction'
import { PageContainer } from '@/components/layout'
import { ChevronLeftIcon } from '@/components/icons'
import { formatCurrency } from '@/lib/utils'

export default function ShopDetailPage() {
  const { id }          = useParams<{ id: string }>()
  const { user }        = useAuth()
  const navigate        = useNavigate()

  const [shop,     setShop]     = useState<Shop | null>(null)
  const [auctions, setAuctions] = useState<Auction[]>([])
  const [loading,  setLoading]  = useState(true)
  const [error,    setError]    = useState<string | null>(null)

  useEffect(() => {
    if (!id) return
    Promise.all([
      api.shops.get(id),
      api.auctions.list().then((all) => all.filter((a) => a.item.shop_id === id)),
    ])
      .then(([s, a]) => { setShop(s); setAuctions(a) })
      .catch((err) => setError(err instanceof Error ? err.message : 'Failed to load shop'))
      .finally(() => setLoading(false))
  }, [id])

  if (loading) {
    return <PageContainer><Spinner className="py-32" /></PageContainer>
  }

  if (error || !shop) {
    return (
      <PageContainer>
        <EmptyState
          message={error ?? 'Shop not found.'}
          action={<Button onClick={() => navigate('/')}>Back to Auctions</Button>}
        />
      </PageContainer>
    )
  }

  const isOwner = user?.user_id === shop.owner_id
  const open    = auctions.filter((a) => a.status === 'OPEN')
  const closed  = auctions.filter((a) => a.status === 'CLOSED')

  return (
    <PageContainer>
      <Link
        to="/"
        className="inline-flex items-center gap-1 text-text-secondary hover:text-brand text-sm font-medium transition-colors mb-8"
      >
        <ChevronLeftIcon /> All Auctions
      </Link>

      {/* Shop header */}
      <Card className="mb-10 flex flex-col items-center text-center" padding="p-10">
        <Avatar src={shop.logo_url} alt={shop.name} size="xl" />
        <h1 className="font-display text-4xl text-text-primary mt-4 mb-2">{shop.name}</h1>
        <p className="text-text-secondary mb-2">{shop.location}</p>
        <p className="text-text-secondary text-sm">Local shop selling surplus food at auction.</p>

        {isOwner && (
          <div className="flex gap-3 mt-6">
            <Button onClick={() => navigate(`/shops/${id}/items/new`)}>
              + Add Item
            </Button>
            <Button variant="primary" onClick={() => navigate(`/auctions/new?shopId=${id}`)}>
              + Publish Auction
            </Button>
          </div>
        )}
      </Card>

      {/* Active auctions */}
      {open.length > 0 && (
        <section className="mb-10">
          <h2 className="font-sans font-semibold text-2xl text-text-primary mb-6">Active Auctions</h2>
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
            {open.map((a) => (
              <AuctionCard key={a.auction_id} auction={a} />
            ))}
          </div>
        </section>
      )}

      {/* Past auctions */}
      {closed.length > 0 && (
        <section>
          <h2 className="font-sans font-semibold text-2xl text-text-primary mb-6">Past Auctions</h2>
          <Card>
            {closed.map((a, i) => (
              <div
                key={a.auction_id}
                className={`p-5 flex items-center justify-between ${i !== 0 ? 'border-t border-border' : ''}`}
              >
                <div>
                  <p className="font-sans font-medium text-text-primary">{a.item.title}</p>
                  <p className="text-text-secondary text-sm mt-0.5">{a.bid_count} bids</p>
                </div>
                <div className="text-right">
                  <p className="font-display text-lg text-text-secondary line-through">{formatCurrency(a.retail_price)}</p>
                  <p className="font-display text-lg text-text-primary">{formatCurrency(a.current_highest_bid)}</p>
                </div>
              </div>
            ))}
          </Card>
        </section>
      )}

      {open.length === 0 && closed.length === 0 && (
        <p className="text-text-secondary text-center py-10">No auctions yet for this shop.</p>
      )}
    </PageContainer>
  )
}
