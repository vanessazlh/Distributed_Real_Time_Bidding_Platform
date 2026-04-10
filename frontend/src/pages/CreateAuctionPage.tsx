import { useState, useEffect, useRef } from 'react'
import type { FormEvent } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import type { Item, Shop } from '@/types'
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
  const [shop,        setShop]        = useState<Shop | null>(null)
  const [loadingItems,setLoadingItems]= useState(true)
  const [itemId,      setItemId]      = useState('')
  const [duration,      setDuration]      = useState('5')
  const [startBid,      setStartBid]     = useState('')
  const [maxPrice,      setMaxPrice]     = useState('')
  const [quantity,      setQuantity]     = useState('1')
  const [scheduledStart,setScheduledStart] = useState('')
  const [pickupStart,   setPickupStart]   = useState('')
  const [pickupEnd,     setPickupEnd]     = useState('')
  const [loading,       setLoading]      = useState(false)
  const [error,         setError]        = useState<string | null>(null)
  const [itemOpen,      setItemOpen]     = useState(false)
  const itemRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (itemRef.current && !itemRef.current.contains(e.target as Node)) setItemOpen(false)
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [])

  useEffect(() => {
    if (!shopId) { setLoadingItems(false); return }
    Promise.all([
      api.shops.items(shopId).catch(() => [] as Item[]),
      api.shops.get(shopId).catch(() => null),
    ]).then(([fetchedItems, fetchedShop]) => {
      setItems(fetchedItems)
      setShop(fetchedShop)
    }).finally(() => setLoadingItems(false))
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
      const payload: Parameters<typeof api.auctions.create>[0] = {
        item_id:          selectedItem.item_id,
        item_title:       selectedItem.title,
        shop_id:          shopId,
        shop_name:        shop?.name             ?? '',
        retail_price:     selectedItem.retail_value,
        image_url:        selectedItem.image_url ?? '',
        shop_logo_url:    shop?.logo_url         ?? '',
        description:      selectedItem.description ?? '',
        category:         selectedItem.category  ?? undefined,
        duration_minutes: parseInt(duration, 10),
        start_bid:        Math.round(parseFloat(startBid) * 100),
        max_price:        maxPrice ? Math.round(parseFloat(maxPrice) * 100) : undefined,
        quantity:          parseInt(quantity, 10) > 1 ? parseInt(quantity, 10) : undefined,
      }
      if (scheduledStart) {
        payload.scheduled_start = new Date(scheduledStart).toISOString()
      }
      payload.pickup_start = new Date(pickupStart).toISOString()
      payload.pickup_end = new Date(pickupEnd).toISOString()
      await api.auctions.create(payload, token!)
      navigate(`/seller/shops/${shopId}/auctions`)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Something went wrong')
    } finally {
      setLoading(false)
    }
  }

  return (
    <PageContainer narrow>
      <Link
        to="/seller/dashboard"
        className="inline-flex items-center gap-1 text-text-secondary hover:text-brand text-base font-medium transition-colors mb-8"
      >
        <ChevronLeftIcon /> Back to Dashboard
      </Link>

      <Card padding="p-8">
        <h1 className="font-display text-3xl text-text-primary mb-2">Publish Auction</h1>
        <p className="text-text-secondary text-base mb-8">
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
          <div className="text-center py-8 w-full">
            <p className="text-text-secondary mb-4">No items in your shop yet.</p>
            <div className="flex justify-center">
              <Button onClick={() => navigate(`/shops/${shopId}/items/new`)}>
                Add an Item First
              </Button>
            </div>
          </div>
        ) : (
          <form onSubmit={handleSubmit} className="flex flex-col gap-4">
            <FormField label="Select Item">
              <div ref={itemRef} className="relative">
                <button
                  type="button"
                  onClick={() => setItemOpen((o) => !o)}
                  className={`w-full flex items-center justify-between px-4 py-3 rounded-xl border-2 font-sans text-base transition-all bg-white ${itemOpen ? 'border-brand ring-2 ring-brand/20' : 'border-border hover:border-brand/50'} ${itemId ? 'text-text-primary' : 'text-text-secondary'}`}
                >
                  <span>{selectedItem ? `${selectedItem.title} (retail ${formatCurrency(selectedItem.retail_value)})` : '— choose an item —'}</span>
                  <svg className={`w-4 h-4 text-text-secondary shrink-0 transition-transform ${itemOpen ? 'rotate-180' : ''}`} fill="none" stroke="currentColor" strokeWidth="2" viewBox="0 0 24 24"><polyline points="6 9 12 15 18 9"/></svg>
                </button>
                {itemOpen && (
                  <ul className="absolute z-50 mt-1 w-full bg-white border border-border rounded-xl shadow-lg overflow-hidden">
                    {items.map((item) => (
                      <li
                        key={item.item_id}
                        onMouseDown={() => { setItemId(item.item_id); setItemOpen(false) }}
                        className={`px-4 py-2.5 text-base cursor-pointer transition-colors ${itemId === item.item_id ? 'bg-brand/10 text-brand font-medium' : 'text-text-primary hover:bg-brand/5'}`}
                      >
                        <span className="font-medium">{item.title}</span>
                        <span className="text-text-secondary text-sm ml-2">retail {formatCurrency(item.retail_value)}</span>
                      </li>
                    ))}
                  </ul>
                )}
              </div>
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

            <FormField label="Max Price / Bid Ceiling ($, optional)">
              <TextInput
                type="number"
                min="0.01"
                step="0.01"
                placeholder="Leave empty for no limit"
                value={maxPrice}
                onChange={(e) => setMaxPrice(e.target.value)}
              />
              <p className="text-sm text-text-secondary mt-1">
                Bids above this amount will be rejected. Leave empty for no ceiling.
              </p>
            </FormField>

            <FormField label="Winners / Quantity (optional)">
              <TextInput
                type="number"
                min="1"
                max="100"
                placeholder="1"
                value={quantity}
                onChange={(e) => setQuantity(e.target.value)}
              />
              <p className="text-sm text-text-secondary mt-1">
                How many buyers can win. Leave at 1 for a standard single-winner auction.
              </p>
            </FormField>

            <FormField label="Schedule Start (optional)">
              <TextInput
                type="datetime-local"
                value={scheduledStart}
                onChange={(e) => setScheduledStart(e.target.value)}
                placeholder=""
              />
              <p className="text-sm text-text-secondary mt-1">
                Leave empty to start immediately. Set a future time to create a scheduled (PENDING) auction.
              </p>
            </FormField>

            <FormField label="Pickup Window">
              <div className="flex gap-3">
                <div className="flex-1">
                  <label className="text-xs text-text-secondary mb-1 block">Start</label>
                  <TextInput
                    type="datetime-local"
                    required
                    value={pickupStart}
                    onChange={(e) => setPickupStart(e.target.value)}
                  />
                </div>
                <div className="flex-1">
                  <label className="text-xs text-text-secondary mb-1 block">End</label>
                  <TextInput
                    type="datetime-local"
                    required
                    value={pickupEnd}
                    onChange={(e) => setPickupEnd(e.target.value)}
                  />
                </div>
              </div>
              <p className="text-sm text-text-secondary mt-1">
                When can the winner collect the item?
              </p>
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
