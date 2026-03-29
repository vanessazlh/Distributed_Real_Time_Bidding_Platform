import { useState, useEffect, useCallback } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import type { Auction, BidHistoryEntry, BidPlacedEvent } from '@/types'
import { useAuth } from '@/context/AuthContext'
import { api } from '@/lib/api'
import { useAuctionWebSocket } from '@/hooks/useAuctionWebSocket'
import { Avatar, Button, Card, EmptyState, Spinner, StatusBanner } from '@/components/ui'
import { BidHistoryFeed, BiddingPanel } from '@/components/auction'
import { PageContainer } from '@/components/layout'
import { ChevronLeftIcon } from '@/components/icons'

type BannerState = 'WINNING' | 'OUTBID' | null

export default function AuctionDetailPage() {
  const { id }          = useParams<{ id: string }>()
  const navigate        = useNavigate()
  const { user, token } = useAuth()

  const [auction,    setAuction]    = useState<Auction | null>(null)
  const [loading,    setLoading]    = useState(true)
  const [fetchError, setFetchError] = useState<string | null>(null)
  const [bidError,   setBidError]   = useState<string | null>(null)

  const [highestBid, setHighestBid] = useState(0)
  const [bidCount,   setBidCount]   = useState(0)
  const [flash,      setFlash]      = useState(false)
  const [banner,     setBanner]     = useState<BannerState>(null)
  const [bidInput,   setBidInput]   = useState('')
  const [bidHistory, setBidHistory] = useState<BidHistoryEntry[]>([])

  useEffect(() => {
    if (!id) return
    api.auctions.get(id)
      .then((a) => {
        setAuction(a)
        setHighestBid(a.current_highest_bid)
        setBidCount(a.bid_count)
      })
      .catch((err) => setFetchError(err instanceof Error ? err.message : 'Failed to load auction'))
      .finally(() => setLoading(false))
  }, [id])

  const triggerFlash = () => {
    setFlash(true)
    setTimeout(() => setFlash(false), 1000)
  }

  const applyNewBid = useCallback((amount: number, bidder: string) => {
    setHighestBid(amount)
    triggerFlash()
    setBidHistory((h) =>
      [{ id: Date.now(), user: bidder, amount, time: Date.now() }, ...h].slice(0, 8)
    )
    setBidCount((c) => c + 1)
  }, [])

  // Real-time updates via WebSocket
  const handleWsMessage = useCallback((event: BidPlacedEvent) => {
    applyNewBid(event.amount, event.user_id)
    setBanner((cur) => (cur === 'WINNING' ? 'OUTBID' : cur))
  }, [applyNewBid])

  useAuctionWebSocket(id ?? '', handleWsMessage)

  if (loading) {
    return (
      <PageContainer>
        <Spinner className="py-32" />
      </PageContainer>
    )
  }

  if (fetchError || !auction) {
    return (
      <PageContainer>
        <EmptyState
          message={fetchError ?? 'Auction not found.'}
          action={<Button onClick={() => navigate('/')}>Back to Auctions</Button>}
        />
      </PageContainer>
    )
  }

  const isClosed = auction.end_time <= Date.now() || auction.status === 'CLOSED'

  const handlePlaceBid = async (e: React.FormEvent) => {
    e.preventDefault()
    setBidError(null)
    const cents = Math.round(parseFloat(bidInput) * 100)
    if (cents < highestBid + 50) {
      setBidError(`Bid must be at least ${((highestBid + 50) / 100).toFixed(2)}`)
      return
    }
    try {
      const result = await api.auctions.placeBid(
        auction.auction_id,
        user?.user_id ?? '',
        cents,
        token ?? '',
      )
      applyNewBid(result.new_highest_bid, user?.username ?? 'you')
      setBanner('WINNING')
      setBidInput('')
    } catch (err) {
      setBidError(err instanceof Error ? err.message : 'Failed to place bid')
    }
  }

  return (
    <PageContainer>
      {/* Back */}
      <Link
        to="/"
        className="inline-flex items-center gap-1 text-text-secondary hover:text-brand text-sm font-medium transition-colors mb-8"
      >
        <ChevronLeftIcon /> All Auctions
      </Link>

      {bidError && (
        <div className="mb-6">
          <StatusBanner type="error" message={bidError} />
        </div>
      )}

      <div className="flex flex-col lg:flex-row gap-10">
        {/* ── Left: item info + bid history ── */}
        <div className="flex-[1.2] flex flex-col gap-8">
          <Card>
            <img
              src={auction.image_url}
              alt={auction.item.title}
              className="w-full h-[400px] object-cover mix-blend-multiply rounded-t-xl"
            />
            <div className="p-8">
              <div className="flex items-center gap-3 mb-4">
                <Avatar src={auction.shop_logo_url} alt={auction.item.shop_name} size="lg" />
                <Link
                  to={`/shop/${auction.item.shop_id}`}
                  className="text-brand font-semibold text-lg hover:underline"
                >
                  {auction.item.shop_name}
                </Link>
              </div>
              <h1 className="font-sans font-semibold text-3xl text-text-primary mb-4">
                {auction.item.title}
              </h1>
              <p className="text-text-secondary text-lg leading-relaxed">{auction.description}</p>
            </div>
          </Card>

          <BidHistoryFeed bids={bidHistory} />
        </div>

        {/* ── Right: sticky bidding panel ── */}
        <div className="flex-1">
          <div className="sticky top-28">
            <BiddingPanel
              auction={auction}
              highestBid={highestBid}
              bidCount={bidCount}
              flash={flash}
              banner={banner}
              isClosed={isClosed}
              user={user}
              bidInput={bidInput}
              onBidInputChange={setBidInput}
              onPlaceBid={handlePlaceBid}
              onSignIn={() => navigate('/login')}
            />
          </div>
        </div>
      </div>
    </PageContainer>
  )
}
