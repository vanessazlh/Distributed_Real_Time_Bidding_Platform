import { useState, useEffect } from 'react'
import { Link, useParams, useNavigate } from 'react-router-dom'
import type { UserBid, Payment, PaymentStatus } from '@/types'
import { ApiError } from '@/lib/api'
import { useAuth } from '@/context/AuthContext'
import { api } from '@/lib/api'
import { formatCurrency, timeAgo } from '@/lib/utils'
import { Card, StatCard, EmptyState, Button, Spinner, TextInput, StatusBanner, Badge } from '@/components/ui'
import { PageContainer } from '@/components/layout'
import { UserIcon, ChevronLeftIcon } from '@/components/icons'

type Tab = 'account' | 'bids'

const ALL_TABS: { key: Tab; label: string; path: string; sellerOnly?: false; buyerOnly?: true }[] = [
  { key: 'account', label: 'Account',  path: '/profile' },
  { key: 'bids',    label: 'My Bids',  path: '/profile/bids', buyerOnly: true },
]

const PAYMENT_STATUS_STYLE: Record<PaymentStatus, string> = {
  pending:    'text-yellow-600',
  processing: 'text-blue-600',
  completed:  'text-green-600',
  failed:     'text-red-600',
  refunded:   'text-text-secondary',
}

const PAYMENT_STATUS_LABEL: Record<PaymentStatus, string> = {
  pending:    'Payment Pending',
  processing: 'Payment Processing',
  completed:  'Paid',
  failed:     'Payment Failed',
  refunded:   'Refunded',
}

function Avatar({ url, size = 16 }: { url?: string; size?: number }) {
  const px = size * 4
  if (url) {
    return (
      <img
        src={url}
        alt="avatar"
        className="rounded-full object-cover"
        style={{ width: px, height: px }}
      />
    )
  }
  return (
    <div
      className="rounded-full bg-brand/10 flex items-center justify-center shrink-0"
      style={{ width: px, height: px }}
    >
      <UserIcon width={px * 0.5} height={px * 0.5} className="text-brand" />
    </div>
  )
}

export default function ProfilePage() {
  const { tab } = useParams<{ tab?: string }>()
  const { user, token, updateUser } = useAuth()
  const navigate = useNavigate()

  const isSeller  = user?.role === 'seller'
  const tabs      = ALL_TABS.filter((t) => !t.buyerOnly || !isSeller)
  const activeTab: Tab = tab === 'bids' && !isSeller ? 'bids' : 'account'

  // Redirect sellers away from the bids tab if they land on it directly
  useEffect(() => {
    if (isSeller && tab === 'bids') navigate('/profile', { replace: true })
  }, [isSeller, tab, navigate])

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
      <div className="max-w-5xl mx-auto mb-6">
        <Link
          to={isSeller ? '/seller/dashboard' : '/'}
          className="inline-flex items-center gap-1 text-text-secondary hover:text-brand text-base font-medium transition-colors"
        >
          <ChevronLeftIcon /> {isSeller ? 'Seller Dashboard' : 'All Auctions'}
        </Link>
      </div>
      <div className="flex gap-8 max-w-5xl mx-auto">
        {/* Sidebar */}
        <aside className="w-56 shrink-0">
          <div className="flex flex-col items-center gap-3 mb-8 pt-4">
            <Avatar url={user.avatar_url} size={16} />
            <p className="font-sans font-semibold text-base text-text-primary">{user.username}</p>
          </div>
          <nav className="flex flex-col gap-1">
            {tabs.map((t) => (
              <Link
                key={t.key}
                to={t.path}
                className={`px-4 py-2.5 rounded-lg text-base font-medium text-center transition-colors ${
                  activeTab === t.key
                    ? 'bg-brand/10 text-brand'
                    : 'text-text-secondary hover:text-text-primary hover:bg-brand/5'
                }`}
              >
                {t.label}
              </Link>
            ))}
            {isSeller && (
              <>
                <div className="my-2 border-t border-border" />
                <Link
                  to="/seller/dashboard"
                  className="px-4 py-2.5 rounded-lg text-base font-medium text-center text-text-secondary hover:text-text-primary hover:bg-brand/5 transition-colors"
                >
                  Seller Dashboard
                </Link>
              </>
            )}
          </nav>
        </aside>

        {/* Main content */}
        <div className="flex-1 min-w-0">
          {activeTab === 'account' && <AccountTab user={user} token={token} updateUser={updateUser} />}
          {activeTab === 'bids'    && <BidsTab user={user} token={token} />}
        </div>
      </div>
    </PageContainer>
  )
}

