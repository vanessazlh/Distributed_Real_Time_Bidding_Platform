import { useState, useEffect } from 'react'
import { useParams, useNavigate, useLocation, Link } from 'react-router-dom'
import type { Auction, Shop, Review, ReviewsResponse } from '@/types'
import { useAuth } from '@/context/AuthContext'
import { api, ApiError } from '@/lib/api'
import { Avatar, Card, Button, EmptyState, Spinner } from '@/components/ui'
import { AuctionCard } from '@/components/auction'
import { PageContainer } from '@/components/layout'
import { ChevronLeftIcon } from '@/components/icons'
import { formatCurrency } from '@/lib/utils'

const PAGE_SIZE = 5

// ── Star display ─────────────────────────────────────────────────────────────

function StarDisplay({ rating, size = 'sm' }: { rating: number; size?: 'sm' | 'lg' }) {
  const px = size === 'lg' ? 'text-xl' : 'text-base'
  return (
    <span className={px} aria-label={`${rating} out of 5 stars`}>
      {[1, 2, 3, 4, 5].map((i) => (
        <span key={i} className={i <= rating ? 'text-yellow-400' : 'text-gray-300'}>★</span>
      ))}
    </span>
  )
}

// ── Rating summary (header) ───────────────────────────────────────────────────

function RatingSummary({ data }: { data: ReviewsResponse | null }) {
  // Still loading — show nothing
  if (data === null) return null
  // Loaded but no reviews
  if (data.total_reviews === 0) {
    return <p className="text-text-secondary text-sm mt-1 mb-2">No ratings yet</p>
  }
  return (
    <div className="flex items-center gap-2 mt-1 mb-2">
      <StarDisplay rating={Math.round(data.average_rating)} size="lg" />
      <span className="font-display text-xl text-text-primary">{data.average_rating.toFixed(1)}</span>
      <span className="text-text-secondary text-sm">
        ({data.total_reviews} {data.total_reviews === 1 ? 'review' : 'reviews'})
      </span>
    </div>
  )
}

// ── Review card ───────────────────────────────────────────────────────────────

