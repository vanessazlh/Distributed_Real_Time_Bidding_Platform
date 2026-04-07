import { useState, useEffect } from 'react'
import { Link, useParams, useNavigate } from 'react-router-dom'
import type { UserBid, Payment } from '@/types'
import { useAuth } from '@/context/AuthContext'
import { api } from '@/lib/api'
import { formatCurrency, timeAgo } from '@/lib/utils'
import { Card, Badge, StatCard, EmptyState, Button, Spinner, FormField, TextInput, StatusBanner } from '@/components/ui'
import { PageContainer } from '@/components/layout'
import { UserIcon } from '@/components/icons'

type Tab = 'account' | 'bids' | 'payments'

const TABS: { key: Tab; label: string; path: string }[] = [
  { key: 'account',  label: 'Account',  path: '/profile' },
  { key: 'bids',     label: 'My Bids',  path: '/profile/bids' },
  { key: 'payments', label: 'Payments', path: '/profile/payments' },
]

const STATUS_BADGE: Record<string, string> = {
  pending:    'bg-yellow-50 text-yellow-700 border border-yellow-200',
  processing: 'bg-blue-50 text-blue-700 border border-blue-200',
  completed:  'bg-green-50 text-green-700 border border-green-200',
  failed:     'bg-red-50 text-red-700 border border-red-200',
  refunded:   'bg-surface-alt text-text-secondary border border-border',
}

export default function ProfilePage() {
  const { tab } = useParams<{ tab?: string }>()
  const activeTab: Tab = tab === 'bids' ? 'bids' : tab === 'payments' ? 'payments' : 'account'

  const { user, token, isSeller, updateUser } = useAuth()
  const navigate = useNavigate()

  if (!user) {
    return (
      <PageContainer narrow>
        <EmptyState
          message="Sign in to view your profile"
          action={<Button onClick={() => navigate('/login')}>Sign In</Button>}
        />
      </PageContainer>
    )
  }

  return (
    <PageContainer>
      <div className="flex gap-8 max-w-5xl mx-auto">
        {/* Sidebar */}
        <aside className="w-56 shrink-0">
          <div className="flex flex-col items-center gap-3 mb-8 pt-4">
            <div className="w-16 h-16 rounded-full bg-brand/10 flex items-center justify-center">
              <UserIcon width={32} height={32} className="text-brand" />
            </div>
            <p className="font-sans font-semibold text-sm text-text-primary">{user.username}</p>
            <Badge status={user.role === 'seller' ? 'SELLER' : 'BUYER'} />
          </div>
          <nav className="flex flex-col gap-1">
            {TABS.map((t) => (
              <Link
                key={t.key}
                to={t.path}
                className={`px-4 py-2.5 rounded-lg text-sm font-medium transition-colors ${
                  activeTab === t.key
                    ? 'bg-brand/10 text-brand'
                    : 'text-text-secondary hover:text-text-primary hover:bg-surface-alt'
                }`}
              >
                {t.label}
              </Link>
            ))}
          </nav>
        </aside>

        {/* Main content */}
        <div className="flex-1 min-w-0">
          {activeTab === 'account'  && <AccountTab user={user} token={token} isSeller={isSeller} updateUser={updateUser} />}
          {activeTab === 'bids'     && <BidsTab user={user} token={token} />}
          {activeTab === 'payments' && <PaymentsTab user={user} token={token} />}
        </div>
      </div>
    </PageContainer>
  )
}

/* ── Account Tab ─────────────────────────────────────────────────── */

