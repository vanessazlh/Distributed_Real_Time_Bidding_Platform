import { useState } from 'react'
import { useSearchParams, useNavigate, Link } from 'react-router-dom'
import { useAuth } from '@/context/AuthContext'
import { api, ApiError } from '@/lib/api'
import { Card, Button, EmptyState } from '@/components/ui'
import { PageContainer } from '@/components/layout'
import { ChevronLeftIcon } from '@/components/icons'

// ── Interactive star input ────────────────────────────────────────────────────

function StarInput({ value, onChange }: { value: number; onChange: (v: number) => void }) {
  const [hovered, setHovered] = useState(0)
  const active = hovered || value
  return (
    <div className="flex gap-1">
      {[1, 2, 3, 4, 5].map((i) => (
        <button
          key={i}
          type="button"
          onMouseEnter={() => setHovered(i)}
          onMouseLeave={() => setHovered(0)}
          onClick={() => onChange(i)}
          className={`text-3xl transition-colors ${i <= active ? 'text-yellow-400' : 'text-gray-300 hover:text-yellow-300'}`}
          aria-label={`Rate ${i} star${i !== 1 ? 's' : ''}`}
        >
          ★
        </button>
      ))}
    </div>
  )
}

export default function WriteReviewPage() {
  const [searchParams] = useSearchParams()
  const navigate = useNavigate()
  const { user, token } = useAuth()

  // All context is passed via URL params — no auction fetch needed
  const auctionId = searchParams.get('auction_id') ?? ''
  const shopId    = searchParams.get('shop_id')    ?? ''
  const shopName  = searchParams.get('shop_name')  ?? ''
  const itemTitle = searchParams.get('item_title') ?? ''

  const [rating,    setRating]    = useState(0)
  const [comment,   setComment]   = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [submitErr,  setSubmitErr]  = useState<string | null>(null)

  const handleSubmit = async () => {
    if (!shopId || !token || rating === 0) return
    setSubmitting(true)
    setSubmitErr(null)
    try {
      await api.reviews.create(shopId, { auction_id: auctionId, rating, comment: comment.trim() || undefined }, token)
      navigate(`/shop/${shopId}`, { state: { reviewSubmitted: true } })
    } catch (e) {
      setSubmitErr(e instanceof ApiError ? e.message : 'Failed to submit review')
    } finally {
      setSubmitting(false)
    }
  }

  if (!user) {
    return (
      <PageContainer narrow>
        <EmptyState
          message="Sign in to leave a review"
          action={<Button><Link to="/login">Sign In</Link></Button>}
        />
      </PageContainer>
    )
  }

  if (!auctionId || !shopId) {
    return (
      <PageContainer narrow>
        <EmptyState message="No auction specified." />
      </PageContainer>
    )
  }

  return (
    <PageContainer narrow>
      <Link
        to="/profile/bids"
        className="inline-flex items-center gap-1 text-text-secondary hover:text-brand text-base font-medium transition-colors mb-8"
      >
        <ChevronLeftIcon /> My Bids
      </Link>

      <h1 className="font-sans font-semibold text-3xl text-text-primary mb-2">Leave a Review</h1>

      {shopName && (
        <p className="text-text-secondary text-base mb-8">
          {itemTitle && <><span className="font-medium text-text-primary">{itemTitle}</span> from </>}
          <span className="font-medium text-brand">{shopName}</span>
        </p>
      )}

      <Card className="p-8">
        <div className="mb-6">
          <label className="block text-sm font-medium text-text-secondary mb-2">Your Rating</label>
          <StarInput value={rating} onChange={setRating} />
          {rating > 0 && (
            <p className="text-sm text-text-secondary mt-1">
              {['', 'Poor', 'Fair', 'Good', 'Very Good', 'Excellent'][rating]}
            </p>
          )}
        </div>

        <div className="mb-6">
          <label className="block text-sm font-medium text-text-secondary mb-2">
            Comment <span className="font-normal">(optional)</span>
          </label>
          <textarea
            value={comment}
            onChange={(e) => setComment(e.target.value)}
            rows={4}
            maxLength={500}
            placeholder="Share your experience with this shop…"
            className="w-full border border-border rounded-lg px-3 py-2 text-sm resize-none focus:outline-none focus:ring-2 focus:ring-brand/40"
          />
          <p className="text-xs text-text-secondary mt-1 text-right">{comment.length}/500</p>
        </div>

        {submitErr && <p className="text-red-500 text-sm mb-4">{submitErr}</p>}

        <div className="flex items-center gap-3">
          <Button
            variant="primary"
            onClick={handleSubmit}
            disabled={submitting || rating === 0}
          >
            {submitting ? 'Submitting…' : 'Submit Review'}
          </Button>
          <Button variant="ghost" onClick={() => navigate('/profile/bids')}>
            Cancel
          </Button>
        </div>
      </Card>
    </PageContainer>
  )
}