function ReviewCard({
  review,
  shopId,
  isOwner,
  token,
  onReplyPosted,
}: {
  review: Review
  shopId: string
  isOwner: boolean
  token: string | null
  onReplyPosted: (updated: Review) => void
}) {
  const [replying,  setReplying]  = useState(false)
  const [replyText, setReplyText] = useState(review.seller_reply ?? '')
  const [saving,    setSaving]    = useState(false)
  const [err,       setErr]       = useState<string | null>(null)

  const date = new Date(review.created_at).toLocaleDateString(undefined, {
    year: 'numeric', month: 'short', day: 'numeric',
  })

  const submitReply = async () => {
    if (!token || !replyText.trim()) return
    setSaving(true)
    setErr(null)
    try {
      const updated = await api.reviews.reply(shopId, review.review_id, replyText.trim(), token)
      onReplyPosted(updated)
      setReplying(false)
    } catch (e) {
      setErr(e instanceof ApiError ? e.message : 'Failed to post reply')
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="p-5 border-b border-border last:border-0">
      <div className="flex items-start justify-between gap-2 mb-1">
        <div className="flex items-center gap-2">
          <span className="font-sans font-medium text-text-primary text-sm">{review.reviewer_username || 'Buyer'}</span>
          <StarDisplay rating={review.rating} />
        </div>
        <span className="text-text-secondary text-xs shrink-0">{date}</span>
      </div>
      {review.comment && (
        <p className="text-text-primary text-sm mt-1">{review.comment}</p>
      )}

      {/* Seller reply */}
      {review.seller_reply && !replying && (
        <div className="mt-3 pl-4 border-l-2 border-brand/40">
          <p className="text-xs text-brand font-semibold mb-0.5">Shop response</p>
          <p className="text-sm text-text-secondary">{review.seller_reply}</p>
          {isOwner && (
            <button
              onClick={() => { setReplyText(review.seller_reply ?? ''); setReplying(true) }}
              className="text-xs text-brand mt-1 hover:underline"
            >
              Edit reply
            </button>
          )}
        </div>
      )}

      {/* Reply form */}
      {isOwner && !review.seller_reply && !replying && (
        <button
          onClick={() => setReplying(true)}
          className="text-xs text-brand mt-2 hover:underline"
        >
          + Reply
        </button>
      )}
      {replying && (
        <div className="mt-3">
          <textarea
            value={replyText}
            onChange={(e) => setReplyText(e.target.value)}
            rows={2}
            className="w-full border border-border rounded-lg p-2 text-sm resize-none focus:outline-none focus:ring-2 focus:ring-brand/40"
            placeholder="Write a reply…"
          />
          {err && <p className="text-red-500 text-xs mt-1">{err}</p>}
          <div className="flex gap-2 mt-2">
            <Button size="sm" onClick={submitReply} disabled={saving || !replyText.trim()}>
              {saving ? 'Saving…' : 'Post Reply'}
            </Button>
            <Button size="sm" variant="ghost" onClick={() => setReplying(false)}>Cancel</Button>
          </div>
        </div>
      )}
    </div>
  )
}

// ── Main page ─────────────────────────────────────────────────────────────────

export default function ShopDetailPage() {
  const { id }          = useParams<{ id: string }>()
  const { user, token } = useAuth()
  const navigate        = useNavigate()
  const location        = useLocation()

  const reviewSubmitted = (location.state as { reviewSubmitted?: boolean } | null)?.reviewSubmitted ?? false

  const [shop,       setShop]       = useState<Shop | null>(null)
  const [auctions,   setAuctions]   = useState<Auction[]>([])
  const [reviewData, setReviewData] = useState<ReviewsResponse | null>(null)
  const [loading,    setLoading]    = useState(true)
  const [error,      setError]      = useState<string | null>(null)

  // Pagination
  const [visibleCount, setVisibleCount] = useState(PAGE_SIZE)

  useEffect(() => {
    if (!id) return
    Promise.all([
      api.shops.get(id),
      api.auctions.list().then((all) => all.filter((a) => a.item.shop_id === id)),
      api.reviews.list(id),
    ])
      .then(([s, a, r]) => { setShop(s); setAuctions(a); setReviewData(r) })
      .catch((err) => setError(err instanceof Error ? err.message : 'Failed to load shop'))
      .finally(() => setLoading(false))
  }, [id])

  if (loading) {
    return <PageContainer><Spinner className="py-32" /></PageContainer>
  }

  if (error || !shop) {
    return (
      <PageContainer>
        <EmptyState
          message={error ?? 'Shop not found.'}
          action={<Button onClick={() => navigate('/')}>Back to Auctions</Button>}
        />
      </PageContainer>
    )
  }

  const isOwner = user?.user_id === shop.owner_id
  const open    = auctions.filter((a) => a.status === 'OPEN')
  const closed  = auctions.filter((a) => a.status === 'CLOSED')
  const reviews = reviewData?.reviews ?? []
  const visibleReviews = reviews.slice(0, visibleCount)
  const hasMore = visibleCount < reviews.length

  const handleReplyPosted = (updated: Review) => {
    setReviewData((prev) => prev
      ? { ...prev, reviews: prev.reviews.map((r) => r.review_id === updated.review_id ? updated : r) }
      : prev
    )
  }

  return (
    <PageContainer>
      <Link
        to={isOwner ? '/seller/dashboard' : '/'}
        className="inline-flex items-center gap-1 text-text-secondary hover:text-brand text-base font-medium transition-colors mb-8"
      >
        <ChevronLeftIcon /> {isOwner ? 'Back to Dashboard' : 'All Auctions'}
      </Link>

      {/* Shop header */}
      <Card className="mb-10 flex flex-col items-center text-center" padding="p-10">
        <Avatar src={shop.logo_url} alt={shop.name} size="xl" />
        <h1 className="font-display text-4xl text-text-primary mt-4 mb-2">{shop.name}</h1>
        <p className="text-text-secondary mb-2">{shop.location}</p>
        <RatingSummary data={reviewData} />
        <p className="text-text-secondary text-base">Local shop selling surplus food at auction.</p>

        {isOwner && (
          <div className="flex gap-3 mt-6">
            <Button onClick={() => navigate(`/shops/${id}/items/new`)}>+ Add Item</Button>
            <Button variant="primary" onClick={() => navigate(`/auctions/new?shopId=${id}`)}>+ Publish Auction</Button>
          </div>
        )}
      </Card>

      {/* Active auctions */}
      {open.length > 0 && (
        <section className="mb-10">
          <h2 className="font-sans font-semibold text-2xl text-text-primary mb-6">Active Auctions</h2>
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
            {open.map((a) => <AuctionCard key={a.auction_id} auction={a} />)}
          </div>
        </section>
      )}

      {/* Past auctions */}
      {closed.length > 0 && (
        <section className="mb-10">
          <h2 className="font-sans font-semibold text-2xl text-text-primary mb-6">Past Auctions</h2>
          <Card>
            {closed.map((a, i) => (
              <div
                key={a.auction_id}
                className={`p-5 flex items-center justify-between ${i !== 0 ? 'border-t border-border' : ''}`}
              >
                <div>
                  <p className="font-sans font-medium text-text-primary">{a.item.title}</p>
                  <p className="text-text-secondary text-base mt-0.5">{a.bid_count} bids</p>
                </div>
                <div className="text-right">
                  <p className="font-display text-lg text-text-secondary line-through">{formatCurrency(a.retail_price)}</p>
                  <p className="font-display text-lg text-text-primary">{formatCurrency(a.current_highest_bid)}</p>
                </div>
              </div>
            ))}
          </Card>
        </section>
      )}

      {open.length === 0 && closed.length === 0 && (
        <p className="text-text-secondary text-center py-10">No auctions yet for this shop.</p>
      )}

      {/* Reviews section */}
      <section className="mt-4">
        <h2 className="font-sans font-semibold text-2xl text-text-primary mb-6">
          Reviews
          {reviewData && reviewData.total_reviews > 0 && (
            <span className="ml-2 text-text-secondary font-normal text-lg">({reviewData.total_reviews})</span>
          )}
        </h2>

        {reviewSubmitted && (
          <p className="text-green-600 text-sm mb-4 font-medium">Review submitted. Thank you!</p>
        )}

        {/* Prompt buyers to leave a review via My Bids */}
        {user && !isOwner && (
          <Card className="mb-6 p-5 flex items-center justify-between">
            <p className="text-text-secondary text-sm">Won an auction here? Share your experience.</p>
            <Link
              to="/mybids"
              className="text-brand text-sm font-medium hover:underline shrink-0 ml-4"
            >
              Go to My Bids →
            </Link>
          </Card>
        )}

        {/* Review list */}
        {reviews.length > 0 ? (
          <>
            <Card>
              {visibleReviews.map((r) => (
                <ReviewCard
                  key={r.review_id}
                  review={r}
                  shopId={id!}
                  isOwner={isOwner}
                  token={token}
                  onReplyPosted={handleReplyPosted}
                />
              ))}
            </Card>
            {hasMore && (
              <div className="text-center mt-4">
                <Button
                  variant="ghost"
                  onClick={() => setVisibleCount((c) => c + PAGE_SIZE)}
                >
                  Show more ({reviews.length - visibleCount} remaining)
                </Button>
              </div>
            )}
          </>
        ) : (
          <p className="text-text-secondary text-center py-8">No reviews yet.</p>
        )}
      </section>
    </PageContainer>
  )
}

