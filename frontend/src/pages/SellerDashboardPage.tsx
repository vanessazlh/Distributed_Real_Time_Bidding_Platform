import { useState, useEffect } from 'react'
import { useNavigate, Link } from 'react-router-dom'
import type { Shop, Auction } from '@/types'
import { useAuth } from '@/context/AuthContext'
import { api } from '@/lib/api'
import { Card, Badge, Button, Spinner, EmptyState, StatCard } from '@/components/ui'
import { PageContainer } from '@/components/layout'
import { formatCurrency } from '@/lib/utils'

const INLINE_LIMIT = 3

export default function SellerDashboardPage() {
  const { user, token, isSeller } = useAuth()
  const navigate = useNavigate()

  const [shops,   setShops]   = useState<Shop[]>([])
  const [shopAuctions, setShopAuctions] = useState<Record<string, Auction[]>>({})
  const [loading, setLoading] = useState(true)
  const [error,   setError]   = useState<string | null>(null)

  useEffect(() => {
    if (!user || !token) return
    api.shops.listByOwner(user.user_id, token)
      .then(async (shops) => {
        setShops(shops)
        const auctionMap: Record<string, Auction[]> = {}
        await Promise.all(
          shops.map((shop) =>
            api.auctions.listByShop(shop.shop_id, token)
              .then((auctions) => { auctionMap[shop.shop_id] = auctions })
              .catch(() => { auctionMap[shop.shop_id] = [] })
          )
        )
        setShopAuctions(auctionMap)
      })
      .catch((err) => setError(err instanceof Error ? err.message : 'Failed to load shops'))
      .finally(() => setLoading(false))
  }, [user, token])

  if (!user || !isSeller) {
    return (
      <PageContainer narrow>
        <EmptyState
          message="Sign in as a seller to view your dashboard"
          action={<Button onClick={() => navigate('/shop/login')}>Seller Sign In</Button>}
        />
      </PageContainer>
    )
  }

  // Aggregate stats across all shops
  const allAuctions = Object.values(shopAuctions).flat()
  const totalOpen   = allAuctions.filter((a) => a.status === 'OPEN').length
  const totalClosed = allAuctions.filter((a) => a.status === 'CLOSED').length
  const totalBids   = allAuctions.reduce((sum, a) => sum + a.bid_count, 0)

  return (
    <PageContainer>
      {/* Header */}
      <div className="py-10 flex items-end justify-between border-b border-border mb-10">
        <div>
          <p className="text-text-secondary text-sm mb-1">Seller dashboard</p>
          <h1 className="font-display text-4xl text-text-primary">
            Welcome back, {user.username}
          </h1>
        </div>
        <Button variant="primary" onClick={() => navigate('/shops/new')}>
          + Register New Shop
        </Button>
      </div>

      {loading && <Spinner className="py-20" />}

      {!loading && error && <EmptyState message={error} />}

      {!loading && !error && shops.length === 0 && (
        <EmptyState
          message="You don't have any shops yet."
          action={
            <Button variant="primary" onClick={() => navigate('/shops/new')}>
              Register Your First Shop
            </Button>
          }
        />
      )}

      {!loading && !error && shops.length > 0 && (
        <>
          {/* Stats overview */}
          <div className="flex gap-4 mb-10">
            <StatCard label="Shops" value={shops.length} />
            <StatCard label="Active Auctions" value={totalOpen} />
            <StatCard label="Closed Auctions" value={totalClosed} />
            <StatCard label="Total Bids" value={totalBids} />
          </div>

          <h2 className="font-sans font-semibold text-xl text-text-primary mb-6">
            Your Shops
          </h2>
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
            {shops.map((shop) => {
              const auctions = shopAuctions[shop.shop_id] ?? []
              const openCount = auctions.filter((a) => a.status === 'OPEN').length
              const preview = auctions.slice(0, INLINE_LIMIT)

              return (
                <Card key={shop.shop_id} padding="p-6" className="flex flex-col gap-4">
                  {/* Shop header */}
                  <div className="flex items-start justify-between">
                    <div>
                      <h3 className="font-sans font-semibold text-lg text-text-primary">
                        {shop.name}
                      </h3>
                      <p className="text-text-secondary text-sm mt-0.5">{shop.location}</p>
                    </div>
                    <span className="text-text-secondary text-sm">
                      {openCount} active
                    </span>
                  </div>

                  {/* Inline auction list */}
                  {preview.length > 0 ? (
                    <div className="border border-border rounded-lg divide-y divide-border">
                      {preview.map((a) => (
                        <div key={a.auction_id} className="p-3 flex items-center justify-between gap-3">
                          <div className="flex items-center gap-3 min-w-0">
                            <Badge status={a.status} />
                            <span className="font-sans text-sm text-text-primary truncate">
                              {a.item.title}
                            </span>
                          </div>
                          <div className="text-right shrink-0">
                            <p className="font-serif text-sm text-text-primary">
                              {formatCurrency(a.current_highest_bid)}
                            </p>
                            <p className="text-text-secondary text-xs">
                              {a.bid_count} bid{a.bid_count !== 1 ? 's' : ''}
                            </p>
                          </div>
                        </div>
                      ))}
                    </div>
                  ) : (
                    <p className="text-text-secondary text-sm py-2">No auctions yet.</p>
                  )}

                  {/* Actions */}
                  <div className="flex flex-col gap-2 mt-auto">
                    <div className="flex items-center justify-between">
                      <Link
                        to={`/shop/${shop.shop_id}`}
                        className="text-brand text-sm font-medium hover:underline"
                      >
                        View public page →
                      </Link>
                      {auctions.length > INLINE_LIMIT && (
                        <Link
                          to={`/seller/shops/${shop.shop_id}/auctions`}
                          className="text-brand text-sm font-medium hover:underline"
                        >
                          View all {auctions.length} auctions →
                        </Link>
                      )}
                      {auctions.length > 0 && auctions.length <= INLINE_LIMIT && (
                        <Link
                          to={`/seller/shops/${shop.shop_id}/auctions`}
                          className="text-brand text-sm font-medium hover:underline"
                        >
                          Manage auctions →
                        </Link>
                      )}
                    </div>
                    <div className="flex gap-2">
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() => navigate(`/shops/${shop.shop_id}/items/new`)}
                      >
                        + Add Item
                      </Button>
                      <Button
                        size="sm"
                        variant="primary"
                        onClick={() => navigate(`/auctions/new?shopId=${shop.shop_id}`)}
                      >
                        + Publish Auction
                      </Button>
                    </div>
                  </div>
                </Card>
              )
            })}
          </div>
        </>
      )}
    </PageContainer>
  )
}
