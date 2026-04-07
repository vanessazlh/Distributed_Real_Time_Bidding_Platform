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
      <div className="pb-8 flex items-end justify-between border-b border-border mb-10">
        <div>
          <p className="text-text-secondary text-base mb-1">Seller dashboard</p>
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
                <Card key={shop.shop_id} padding="p-8" className="flex flex-col gap-6">
                  {/* Shop header */}
                  <div className="flex items-start justify-between">
                    <div>
                      <h3 className="font-sans font-semibold text-xl text-text-primary">
                        {shop.name}
                      </h3>
                      <p className="text-text-secondary text-base mt-1">{shop.location}</p>
                    </div>
                    <span className="text-text-secondary text-base">
                      {openCount} active
                    </span>
                  </div>

                  {/* Inline auction list */}
                  {preview.length > 0 ? (
                    <div className="border border-border rounded-lg divide-y divide-border">
                      {preview.map((a) => (
                        <div key={a.auction_id} className="px-4 py-3 flex items-center justify-between gap-4">
                          <div className="flex items-center gap-3 min-w-0">
                            <Badge status={a.status} />
                            <span className="font-sans text-base text-text-primary truncate">
                              {a.item.title}
                            </span>
                          </div>
                          <div className="text-right shrink-0">
                            <p className="font-serif text-base text-text-primary">
                              {formatCurrency(a.current_highest_bid)}
                            </p>
                            <p className="text-text-secondary text-base">
                              {a.bid_count} bid{a.bid_count !== 1 ? 's' : ''}
                            </p>
                          </div>
                        </div>
                      ))}
                    </div>
                  ) : (
                    <div className="flex flex-col items-center justify-center gap-2 py-6 rounded-lg border border-dashed border-border bg-surface-secondary">
                      <span className="text-2xl">🏷️</span>
                      <p className="text-text-secondary text-sm font-medium">No auctions yet</p>
                      <p className="text-text-secondary text-xs">Publish an auction to get started</p>
                    </div>
                  )}

                  {/* Actions */}
                  <div className="flex flex-col gap-3 mt-auto">
                    <Button
                      size="md"
                      variant="primary"
                      onClick={() => navigate(`/seller/shops/${shop.shop_id}/auctions`)}
                      className="w-full"
                    >
                      Manage Auctions {auctions.length > 0 ? `(${auctions.length})` : ''}
                    </Button>
                    <div className="flex gap-4 justify-center">
                      <Button
                        size="md"
                        variant="outline"
                        onClick={() => navigate(`/shops/${shop.shop_id}/items/new`)}
                      >
                        + Add Item
                      </Button>
                      <Button
                        size="md"
                        variant="outline"
                        onClick={() => navigate(`/auctions/new?shopId=${shop.shop_id}`)}
                      >
                        + Publish Auction
                      </Button>
                    </div>
                    <div className="flex justify-center pt-1">
                      <Link
                        to={`/shop/${shop.shop_id}`}
                        className="text-brand text-base font-medium hover:underline"
                      >
                        View public page
                      </Link>
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
