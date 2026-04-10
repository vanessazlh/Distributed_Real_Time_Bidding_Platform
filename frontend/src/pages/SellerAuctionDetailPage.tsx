import { useState, useEffect, useCallback } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import type { Auction, AuctionBid, BidPlacedEvent, AuctionClosedEvent, Payment } from '@/types'
import { useAuth } from '@/context/AuthContext'
import { api } from '@/lib/api'
import { useAuctionWebSocket } from '@/hooks/useAuctionWebSocket'
import { Badge, Button, Card, EmptyState, Spinner, StatCard, StatusBanner } from '@/components/ui'
import { CountdownTimer } from '@/components/auction'
import { PageContainer } from '@/components/layout'
import { ChevronLeftIcon } from '@/components/icons'
import { formatCurrency, formatPickupWindow, maskUsername, timeAgo } from '@/lib/utils'

export default function SellerAuctionDetailPage() {
  const { auctionId } = useParams<{ auctionId: string }>()
  const navigate = useNavigate()
  const { user, token, isSeller } = useAuth()

  const [auction,  setAuction]  = useState<Auction | null>(null)
  const [bids,     setBids]     = useState<AuctionBid[]>([])
  const [payment,  setPayment]  = useState<Payment | null>(null)
  const [loading,  setLoading]  = useState(true)
  const [error,    setError]    = useState<string | null>(null)
  const [closing,  setClosing]  = useState(false)
  const [payError, setPayError] = useState<string | null>(null)

  // Derived state kept in sync with WebSocket
  const [highestBid, setHighestBid] = useState(0)
  const [bidCount,   setBidCount]   = useState(0)

  // ── Fetch auction + bids ──────────────────────────────────────────────────
  useEffect(() => {
    if (!auctionId) return

    Promise.all([
      api.auctions.get(auctionId),
      api.auctions.bids(auctionId),
    ])
      .then(([a, b]) => {
        setAuction(a)
        setHighestBid(a.current_highest_bid)
        setBidCount(a.bid_count)
        // Sort bids newest-first
        setBids(b.sort((x, y) => y.timestamp - x.timestamp))
      })
      .catch((err) => setError(err instanceof Error ? err.message : 'Failed to load auction'))
      .finally(() => setLoading(false))
  }, [auctionId])

  // ── Fetch payment status for closed auctions ─────────────────────────────
  useEffect(() => {
    if (!auctionId || !token || !auction || auction.status !== 'CLOSED') return
    api.payments.getByAuction(auctionId, token)
      .then(setPayment)
      .catch(() => setPayError('No payment record found'))
  }, [auctionId, token, auction?.status])

  // ── WebSocket: real-time bid + close events ───────────────────────────────
  const handleBidPlaced = useCallback((event: BidPlacedEvent) => {
    setHighestBid(event.amount)
    setBidCount((c) => c + 1)
    setBids((prev) => [
      {
        bid_id:    `ws-${Date.now()}`,
        user_id:   event.user_id,
        amount:    event.amount,
        timestamp: Date.now(),
        status:    'WINNING',
      },
      // Mark the previous top bid as OUTBID
      ...prev.map((b, i) => i === 0 && b.status === 'WINNING' ? { ...b, status: 'OUTBID' as const } : b),
    ])
  }, [])

  const handleAuctionClosed = useCallback((event: AuctionClosedEvent) => {
    setAuction((prev) => prev ? { ...prev, status: 'CLOSED' } : prev)
    // Mark winner bid
    setBids((prev) =>
      prev.map((b) =>
        b.amount === event.winning_bid ? { ...b, status: 'WON' } : b
      )
    )
  }, [])

  useAuctionWebSocket(auctionId ?? '', {
    onBidPlaced: handleBidPlaced,
    onAuctionClosed: handleAuctionClosed,
  })

  // ── Close auction handler ─────────────────────────────────────────────────
  const handleClose = async () => {
    if (!auctionId || !token) return
    setClosing(true)
    try {
      await api.auctions.close(auctionId, token)
      setAuction((prev) => prev ? { ...prev, status: 'CLOSED' } : prev)
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Failed to close auction')
    } finally {
      setClosing(false)
    }
  }

  // ── Guards ────────────────────────────────────────────────────────────────
  if (!user || !isSeller) {
    return (
      <PageContainer narrow>
        <EmptyState
          message="Sign in as a seller to view auction details"
          action={<Button onClick={() => navigate('/shop/login')}>Seller Sign In</Button>}
        />
      </PageContainer>
    )
  }

  if (loading) {
    return <PageContainer><Spinner className="py-32" /></PageContainer>
  }

  if (error || !auction) {
    return (
      <PageContainer>
        <EmptyState
          message={error ?? 'Auction not found.'}
          action={<Button onClick={() => navigate('/seller/dashboard')}>Back to Dashboard</Button>}
        />
      </PageContainer>
    )
  }

  const isClosed  = auction.status === 'CLOSED'
  const isPending = auction.status === 'PENDING'
  const wonBids   = bids.filter((b) => b.status === 'WON')

  // Determine shopId for back navigation from the auction item data
  const shopId = auction.item.shop_id

  const PAYMENT_BADGE: Record<string, string> = {
    pending:    'border-alert text-alert',
    processing: 'border-blue-400 text-blue-600',
    completed:  'border-green-600 text-green-700',
    failed:     'border-critical text-critical',
    refunded:   'border-text-secondary text-text-secondary',
  }

  return (
    <PageContainer>
      {/* Back link */}
      <Link
        to={`/seller/shops/${shopId}/auctions`}
        className="inline-flex items-center gap-1 text-text-secondary hover:text-brand text-base font-medium transition-colors mb-8"
      >
        <ChevronLeftIcon /> Back to Auctions
      </Link>

      {/* ── Header ───────────────────────────────────────────────────────── */}
      <div className="py-6 flex items-end justify-between border-b border-border mb-8">
        <div className="flex items-center gap-5 min-w-0">
          {auction.image_url && (
            <img
              src={auction.image_url}
              alt={auction.item.title}
              className="w-20 h-20 rounded-xl object-cover shrink-0"
            />
          )}
          <div className="min-w-0">
            <div className="flex items-center gap-2 mb-1">
              <Badge status={auction.status} />
              {auction.quantity > 1 && (
                <span className="px-3 py-1 text-xs font-semibold rounded-full border border-brand text-brand">
                  {auction.quantity} winners
                </span>
              )}
            </div>
            <h1 className="font-display text-3xl text-text-primary truncate">
              {auction.item.title}
            </h1>
            <p className="text-text-secondary text-base mt-1">
              {auction.item.shop_name}
            </p>
          </div>
        </div>

        <div className="flex items-center gap-3 shrink-0">
          {!isClosed && !isPending && (
            <Button
              variant="action"
              disabled={closing}
              onClick={handleClose}
            >
              {closing ? 'Closing...' : 'Close Auction'}
            </Button>
          )}
        </div>
      </div>

      {/* ── Stats row ────────────────────────────────────────────────────── */}
      <div className="flex gap-4 mb-8">
        <StatCard label="Current Bid" value={formatCurrency(highestBid)} />
        <StatCard label="Retail Price" value={formatCurrency(auction.retail_price)} />
        <StatCard label="Total Bids" value={bidCount} />
        {auction.max_price > 0 && (
          <StatCard label="Max Price" value={formatCurrency(auction.max_price)} />
        )}
        <Card className="flex-1 text-center" padding="p-5">
          <p className="text-text-secondary text-sm mb-1">Time Left</p>
          <div className="font-display text-2xl text-text-primary">
            {isClosed
              ? <span className="text-text-secondary">Ended</span>
              : isPending
                ? <span className="text-blue-600">Scheduled</span>
                : <CountdownTimer endTime={auction.end_time} />}
          </div>
        </Card>
      </div>

      <div className="flex flex-col lg:flex-row gap-10">
        {/* ── Left: Bid History ────────────────────────────────────────── */}
        <div className="flex-[1.4]">
          <h2 className="font-sans font-semibold text-xl text-text-primary mb-4">
            Bid History
          </h2>

          {bids.length === 0 ? (
            <EmptyState message="No bids placed yet." />
          ) : (
            <Card>
              {/* Table header */}
              <div className="px-8 py-3 flex items-center text-text-secondary text-sm font-semibold border-b border-border">
                <span className="flex-1">Bidder</span>
                <span className="w-28 text-right">Amount</span>
                <span className="w-24 text-center">Status</span>
                <span className="w-28 text-right">Time</span>
              </div>

              {bids.map((bid, i) => (
                <div
                  key={bid.bid_id}
                  className={`px-8 py-4 flex items-center ${i !== 0 ? 'border-t border-border' : ''}`}
                >
                  <span className="flex-1 font-sans text-text-primary">
                    {maskUsername(bid.user_id)}
                  </span>
                  <span className="w-28 text-right font-serif text-lg text-text-primary">
                    {formatCurrency(bid.amount)}
                  </span>
                  <span className="w-24 flex justify-center">
                    <Badge status={bid.status} />
                  </span>
                  <span className="w-28 text-right text-text-secondary text-sm">
                    {timeAgo(bid.timestamp)}
                  </span>
                </div>
              ))}
            </Card>
          )}
        </div>

        {/* ── Right: Details + Payment ─────────────────────────────────── */}
        <div className="flex-1">
          <div className="sticky top-28 flex flex-col gap-6">
            {/* Item details */}
            <Card padding="p-6">
              <h3 className="font-sans font-semibold text-lg text-text-primary mb-4">
                Item Details
              </h3>
              {auction.description && (
                <p className="text-text-secondary text-base leading-relaxed mb-4">
                  {auction.description}
                </p>
              )}
              <dl className="flex flex-col gap-3">
                {auction.category && (
                  <div className="flex justify-between">
                    <dt className="text-text-secondary text-sm">Category</dt>
                    <dd className="text-text-primary text-sm font-medium">{auction.category}</dd>
                  </div>
                )}
                <div className="flex justify-between">
                  <dt className="text-text-secondary text-sm">Quantity</dt>
                  <dd className="text-text-primary text-sm font-medium">
                    {auction.quantity} winner{auction.quantity !== 1 ? 's' : ''}
                  </dd>
                </div>
                {auction.max_price > 0 && (
                  <div className="flex justify-between">
                    <dt className="text-text-secondary text-sm">Bid Ceiling</dt>
                    <dd className="text-text-primary text-sm font-medium">{formatCurrency(auction.max_price)}</dd>
                  </div>
                )}
                {auction.pickup_start && auction.pickup_end && (
                  <div className="flex justify-between">
                    <dt className="text-text-secondary text-sm">Pickup Window</dt>
                    <dd className="text-text-primary text-sm font-medium">
                      {formatPickupWindow(auction.pickup_start, auction.pickup_end)}
                    </dd>
                  </div>
                )}
              </dl>
            </Card>

            {/* Winners (for closed auctions) */}
            {isClosed && wonBids.length > 0 && (
              <Card padding="p-6">
                <h3 className="font-sans font-semibold text-lg text-text-primary mb-4">
                  Winner{wonBids.length !== 1 ? 's' : ''}
                </h3>
                {wonBids.map((b) => (
                  <div key={b.bid_id} className="flex items-center justify-between py-2">
                    <div className="flex items-center gap-2">
                      <Badge status="WON" />
                      <span className="text-text-primary text-sm font-medium">
                        {maskUsername(b.user_id)}
                      </span>
                    </div>
                    <span className="font-serif text-lg text-text-primary">
                      {formatCurrency(b.amount)}
                    </span>
                  </div>
                ))}
              </Card>
            )}

            {/* Payment status (for closed auctions) */}
            {isClosed && (
              <Card padding="p-6">
                <h3 className="font-sans font-semibold text-lg text-text-primary mb-4">
                  Payment Status
                </h3>
                {payment ? (
                  <div className="flex flex-col gap-3">
                    <div className="flex items-center justify-between">
                      <span className="text-text-secondary text-sm">Status</span>
                      <span className={`px-3 py-1 text-xs font-semibold rounded-full border ${PAYMENT_BADGE[payment.status] ?? ''}`}>
                        {payment.status.toUpperCase()}
                      </span>
                    </div>
                    <div className="flex justify-between">
                      <span className="text-text-secondary text-sm">Amount</span>
                      <span className="text-text-primary text-sm font-medium">{formatCurrency(payment.amount)}</span>
                    </div>
                    {payment.fail_reason && (
                      <StatusBanner type="error" message={payment.fail_reason} />
                    )}
                  </div>
                ) : (
                  <p className="text-text-secondary text-sm">
                    {payError ?? 'Loading payment info...'}
                  </p>
                )}
              </Card>
            )}
          </div>
        </div>
      </div>
    </PageContainer>
  )
}