/* ── Account Tab ─────────────────────────────────────────────────── */

function AccountTab({ user, token, updateUser }: {
  user: { user_id: string; username: string; email: string; avatar_url?: string }
  token: string | null
  updateUser: (patch: { username?: string; avatar_url?: string }) => void
}) {
  const [editing,   setEditing]   = useState(false)
  const [username,  setUsername]  = useState(user.username)
  const [avatarURL, setAvatarURL] = useState(user.avatar_url ?? '')
  const [saving,    setSaving]    = useState(false)
  const [success,   setSuccess]   = useState<string | null>(null)
  const [error,     setError]     = useState<string | null>(null)

  const handleSave = async () => {
    if (!token || username.trim().length < 2) return
    setSaving(true)
    setError(null)
    setSuccess(null)
    try {
      await api.users.updateProfile(
        user.user_id,
        { username: username.trim(), avatar_url: avatarURL.trim() || undefined },
        token,
      )
      updateUser({ username: username.trim(), avatar_url: avatarURL.trim() || undefined })
      setEditing(false)
      setSuccess('Profile updated')
      setTimeout(() => setSuccess(null), 3000)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update')
    } finally {
      setSaving(false)
    }
  }

  const handleCancel = () => {
    setUsername(user.username)
    setAvatarURL(user.avatar_url ?? '')
    setEditing(false)
    setError(null)
  }

  return (
    <>
      <h1 className="font-sans font-semibold text-2xl text-text-primary mb-6">Account Settings</h1>

      {success && <div className="mb-4"><StatusBanner type="success" message={success} /></div>}
      {error   && <div className="mb-4"><StatusBanner type="error"   message={error}   /></div>}

      <Card padding="p-8 md:p-12">
        <div className="flex flex-col gap-7 w-full">

          {/* Avatar */}
          <div className="flex flex-col gap-2">
            <label className="text-sm font-semibold text-text-secondary uppercase tracking-wide">Avatar</label>
            <div className="flex items-center gap-4">
              <Avatar url={editing ? (avatarURL.trim() || undefined) : user.avatar_url} size={12} />
              {editing && (
                <TextInput
                  type="url"
                  value={avatarURL}
                  onChange={(e) => setAvatarURL(e.target.value)}
                  placeholder="https://example.com/avatar.png"
                  className="flex-1 max-w-sm"
                />
              )}
            </div>
          </div>

          {/* Username */}
          <div className="flex flex-col gap-2">
            <label className="text-sm font-semibold text-text-secondary uppercase tracking-wide">Username</label>
            {editing ? (
              <TextInput
                type="text"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                className="max-w-xs"
              />
            ) : (
              <p className="text-text-primary text-base font-medium">{user.username}</p>
            )}
          </div>

          {/* Email */}
          <div className="flex flex-col gap-2">
            <label className="text-sm font-semibold text-text-secondary uppercase tracking-wide">Email</label>
            <p className="text-text-primary text-base font-medium">{user.email}</p>
          </div>

          {/* Actions */}
          <div className="flex items-center justify-center gap-3 pt-2">
            {editing ? (
              <>
                <Button variant="primary" size="md" onClick={handleSave} disabled={saving || username.trim().length < 2}>
                  {saving ? 'Saving…' : 'Save'}
                </Button>
                <Button variant="outline" size="md" onClick={handleCancel}>Cancel</Button>
              </>
            ) : (
              <Button variant="primary" size="md" onClick={() => setEditing(true)}>
                Edit profile
              </Button>
            )}
          </div>

        </div>
      </Card>
    </>
  )
}

