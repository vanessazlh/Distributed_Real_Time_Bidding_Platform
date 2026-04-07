import { useState, useEffect } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import type { Auction, Shop } from '@/types'
import { useAuth } from '@/context/AuthContext'
import { api } from '@/lib/api'
import { Card, Badge, Button, Spinner, EmptyState, StatCard } from '@/components/ui'
import { CountdownTimer } from '@/components/auction'
import { PageContainer } from '@/components/layout'
import { ChevronLeftIcon } from '@/components/icons'
import { formatCurrency } from '@/lib/utils'

export default function SellerAuctionPage() {
  const { shopId } = useParams<{ shopId: string }>()
  const { user, token, isSeller } = useAuth()
  const navigate = useNavigate()

  const [shop,     setShop]     = useState<Shop | null>(null)
  const [auctions, setAuctions] = useState<Auction[]>([])
  const [loading,  setLoading]  = useState(true)
  const [error,    setError]    = useState<string | null>(null)
  const [closing,  setClosing]  = useState<string | null>(null)

  useEffect(() => {
    if (!shopId || !token) return
    Promise.all([
      api.shops.get(shopId),
      api.auctions.listByShop(shopId, token),
    ])
      .then(([s, a]) => { setShop(s); setAuctions(a) })
      .catch((err) => setError(err instanceof Error ? err.message : 'Failed to load auctions'))
      .finally(() => setLoading(false))
  }, [shopId, token])

  const handleClose = async (auctionId: string) => {
    if (!token) return
    setClosing(auctionId)
    try {
      await api.auctions.close(auctionId, token)
      setAuctions((prev) =>
        prev.map((a) =>
          a.auction_id === auctionId ? { ...a, status: 'CLOSED' as const } : a
        )
      )
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Failed to close auction')
    } finally {
      setClosing(null)
    }
  }

  if (!user || !isSeller) {
    return (
      <PageContainer narrow>
        <EmptyState
          message="Sign in as a seller to manage auctions"
          action={<Button onClick={() => navigate('/shop/login')}>Seller Sign In</Button>}
        />
      </PageContainer>
    )
  }

  if (loading) {
    return <PageContainer><Spinner className="py-32" /></PageContainer>
  }

  if (error || !shop) {
    return (
      <PageContainer>
        <EmptyState
          message={error ?? 'Shop not found.'}
          action={<Button onClick={() => navigate('/seller/dashboard')}>Back to Dashboard</Button>}
        />
      </PageContainer>
    )
  }

  const open   = auctions.filter((a) => a.status === 'OPEN')
  const closed = auctions.filter((a) => a.status === 'CLOSED')
  const totalBids = auctions.reduce((sum, a) => sum + a.bid_count, 0)
  const totalRevenue = closed.reduce((sum, a) => sum + a.current_highest_bid, 0)

  return (
    <PageContainer>
      {/* Back link */}
      <Link
        to="/seller/dashboard"
        className="inline-flex items-center gap-1 text-text-secondary hover:text-brand text-sm font-medium transition-colors mb-8"
      >
        <ChevronLeftIcon /> Back to Dashboard
      </Link>

      {/* Shop header */}
      <div className="py-6 flex items-end justify-between border-b border-border mb-8">
        <div>
          <p className="text-text-secondary text-sm mb-1">{shop.name}</p>
          <h1 className="font-display text-3xl text-text-primary">Auction Management</h1>
        </div>
        <Button variant="primary" onClick={() => navigate(`/auctions/new?shopId=${shopId}`)}>
          + Publish Auction
        </Button>
      </div>

      {/* Stats */}
      <div className="flex gap-4 mb-8">
        <StatCard label="Active" value={open.length} />
        <StatCard label="Closed" value={closed.length} />
        <StatCard label="Total Bids" value={totalBids} />
        <StatCard label="Revenue" value={formatCurrency(totalRevenue)} />
      </div>

      {auctions.length === 0 && (
        <EmptyState
          message="No auctions yet for this shop."
          action={
            <Button variant="primary" onClick={() => navigate(`/auctions/new?shopId=${shopId}`)}>
              Publish Your First Auction
            </Button>
          }
        />
      )}

      {/* Active auctions */}
      {open.length > 0 && (
        <section className="mb-10">
          <h2 className="font-sans font-semibold text-xl text-text-primary mb-4">
            Active Auctions
          </h2>
          <Card>
            {open.map((a, i) => (
              <div
                key={a.auction_id}
                className={`p-5 flex items-center justify-between gap-4 ${i !== 0 ? 'border-t border-border' : ''}`}
              >
                <div className="flex items-center gap-4 min-w-0 flex-1">
                  {a.image_url && (
                    <img
                      src={a.image_url}
                      alt={a.item.title}
                      className="w-12 h-12 rounded-lg object-cover shrink-0"
                    />
                  )}
                  <div className="min-w-0">
                    <div className="flex items-center gap-2 mb-1">
                      <Badge status={a.status} />
                      <Link
                        to={`/auction/${a.auction_id}`}
                        className="font-sans font-medium text-text-primary hover:text-brand truncate"
                      >
                        {a.item.title}
                      </Link>
                    </div>
                    <div className="flex items-center gap-3 text-text-secondary text-xs">
                      <span>{a.bid_count} bid{a.bid_count !== 1 ? 's' : ''}</span>
                      <CountdownTimer endTime={a.end_time} />
                    </div>
                  </div>
                </div>

                <div className="flex items-center gap-4 shrink-0">
                  <div className="text-right">
                    <p className="font-serif text-lg text-text-primary">
                      {formatCurrency(a.current_highest_bid)}
                    </p>
                    <p className="text-text-secondary text-xs">current bid</p>
                  </div>
                  <Button
                    size="sm"
                    variant="outline"
                    disabled={closing === a.auction_id}
                    onClick={() => handleClose(a.auction_id)}
                  >
                    {closing === a.auction_id ? 'Closing...' : 'Close'}
                  </Button>
                </div>
              </div>
            ))}
          </Card>
        </section>
      )}

      {/* Closed auctions */}
      {closed.length > 0 && (
        <section>
          <h2 className="font-sans font-semibold text-xl text-text-primary mb-4">
            Closed Auctions
          </h2>
          <Card>
            {closed.map((a, i) => (
              <div
                key={a.auction_id}
                className={`p-5 flex items-center justify-between gap-4 ${i !== 0 ? 'border-t border-border' : ''}`}
              >
                <div className="flex items-center gap-4 min-w-0 flex-1">
                  {a.image_url && (
                    <img
                      src={a.image_url}
                      alt={a.item.title}
                      className="w-12 h-12 rounded-lg object-cover shrink-0 opacity-60"
                    />
                  )}
                  <div className="min-w-0">
                    <div className="flex items-center gap-2 mb-1">
                      <Badge status={a.status} />
                      <Link
                        to={`/auction/${a.auction_id}`}
                        className="font-sans font-medium text-text-secondary hover:text-brand truncate"
                      >
                        {a.item.title}
                      </Link>
                    </div>
                    <p className="text-text-secondary text-xs">
                      {a.bid_count} bid{a.bid_count !== 1 ? 's' : ''}
                    </p>
                  </div>
                </div>

                <div className="text-right shrink-0">
                  <p className="font-display text-lg text-text-secondary line-through">
                    {formatCurrency(a.retail_price)}
                  </p>
                  <p className="font-serif text-lg text-text-primary">
                    {formatCurrency(a.current_highest_bid)}
                  </p>
                </div>
              </div>
            ))}
          </Card>
        </section>
      )}
    </PageContainer>
  )
}
