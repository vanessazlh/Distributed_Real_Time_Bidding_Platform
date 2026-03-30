import { useState, useEffect } from 'react'
import { Link, useParams, useNavigate } from 'react-router-dom'
import type { Payment, PaymentStatus } from '@/types'
import { useAuth } from '@/context/AuthContext'
import { api } from '@/lib/api'
import { formatCurrency } from '@/lib/utils'
import { Card, Button, Spinner, EmptyState } from '@/components/ui'
import { PageContainer } from '@/components/layout'
import { ChevronLeftIcon } from '@/components/icons'

const STATUS_STYLES: Record<PaymentStatus, { label: string; classes: string }> = {
  pending:    { label: 'Pending',    classes: 'bg-yellow-50 text-yellow-700 border border-yellow-200' },
  processing: { label: 'Processing', classes: 'bg-blue-50 text-blue-700 border border-blue-200' },
  completed:  { label: 'Completed',  classes: 'bg-green-50 text-green-700 border border-green-200' },
  failed:     { label: 'Failed',     classes: 'bg-red-50 text-red-700 border border-red-200' },
  refunded:   { label: 'Refunded',   classes: 'bg-surface-alt text-text-secondary border border-border' },
}

const STATUS_MESSAGES: Record<PaymentStatus, string> = {
  pending:    'Your payment is queued and will be processed shortly.',
  processing: 'Your payment is currently being processed.',
  completed:  'Payment successful — the item is yours.',
  failed:     'Payment could not be completed.',
  refunded:   'This payment has been refunded.',
}

export default function PaymentPage() {
  const { auctionId }   = useParams<{ auctionId: string }>()
  const { user, token } = useAuth()
  const navigate        = useNavigate()

  const [payment, setPayment] = useState<Payment | null>(null)
  const [loading, setLoading] = useState(true)
  const [error,   setError]   = useState<string | null>(null)

  useEffect(() => {
    if (!auctionId || !token) return
    api.payments.getByAuction(auctionId, token)
      .then(setPayment)
      .catch((err) => setError(err instanceof Error ? err.message : 'Failed to load payment'))
      .finally(() => setLoading(false))
  }, [auctionId, token])

  if (!user) {
    return (
      <PageContainer narrow>
        <EmptyState
          message="Sign in to view your payment"
          action={<Button onClick={() => navigate('/login')}>Sign In</Button>}
        />
      </PageContainer>
    )
  }

  if (loading) return <PageContainer narrow><Spinner className="py-20" /></PageContainer>

  if (error || !payment) {
    return (
      <PageContainer narrow>
        <EmptyState
          message={error ?? 'Payment not found.'}
          action={<Button onClick={() => navigate('/my-bids')}>Back to My Bids</Button>}
        />
      </PageContainer>
    )
  }

  const style = STATUS_STYLES[payment.status]

  return (
    <PageContainer narrow>
      <Link
        to="/my-bids"
        className="inline-flex items-center gap-1 text-text-secondary hover:text-brand text-sm font-medium transition-colors mb-8"
      >
        <ChevronLeftIcon /> My Bids
      </Link>

      <h1 className="font-sans font-semibold text-3xl text-text-primary mb-8">Payment</h1>

      <Card padding="p-8" className="flex flex-col gap-6">
        {/* Status banner */}
        <div className={`flex items-center gap-3 px-4 py-3 rounded-lg ${style.classes}`}>
          <span className="font-sans font-semibold text-sm">{style.label}</span>
          <span className="text-sm">{STATUS_MESSAGES[payment.status]}</span>
        </div>

        {payment.fail_reason && (
          <p className="text-sm text-red-600 bg-red-50 border border-red-200 rounded-lg px-4 py-3">
            {payment.fail_reason}
          </p>
        )}

        {/* Amount */}
        <div className="flex items-baseline justify-between border-b border-border pb-6">
          <span className="text-text-secondary font-sans text-sm">Amount due</span>
          <span className="font-display text-4xl text-text-primary">
            {formatCurrency(payment.amount)}
          </span>
        </div>

        {/* Details */}
        <dl className="flex flex-col gap-3 text-sm">
          <div className="flex justify-between">
            <dt className="text-text-secondary">Payment ID</dt>
            <dd className="text-text-primary font-mono text-xs">{payment.payment_id}</dd>
          </div>
          <div className="flex justify-between">
            <dt className="text-text-secondary">Auction</dt>
            <dd>
              <Link
                to={`/auction/${payment.auction_id}`}
                className="text-brand font-medium hover:underline text-xs font-mono"
              >
                {payment.auction_id}
              </Link>
            </dd>
          </div>
          <div className="flex justify-between">
            <dt className="text-text-secondary">Created</dt>
            <dd className="text-text-primary">{new Date(payment.created_at).toLocaleString()}</dd>
          </div>
          <div className="flex justify-between">
            <dt className="text-text-secondary">Last updated</dt>
            <dd className="text-text-primary">{new Date(payment.updated_at).toLocaleString()}</dd>
          </div>
        </dl>
      </Card>
    </PageContainer>
  )
}