/* ── Bids Tab ────────────────────────────────────────────────────── */

function BidsTab({ user, token }: { user: { user_id: string }; token: string | null }) {
  const [bids,              setBids]              = useState<UserBid[]>([])
  const [paymentsByAuction, setPaymentsByAuction] = useState<Map<string, PaymentStatus>>(new Map())
  const [loading,           setLoading]           = useState(true)
  const [error,             setError]             = useState<string | null>(null)

  useEffect(() => {
    if (!token) { setLoading(false); return }
    Promise.all([
      api.users.bids(user.user_id, token),
      api.payments.listByUser(user.user_id, token).catch(() => [] as Payment[]),
    ])
      .then(([loadedBids, payments]) => {
        setBids(loadedBids)
        const map = new Map<string, PaymentStatus>()
        for (const p of payments) map.set(p.auction_id, p.status)
        setPaymentsByAuction(map)
      })
      .catch((err) => {
        if (err instanceof ApiError && err.status === 401) {
          setError('Your session has expired. Please sign in again.')
        } else {
          setError(err instanceof Error ? err.message : 'Failed to load bids')
        }
      })
      .finally(() => setLoading(false))
  }, [user.user_id, token])

  if (loading) return <Spinner className="py-20" />
  if (error)   return <EmptyState message={error} />

  const stats = [
    { label: 'Total Bids', value: bids.length },
    { label: 'Active',     value: bids.filter((b) => b.status === 'WINNING').length },
    { label: 'Won',        value: bids.filter((b) => b.status === 'WON').length },
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
        <div className="flex flex-col gap-3">
          {bids.map((bid) => {
            const paymentStatus = bid.status === 'WON' ? paymentsByAuction.get(bid.auction_id) : undefined
            const hasActions = bid.status === 'WON'
            return (
              <Link
                key={bid.bid_id}
                to={`/auction/${bid.auction_id}`}
                className="px-5 py-5 flex flex-col bg-surface-alt rounded-xl border border-border shadow-sm cursor-pointer transition-transform hover:-translate-y-1 hover:shadow-lg"
              >
                {/* Main row */}
                <div className="flex items-center justify-between">
                  <div>
                    <Badge status={bid.status} />
                    <p className="text-brand text-sm font-semibold mt-1.5 mb-1">{bid.shop_name}</p>
                    <p className="font-sans font-medium text-lg text-text-primary mb-1">{bid.item_title}</p>
                    <div className="flex items-center gap-4">
                      <p className="text-text-secondary text-base">{timeAgo(bid.timestamp)}</p>
                      {hasActions && (
                        <>
                          <Link
                            to={`/payment/auction/${bid.auction_id}`}
                            onClick={(e) => e.stopPropagation()}
                            className="text-brand text-sm font-medium hover:underline"
                          >
                            View Payment →
                          </Link>
                          <Link
                            to={`/reviews/new?auction_id=${bid.auction_id}&shop_id=${bid.shop_id}&shop_name=${encodeURIComponent(bid.shop_name)}&item_title=${encodeURIComponent(bid.item_title)}`}
                            onClick={(e) => e.stopPropagation()}
                            className="text-text-secondary text-sm font-medium hover:text-brand hover:underline transition-colors"
                          >
                            Leave a Review →
                          </Link>
                        </>
                      )}
                    </div>
                  </div>
                  <div className="flex flex-col items-end gap-1">
                    <p className="font-display text-2xl text-text-primary">{formatCurrency(bid.amount)}</p>
                    {paymentStatus && (
                      <span className={`text-sm font-medium ${PAYMENT_STATUS_STYLE[paymentStatus]}`}>
                        {PAYMENT_STATUS_LABEL[paymentStatus]}
                      </span>
                    )}
                  </div>
                </div>
              </Link>
            )
          })}
        </div>
      )}
    </>
  )
}
