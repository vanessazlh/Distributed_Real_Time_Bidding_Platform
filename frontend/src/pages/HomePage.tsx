import { useState, useEffect, useCallback } from 'react'
import { useNavigate } from 'react-router-dom'
import type { Auction } from '@/types'
import { CATEGORIES } from '@/types'
import { api } from '@/lib/api'
import { pickupOverlaps } from '@/lib/utils'
import { useAuth } from '@/context/AuthContext'
import { AuctionCard } from '@/components/auction'
import { PageContainer } from '@/components/layout'
import { Spinner, EmptyState, FilterDropdown } from '@/components/ui'

const TABS = ['All', ...CATEGORIES] as const
type TabFilter = typeof TABS[number]

const PICKUP_OPTIONS = [
  { value: 'any',       label: 'Any Time' },
  { value: 'morning',   label: 'Morning' },
  { value: 'afternoon', label: 'Afternoon' },
  { value: 'evening',   label: 'Evening' },
] as const

const DISTANCE_OPTIONS = [
  { value: 'any', label: 'Any Distance' },
  { value: '2',   label: 'Within 2 km' },
  { value: '5',   label: 'Within 5 km' },
  { value: '10',  label: 'Within 10 km' },
] as const

export default function HomePage() {
  const { isSeller } = useAuth()
  const navigate = useNavigate()
  const [auctions, setAuctions] = useState<Auction[]>([])
  const [loading,  setLoading]  = useState(true)
  const [error,    setError]    = useState<string | null>(null)
  const [filter,         setFilter]         = useState<TabFilter>('All')
  const [pickupFilter,   setPickupFilter]   = useState('any')
  const [distanceFilter, setDistanceFilter] = useState('any')
  const [userCoords,     setUserCoords]     = useState<{ lat: number; lng: number } | null>(null)
  const [geoLoading,     setGeoLoading]     = useState(false)

  const fetchAuctions = useCallback((opts?: { lat?: number; lng?: number; radius_km?: number }) => {
    setLoading(true)
    setError(null)
    api.auctions.list(opts)
      .then(setAuctions)
      .catch((err) => setError(err instanceof Error ? err.message : 'Failed to load auctions'))
      .finally(() => setLoading(false))
  }, [])

  useEffect(() => {
    if (isSeller) { navigate('/seller/dashboard', { replace: true }); return }
    fetchAuctions()
    // Silently request location on load so distance badges and filter work immediately
    navigator.geolocation?.getCurrentPosition(
      (pos) => setUserCoords({ lat: pos.coords.latitude, lng: pos.coords.longitude }),
      () => {}, // denied — distance filter will re-prompt when selected
      { enableHighAccuracy: false, timeout: 5000, maximumAge: 60000 },
    )
  }, [isSeller, navigate, fetchAuctions])

  const handleDistanceFilter = (value: string) => {
    setDistanceFilter(value)
    if (value === 'any') {
      fetchAuctions()
      return
    }
    const radiusKm = parseFloat(value)
    if (userCoords) {
      fetchAuctions({ lat: userCoords.lat, lng: userCoords.lng, radius_km: radiusKm })
    } else {
      setGeoLoading(true)
      navigator.geolocation?.getCurrentPosition(
        (pos) => {
          const coords = { lat: pos.coords.latitude, lng: pos.coords.longitude }
          setUserCoords(coords)
          setGeoLoading(false)
          fetchAuctions({ lat: coords.lat, lng: coords.lng, radius_km: radiusKm })
        },
        (err) => {
          setGeoLoading(false)
          setDistanceFilter('any')
          setError(
            err.code === err.TIMEOUT
              ? 'Location request timed out — please try again.'
              : 'Location access denied — cannot filter by distance.',
          )
        },
        { enableHighAccuracy: false, timeout: 5000, maximumAge: 60000 },
      )
    }
  }

  const byCategory = filter === 'All'
    ? auctions
    : auctions.filter((a) => a.category === filter)

  const visible = pickupFilter === 'any'
    ? byCategory
    : byCategory.filter((a) => {
        if (!a.pickup_start || !a.pickup_end) return false
        if (pickupFilter === 'morning')   return pickupOverlaps(a.pickup_start, a.pickup_end, 0, 11)
        if (pickupFilter === 'afternoon') return pickupOverlaps(a.pickup_start, a.pickup_end, 12, 16)
        return pickupOverlaps(a.pickup_start, a.pickup_end, 17, 23) // evening
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

      {/* Category tabs + secondary filters */}
      <div className="grid grid-cols-[1fr_auto_1fr] items-end border-b border-border mb-8">
        <div />
        <div className="flex gap-8">
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

        <div className="flex items-center justify-end gap-2 pb-2.5">
          <FilterDropdown
            label="Pickup"
            options={[...PICKUP_OPTIONS]}
            value={pickupFilter}
            onChange={setPickupFilter}
          />
          <FilterDropdown
            label={geoLoading ? 'Locating…' : 'Nearby'}
            options={[...DISTANCE_OPTIONS]}
            value={distanceFilter}
            onChange={handleDistanceFilter}
          />
        </div>
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
            <AuctionCard key={auction.auction_id} auction={auction} userCoords={userCoords ?? undefined} />
          ))}
        </div>
      )}
    </PageContainer>
  )
}
