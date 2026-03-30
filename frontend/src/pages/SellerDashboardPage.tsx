import { useState, useEffect } from 'react'
import { useNavigate, Link } from 'react-router-dom'
import type { Shop } from '@/types'
import { useAuth } from '@/context/AuthContext'
import { api } from '@/lib/api'
import { Card, Button, Spinner, EmptyState } from '@/components/ui'
import { PageContainer } from '@/components/layout'

export default function SellerDashboardPage() {
  const { user, token, isSeller } = useAuth()
  const navigate = useNavigate()

  const [shops,   setShops]   = useState<Shop[]>([])
  const [loading, setLoading] = useState(true)
  const [error,   setError]   = useState<string | null>(null)

  useEffect(() => {
    if (!user || !token) return
    api.shops.listByOwner(user.user_id, token)
      .then(setShops)
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
          <h2 className="font-sans font-semibold text-xl text-text-primary mb-6">
            Your Shops
          </h2>
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
            {shops.map((shop) => (
              <Card key={shop.shop_id} padding="p-6" className="flex flex-col gap-4">
                <div>
                  <h3 className="font-sans font-semibold text-lg text-text-primary">
                    {shop.name}
                  </h3>
                  <p className="text-text-secondary text-sm mt-0.5">{shop.location}</p>
                </div>

                <div className="flex flex-col gap-2 mt-auto">
                  <Link
                    to={`/shop/${shop.shop_id}`}
                    className="text-brand text-sm font-medium hover:underline"
                  >
                    View public page →
                  </Link>
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
            ))}
          </div>
        </>
      )}
    </PageContainer>
  )
}
