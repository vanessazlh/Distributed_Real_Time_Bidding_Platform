import { useState, useEffect } from 'react'
import { useNavigate, Link } from 'react-router-dom'
import type { Shop, Auction } from '@/types'
import { useAuth } from '@/context/AuthContext'
import { api } from '@/lib/api'
import { Card, Button, Spinner, EmptyState, StatCard } from '@/components/ui'
import { PageContainer } from '@/components/layout'
import { ArrowRightIcon } from '@/components/icons'

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

              return (
                <Card key={shop.shop_id} padding="p-8" className="flex flex-col gap-5">
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

                  {/* Quick stats */}
                  <div className="flex gap-6 text-sm text-text-secondary">
                    <span>{auctions.length} auction{auctions.length !== 1 ? 's' : ''}</span>
                    <span>{auctions.reduce((s, a) => s + a.bid_count, 0)} total bids</span>
                  </div>

                  {/* Manage link */}
                  <Link
                    to={`/seller/shops/${shop.shop_id}/items`}
                    className="mt-auto inline-flex items-center gap-2 font-sans font-semibold text-brand hover:text-brand-dark transition-colors text-base"
                  >
                    Manage <ArrowRightIcon />
                  </Link>
                </Card>
              )
            })}
          </div>
        </>
      )}
    </PageContainer>
  )
}
