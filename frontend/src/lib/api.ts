import type { Auction, AuctionBid, AuctionStatus, Category, User, UserBid, Item, Shop, Payment, StoredNotification } from '@/types'

// ── Error type ───────────────────────────────────────────────────────────────

export class ApiError extends Error {
  constructor(
    public readonly status: number,
    message: string,
    public readonly details?: Record<string, unknown>,
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

// ── Core fetch wrapper ───────────────────────────────────────────────────────

// Translates gin/backend validation messages into plain English.
// Applied centrally in request() so every page benefits automatically.
function friendlyError(raw: string): string {
  // Backend registration conflict messages
  if (raw === 'email already registered')
    return 'You already have an account. Try signing in instead.'
  if (raw === 'account is already a seller')
    return 'You already have an account with this email. Sign in and select Buyer — your seller account can also be used to browse and bid.'

  const m = raw.match(/Field validation for '(\w+)' failed on the '(\w+)' tag/)
  if (m) {
    const field = m[1]
    const tag   = m[2]
    const label = field.replace(/([A-Z])/g, ' $1').trim() // "RetailValue" → "Retail Value"
    switch (tag) {
      case 'required': return `${label} is required.`
      case 'email':    return 'Please enter a valid email address.'
      case 'url':      return 'Please enter a valid URL.'
      case 'min':
        if (field === 'Password') return 'Password must be at least 6 characters.'
        if (field === 'Username') return 'Username must be at least 2 characters.'
        return `${label} is too short.`
      case 'max':      return `${label} is too long.`
      case 'gt':
      case 'gte':      return `${label} must be greater than zero.`
      case 'numeric':  return `${label} must be a number.`
    }
  }
  return raw
}

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(path, options)
  if (!res.ok) {
    let message = `HTTP ${res.status}`
    let details: Record<string, unknown> | undefined
    try {
      const body = await res.json() as Record<string, unknown>
      message = (body.error ?? body.message ?? message) as string
      details = body
    } catch {
      message = await res.text().catch(() => message)
    }
    throw new ApiError(res.status, friendlyError(message), details)
  }
  return res.json() as Promise<T>
}

function jsonHeaders(token?: string | null): HeadersInit {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  if (token) headers['Authorization'] = `Bearer ${token}`
  return headers
}

// ── Bid response transform ───────────────────────────────────────────────────

interface BackendBid {
  bid_id:     string
  auction_id: string
  user_id:    string
  item_title: string
  shop_name:  string
  amount:     number
  timestamp:  string  // RFC3339
  status:     string  // ACCEPTED | OUTBID | WON
}

function toUserBid(b: BackendBid): UserBid {
  return {
    bid_id:     b.bid_id,
    auction_id: b.auction_id,
    item_title: b.item_title ?? '',
    shop_name:  b.shop_name  ?? '',
    amount:     b.amount,
    timestamp:  new Date(b.timestamp).getTime(),
    status:     b.status === 'ACCEPTED' ? 'WINNING'
              : b.status === 'WON'      ? 'WON'
              : 'OUTBID',
  }
}

// ── Auction response transform ───────────────────────────────────────────────

/** Shape the auction service actually returns (flat, end_time as RFC3339 string). */
interface BackendAuction {
  auction_id:          string
  item_id:             string
  item_title:          string
  shop_id:             string
  shop_name:           string
  retail_price:        number
  max_price:           number
  quantity:            number
  image_url:           string
  shop_logo_url:       string
  description:         string
  category?:           string
  pickup_start?:       string   // RFC3339
  pickup_end?:         string   // RFC3339
  end_time:            string   // RFC3339
  current_highest_bid: number
  bid_count:           number
  status:              string
}

function toAuction(b: BackendAuction): Auction {
  return {
    auction_id:          b.auction_id,
    item: {
      title:     b.item_title  ?? '',
      shop_name: b.shop_name   ?? '',
      shop_id:   b.shop_id     ?? '',
    },
    current_highest_bid: b.current_highest_bid ?? 0,
    retail_price:        b.retail_price        ?? 0,
    max_price:           b.max_price           ?? 0,
    quantity:            b.quantity && b.quantity > 0 ? b.quantity : 1,
    end_time:            new Date(b.end_time).getTime(),
    status:              (b.status as AuctionStatus) ?? 'OPEN',
    bid_count:           b.bid_count           ?? 0,
    image_url:           b.image_url           ?? '',
    shop_logo_url:       b.shop_logo_url       ?? '',
    description:         b.description         ?? '',
    category:            (b.category as Category) || undefined,
    pickup_start:        b.pickup_start ? new Date(b.pickup_start).getTime() : undefined,
    pickup_end:          b.pickup_end   ? new Date(b.pickup_end).getTime()   : undefined,
  }
}

// ── API surface ──────────────────────────────────────────────────────────────

export const api = {
  auth: {
    /** POST /auth/login → { token } */
    login: (email: string, password: string) =>
      request<{ token: string }>('/auth/login', {
        method: 'POST',
        headers: jsonHeaders(),
        body: JSON.stringify({ email, password }),
      }),

    /** POST /users → User */
    register: (username: string, email: string, password: string, role: 'buyer' | 'seller' = 'buyer', confirmUpgrade = false) =>
      request<User>('/users', {
        method: 'POST',
        headers: jsonHeaders(),
        body: JSON.stringify({ username, email, password, role, confirm_upgrade: confirmUpgrade || undefined }),
      }),
  },

  auctions: {
    /** GET /auctions → { auctions: Auction[] } */
    list: () =>
      request<{ auctions: BackendAuction[] }>('/auctions')
        .then((r) => (r.auctions ?? []).map(toAuction)),

    /** GET /auctions/:id → Auction */
    get: (id: string) =>
      request<BackendAuction>(`/auctions/${id}`).then(toAuction),

    /** POST /auctions → Auction */
    create: (payload: { item_id: string; item_title: string; shop_id: string; shop_name: string; retail_price: number; max_price?: number; quantity?: number; image_url: string; shop_logo_url: string; description: string; category?: string; duration_minutes: number; start_bid: number; scheduled_start?: string; pickup_start?: string; pickup_end?: string }, token: string) =>
      request<BackendAuction>('/auctions', {
        method: 'POST',
        headers: jsonHeaders(token),
        body: JSON.stringify(payload),
      }).then(toAuction),

    /** POST /auctions/:id/bid → { bid_id, amount, new_highest_bid, status } */
    placeBid: (id: string, userId: string, amount: number, token: string) =>
      request<{ bid_id: string; amount: number; new_highest_bid: number; status: string }>(`/auctions/${id}/bid`, {
        method: 'POST',
        headers: jsonHeaders(token),
        body: JSON.stringify({ user_id: userId, amount }),
      }),

    /** GET /shops/:shopId/auctions → { auctions: Auction[] } */
    listByShop: (shopId: string, token: string) =>
      request<{ auctions: BackendAuction[] }>(`/shops/${shopId}/auctions`, {
        headers: jsonHeaders(token),
      }).then((r) => (r.auctions ?? []).map(toAuction)),

    /** GET /auctions/:id/bids → { bids: AuctionBid[] } */
    bids: (id: string) =>
      request<{ bids: BackendBid[] }>(`/auctions/${id}/bids`)
        .then((r) => (r.bids ?? []).map((b): AuctionBid => ({
          bid_id:    b.bid_id,
          user_id:   b.user_id,
          amount:    b.amount,
          timestamp: new Date(b.timestamp).getTime(),
          status:    b.status === 'ACCEPTED' ? 'WINNING'
                   : b.status === 'WON'      ? 'WON'
                   : 'OUTBID',
        }))),

    /** POST /auctions/:id/close → { message } */
    close: (id: string, token: string) =>
      request<{ message: string }>(`/auctions/${id}/close`, {
        method: 'POST',
        headers: jsonHeaders(token),
      }),
  },

  users: {
    /** GET /users/:userId → User */
    get: (userId: string, token: string) =>
      request<User>(`/users/${userId}`, {
        headers: jsonHeaders(token),
      }),

    /** PUT /users/:userId → { ok } */
    updateProfile: (userId: string, payload: { username: string; avatar_url?: string }, token: string) =>
      request<{ ok: boolean }>(`/users/${userId}`, {
        method: 'PUT',
        headers: jsonHeaders(token),
        body: JSON.stringify(payload),
      }),

    /** GET /users/:userId/bids → { bids: UserBid[] } */
    bids: (userId: string, token: string) =>
      request<{ bids: BackendBid[] }>(`/users/${userId}/bids`, {
        headers: { Authorization: `Bearer ${token}` },
      }).then((r) => (r.bids ?? []).map(toUserBid)),
  },

  shops: {
    /** GET /shops/:shopId → Shop */
    get: (shopId: string) => request<Shop>(`/shops/${shopId}`),

    /** GET /sellers/:userId/shops → { shops: Shop[] } */
    listByOwner: (userId: string, token: string) =>
      request<{ shops: Shop[] }>(`/sellers/${userId}/shops`, {
        headers: jsonHeaders(token),
      }).then((r) => r.shops ?? []),

    /** POST /shops → Shop */
    create: (payload: { name: string; location: string; logo_url?: string }, token: string) =>
      request<Shop>('/shops', {
        method: 'POST',
        headers: jsonHeaders(token),
        body: JSON.stringify(payload),
      }),

    /** PUT /shops/:shopId → Shop */
    update: (shopId: string, payload: { name?: string; location?: string; logo_url?: string }, token: string) =>
      request<Shop>(`/shops/${shopId}`, {
        method: 'PUT',
        headers: jsonHeaders(token),
        body: JSON.stringify(payload),
      }),

    /** GET /shops/:shopId/items → { items: Item[] } */
    items: (shopId: string) =>
      request<{ items: Item[] }>(`/shops/${shopId}/items`).then((r) => r.items ?? []),

    /** POST /shops/:shopId/items → Item */
    createItem: (shopId: string, payload: { title: string; description: string; retail_value: number; image_url?: string; category?: string }, token: string) =>
      request<Item>(`/shops/${shopId}/items`, {
        method: 'POST',
        headers: jsonHeaders(token),
        body: JSON.stringify(payload),
      }),
  },

  notifications: {
    /** GET /notifications → { notifications, unread_count } */
    list: (token: string) =>
      request<{ notifications: StoredNotification[]; unread_count: number }>('/notifications', {
        headers: jsonHeaders(token),
      }),

    /** POST /notifications/read → { ok } */
    markAllRead: (token: string) =>
      request<{ ok: boolean }>('/notifications/read', {
        method: 'POST',
        headers: jsonHeaders(token),
      }),
  },

  uploads: {
    /** POST /uploads (multipart) → url string */
    image: async (file: File, token: string): Promise<string> => {
      const form = new FormData()
      form.append('file', file)

      const res = await fetch('/uploads', {
        method: 'POST',
        headers: { Authorization: `Bearer ${token}` },
        body: form,
      })

      if (!res.ok) {
        let message = `HTTP ${res.status}`
        try {
          const body = await res.json() as Record<string, unknown>
          message = (body.error ?? body.message ?? message) as string
        } catch {
          message = await res.text().catch(() => message)
        }
        throw new ApiError(res.status, friendlyError(message))
      }

      const data = await res.json() as { url: string }
      return data.url
    },
  },

  payments: {
    /** GET /users/:userId/payments → Payment[] */
    listByUser: (userId: string, token: string) =>
      request<Payment[]>(`/users/${userId}/payments`, {
        headers: jsonHeaders(token),
      }).then((r) => r ?? []),

    /** GET /auctions/:auctionId/payment → Payment */
    getByAuction: (auctionId: string, token: string) =>
      request<Payment>(`/auctions/${auctionId}/payment`, {
        headers: jsonHeaders(token),
      }),
  },
}

// ── JWT helpers ──────────────────────────────────────────────────────────────

/** Decode a JWT payload without verification (client-side only). */
export function decodeToken(token: string): Partial<User> | null {
  try {
    const payload = token.split('.')[1]
    const decoded = JSON.parse(atob(payload)) as Record<string, unknown>
    return {
      user_id:  (decoded['sub'] ?? decoded['user_id']) as string,
      username: decoded['username'] as string,
      email:    decoded['email']    as string,
      role:     (decoded['role'] as 'buyer' | 'seller') ?? 'buyer',
    }
  } catch {
    return null
  }
}
