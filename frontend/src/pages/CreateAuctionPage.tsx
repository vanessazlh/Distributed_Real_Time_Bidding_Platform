import { useState, useEffect } from 'react'
import type { FormEvent } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import type { Item } from '@/types'
import { useAuth } from '@/context/AuthContext'
import { api } from '@/lib/api'
import { Card, Button, FormField, TextInput, StatusBanner, EmptyState, Spinner } from '@/components/ui'
import { PageContainer } from '@/components/layout'
import { ChevronLeftIcon } from '@/components/icons'
import { formatCurrency } from '@/lib/utils'

export default function CreateAuctionPage() {
  const [searchParams]  = useSearchParams()
  const shopId          = searchParams.get('shopId') ?? ''
  const { user, token, isSeller } = useAuth()
  const navigate        = useNavigate()

  const [items,       setItems]       = useState<Item[]>([])
  const [loadingItems,setLoadingItems]= useState(true)
  const [itemId,      setItemId]      = useState('')
  const [duration,    setDuration]    = useState('5')
  const [startBid,    setStartBid]    = useState('')
  const [loading,     setLoading]     = useState(false)
  const [error,       setError]       = useState<string | null>(null)

  useEffect(() => {
    if (!shopId) { setLoadingItems(false); return }
    api.shops.items(shopId)
      .then(setItems)
      .catch(() => setItems([]))
      .finally(() => setLoadingItems(false))
  }, [shopId])

  if (!user || !isSeller) {
    return (
      <PageContainer narrow>
        <EmptyState
          message="Sign in as a seller to publish an auction"
          action={<Button onClick={() => navigate('/shop/login')}>Seller Sign In</Button>}
        />
      </PageContainer>
    )
  }

  if (!shopId) {
    return (
      <PageContainer narrow>
        <EmptyState
          message="No shop selected."
          action={<Button onClick={() => navigate('/')}>Go Home</Button>}
        />
      </PageContainer>
    )
  }

  const selectedItem = items.find((i) => i.item_id === itemId)

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    if (!selectedItem) return
    setError(null)
    setLoading(true)
    try {
      const auction = await api.auctions.create(
        {
          item_id:          selectedItem.item_id,
          item_title:       selectedItem.title,
          shop_id:          shopId,
          duration_minutes: parseInt(duration, 10),
          start_bid:        Math.round(parseFloat(startBid) * 100),
        },
        token!,
      )
      navigate(`/auction/${auction.auction_id}`)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Something went wrong')
    } finally {
      setLoading(false)
    }
  }

  return (
    <PageContainer narrow>
      <Link
        to={`/shop/${shopId}`}
        className="inline-flex items-center gap-1 text-text-secondary hover:text-brand text-sm font-medium transition-colors mb-8"
      >
        <ChevronLeftIcon /> Back to Shop
      </Link>

      <Card padding="p-8">
        <h1 className="font-display text-3xl text-text-primary mb-2">Publish Auction</h1>
        <p className="text-text-secondary text-sm mb-8">
          Choose an item and set the auction duration and starting bid.
        </p>

        {error && (
          <div className="mb-4">
            <StatusBanner type="error" message={error} />
          </div>
        )}

        {loadingItems ? (
          <Spinner className="py-10" />
        ) : items.length === 0 ? (
          <div className="text-center py-8">
            <p className="text-text-secondary mb-4">No items in your shop yet.</p>
            <Button onClick={() => navigate(`/shops/${shopId}/items/new`)}>
              Add an Item First
            </Button>
          </div>
        ) : (
          <form onSubmit={handleSubmit} className="flex flex-col gap-4">
            <FormField label="Select Item">
              <select
                required
                value={itemId}
                onChange={(e) => setItemId(e.target.value)}
                className="w-full bg-surface-alt border-2 border-border rounded-lg py-3 px-4 font-sans text-text-primary focus:outline-none focus:border-brand focus:ring-1 focus:ring-brand transition-all"
              >
                <option value="">— choose an item —</option>
                {items.map((item) => (
                  <option key={item.item_id} value={item.item_id}>
                    {item.title} (retail {formatCurrency(item.retail_value)})
                  </option>
                ))}
              </select>
            </FormField>

            <FormField label="Duration (minutes)">
              <TextInput
                type="number"
                required
                min="1"
                max="1440"
                placeholder="5"
                value={duration}
                onChange={(e) => setDuration(e.target.value)}
              />
            </FormField>

            <FormField label="Starting Bid ($)">
              <TextInput
                type="number"
                required
                min="0.01"
                step="0.01"
                placeholder="1.00"
                value={startBid}
                onChange={(e) => setStartBid(e.target.value)}
              />
            </FormField>

            <Button
              variant="primary"
              size="lg"
              type="submit"
              fullWidth
              disabled={loading || !itemId}
              className="mt-2"
            >
              {loading ? 'Publishing…' : 'Publish Auction'}
            </Button>
          </form>
        )}
      </Card>
    </PageContainer>
  )
}