function AccountTab({ user, token, isSeller, updateUser }: {
  user: { user_id: string; username: string; email: string; role: string }
  token: string | null
  isSeller: boolean
  updateUser: (patch: { username?: string }) => void
}) {
  const [editing,    setEditing]    = useState(false)
  const [username,   setUsername]   = useState(user.username)
  const [saving,     setSaving]     = useState(false)
  const [success,    setSuccess]    = useState<string | null>(null)
  const [error,      setError]      = useState<string | null>(null)

  const handleSave = async () => {
    if (!token || username.trim().length < 2) return
    setSaving(true)
    setError(null)
    setSuccess(null)
    try {
      await api.users.updateProfile(user.user_id, { username: username.trim() }, token)
      updateUser({ username: username.trim() })
      setEditing(false)
      setSuccess('Username updated')
      setTimeout(() => setSuccess(null), 3000)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update')
    } finally {
      setSaving(false)
    }
  }

  const handleCancel = () => {
    setUsername(user.username)
    setEditing(false)
    setError(null)
  }

  return (
    <>
      <h1 className="font-sans font-semibold text-2xl text-text-primary mb-6">Account Settings</h1>

      {success && <div className="mb-4"><StatusBanner type="success" message={success} /></div>}
      {error && <div className="mb-4"><StatusBanner type="error" message={error} /></div>}

      <Card padding="p-6" className="mb-6">
        <div className="flex flex-col gap-5">
          <div>
            <label className="text-xs font-semibold text-text-secondary uppercase tracking-wide">Username</label>
            {editing ? (
              <div className="flex items-center gap-3 mt-1">
                <TextInput
                  type="text"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  className="max-w-xs"
                />
                <Button variant="primary" size="sm" onClick={handleSave} disabled={saving || username.trim().length < 2}>
                  {saving ? 'Saving…' : 'Save'}
                </Button>
                <Button variant="outline" size="sm" onClick={handleCancel}>Cancel</Button>
              </div>
            ) : (
              <div className="flex items-center gap-3 mt-1">
                <p className="text-text-primary font-medium">{user.username}</p>
                <button
                  onClick={() => setEditing(true)}
                  className="text-brand text-xs font-medium hover:underline"
                >
                  Edit
                </button>
              </div>
            )}
          </div>

          <div>
            <label className="text-xs font-semibold text-text-secondary uppercase tracking-wide">Email</label>
            <p className="text-text-primary font-medium mt-1">{user.email}</p>
          </div>

          <div>
            <label className="text-xs font-semibold text-text-secondary uppercase tracking-wide">Role</label>
            <div className="mt-1">
              <Badge status={user.role === 'seller' ? 'SELLER' : 'BUYER'} />
            </div>
          </div>
        </div>
      </Card>

      {!isSeller && (
        <Card padding="p-6">
          <h2 className="font-sans font-semibold text-lg text-text-primary mb-2">Become a Seller</h2>
          <p className="text-text-secondary text-sm mb-4">
            Upgrade your account to list items and run auctions. Uses the same email and password.
          </p>
          <Link to="/shop/register">
            <Button variant="primary" size="md">Upgrade to Seller</Button>
          </Link>
        </Card>
      )}

      {isSeller && (
        <Card padding="p-6">
          <h2 className="font-sans font-semibold text-lg text-text-primary mb-2">Seller Dashboard</h2>
          <p className="text-text-secondary text-sm mb-4">
            Manage your shops, inventory, and auctions.
          </p>
          <Link to="/seller/dashboard">
            <Button variant="primary" size="md">Go to Dashboard</Button>
          </Link>
        </Card>
      )}
    </>
  )
}

/* ── Bids Tab ────────────────────────────────────────────────────── */

