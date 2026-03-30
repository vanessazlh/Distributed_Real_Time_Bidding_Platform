import { useState, useEffect } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import type { Payment } from '@/types'
import { useAuth } from '@/context/AuthContext'
import { api } from '@/lib/api'
import { formatCurrency } from '@/lib/utils'
import { Card, Button, Spinner, EmptyState } from '@/components/ui'
import { PageContainer } from '@/components/layout'
import { ChevronLeftIcon } from '@/components/icons'

const STATUS_BADGE: Record<string, string> = {
  pending:    'bg-yellow-50 text-yellow-700 border border-yellow-200',
  processing: 'bg-blue-50 text-blue-700 border border-blue-200',
  completed:  'bg-green-50 text-green-700 border border-green-200',
  failed:     'bg-red-50 text-red-700 border border-red-200',
  refunded:   'bg-surface-alt text-text-secondary border border-border',
}

export default function MyPaymentsPage() {
  const { user, token } = useAuth()
  const navigate        = useNavigate()

  const [payments, setPayments] = useState<Payment[]>([])
  const [loading,  setLoading]  = useState(true)
  const [error,    setError]    = useState<string | null>(null)

  useEffect(() => {
    if (!user || !token) { setLoading(false); return }
    api.payments.listByUser(user.user_id, token)
      .then(setPayments)
      .catch((err) => setError(err instanceof Error ? err.message : 'Failed to load payments'))
      .finally(() => setLoading(false))
  }, [user, token])

  if (!user) {
    return (
      <PageContainer narrow>
        <EmptyState
          message="Sign in to view your payments"
          action={<Button onClick={() => navigate('/login')}>Sign In</Button>}
        />
      </PageContainer>
    )
  }

  if (loading) return <PageContainer narrow><Spinner className="py-20" /></PageContainer>

  if (error) return <PageContainer narrow><EmptyState message={error} /></PageContainer>

  const totalSpent = payments
    .filter((p) => p.status === 'completed')
    .reduce((sum, p) => sum + p.amount, 0)

  return (
    <PageContainer narrow>
      <Link
        to="/"
        className="inline-flex items-center gap-1 text-text-secondary hover:text-brand text-sm font-medium transition-colors mb-8"
      >
        <ChevronLeftIcon /> All Auctions
      </Link>

      <div className="flex items-end justify-between mb-8">
        <h1 className="font-sans font-semibold text-3xl text-text-primary">My Payments</h1>
        {totalSpent > 0 && (
          <p className="text-text-secondary text-sm">
            Total spent: <span className="font-semibold text-text-primary">{formatCurrency(totalSpent)}</span>
          </p>
        )}
      </div>

      {payments.length === 0 ? (
        <EmptyState
          message="No payments yet. Win an auction to see your payments here."
          action={<Button onClick={() => navigate('/')}>Browse Auctions</Button>}
        />
      ) : (
        <Card>
          {payments.map((payment, i) => (
            <div
              key={payment.payment_id}
              className={`p-6 flex items-center justify-between ${i !== 0 ? 'border-t border-border' : ''}`}
            >
              <div className="flex flex-col gap-1">
                <span
                  className={`self-start text-xs font-semibold px-2 py-0.5 rounded-full capitalize ${STATUS_BADGE[payment.status] ?? ''}`}
                >
                  {payment.status}
                </span>
                <p className="text-text-secondary text-xs font-mono mt-1">
                  {payment.auction_id}
                </p>
                <p className="text-text-secondary text-xs">
                  {new Date(payment.created_at).toLocaleDateString()}
                </p>
              </div>

              <div className="flex flex-col items-end gap-2">
                <p className="font-display text-2xl text-text-primary">
                  {formatCurrency(payment.amount)}
                </p>
                <Link
                  to={`/payment/auction/${payment.auction_id}`}
                  className="text-brand text-xs font-medium hover:underline"
                >
                  View details →
                </Link>
              </div>
            </div>
          ))}
        </Card>
      )}
    </PageContainer>
  )
}