function BidsTab({ user, token }: { user: { user_id: string }; token: string | null }) {
  const [bids,    setBids]    = useState<UserBid[]>([])
  const [loading, setLoading] = useState(true)
  const [error,   setError]   = useState<string | null>(null)

  useEffect(() => {
    if (!token) { setLoading(false); return }
    api.users.bids(user.user_id, token)
      .then(setBids)
      .catch((err) => setError(err instanceof Error ? err.message : 'Failed to load bids'))
      .finally(() => setLoading(false))
  }, [user.user_id, token])

  if (loading) return <Spinner className="py-20" />
  if (error) return <EmptyState message={error} />

  const stats = [
    { label: 'Total Bids',  value: bids.length },
    { label: 'Active',      value: bids.filter((b) => b.status === 'WINNING').length },
    { label: 'Won',         value: bids.filter((b) => b.status === 'WON').length },
  ]

  return (
    <>
      <h1 className="font-sans font-semibold text-2xl text-text-primary mb-6">My Bids</h1>

      <div className="flex gap-4 mb-6">
        {stats.map((s) => <StatCard key={s.label} label={s.label} value={s.value} />)}
      </div>

      {bids.length === 0 ? (
        <EmptyState message="You haven't placed any bids yet." />
      ) : (
        <Card>
          {bids.map((bid, i) => (
            <div
              key={bid.bid_id}
              className={`p-5 flex items-center justify-between ${i !== 0 ? 'border-t border-border' : ''}`}
            >
              <div>
                <p className="text-brand text-xs font-semibold mb-1">{bid.shop_name}</p>
                <p className="font-sans font-medium text-lg text-text-primary mb-1">{bid.item_title}</p>
                <p className="text-text-secondary text-sm">{timeAgo(bid.timestamp)}</p>
              </div>
              <div className="flex flex-col items-end gap-2">
                <p className="font-display text-2xl text-text-primary">{formatCurrency(bid.amount)}</p>
                <Badge status={bid.status} />
                {bid.status === 'OUTBID' && (
                  <Link to={`/auction/${bid.auction_id}`} className="text-brand text-xs font-medium hover:underline">
                    Bid Again →
                  </Link>
                )}
                {bid.status === 'WON' && (
                  <Link to={`/payment/auction/${bid.auction_id}`} className="text-brand text-xs font-medium hover:underline">
                    View Payment →
                  </Link>
                )}
              </div>
            </div>
          ))}
        </Card>
      )}
    </>
  )
}

/* ── Payments Tab ────────────────────────────────────────────────── */

function PaymentsTab({ user, token }: { user: { user_id: string }; token: string | null }) {
  const [payments, setPayments] = useState<Payment[]>([])
  const [loading,  setLoading]  = useState(true)
  const [error,    setError]    = useState<string | null>(null)

  useEffect(() => {
    if (!token) { setLoading(false); return }
    api.payments.listByUser(user.user_id, token)
      .then(setPayments)
      .catch((err) => setError(err instanceof Error ? err.message : 'Failed to load payments'))
      .finally(() => setLoading(false))
  }, [user.user_id, token])

  if (loading) return <Spinner className="py-20" />
  if (error) return <EmptyState message={error} />

  const totalSpent = payments
    .filter((p) => p.status === 'completed')
    .reduce((sum, p) => sum + p.amount, 0)

  return (
    <>
      <div className="flex items-end justify-between mb-6">
        <h1 className="font-sans font-semibold text-2xl text-text-primary">Payments</h1>
        {totalSpent > 0 && (
          <p className="text-text-secondary text-sm">
            Total spent: <span className="font-semibold text-text-primary">{formatCurrency(totalSpent)}</span>
          </p>
        )}
      </div>

      {payments.length === 0 ? (
        <EmptyState message="No payments yet. Win an auction to see your payments here." />
      ) : (
        <Card>
          {payments.map((payment, i) => (
            <div
              key={payment.payment_id}
              className={`p-5 flex items-center justify-between ${i !== 0 ? 'border-t border-border' : ''}`}
            >
              <div className="flex flex-col gap-1">
                <span
                  className={`self-start text-xs font-semibold px-2 py-0.5 rounded-full capitalize ${STATUS_BADGE[payment.status] ?? ''}`}
                >
                  {payment.status}
                </span>
                <p className="text-text-secondary text-xs font-mono mt-1">{payment.auction_id}</p>
                <p className="text-text-secondary text-xs">{new Date(payment.created_at).toLocaleDateString()}</p>
              </div>
              <div className="flex flex-col items-end gap-2">
                <p className="font-display text-2xl text-text-primary">{formatCurrency(payment.amount)}</p>
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
    </>
  )
}
