import type { Auction, AuctionStatus, User, UserBid, Item, Shop, Payment } from '@/types'

// ── Error type ───────────────────────────────────────────────────────────────

export class ApiError extends Error {
  constructor(public readonly status: number, message: string) {
    super(message)
    this.name = 'ApiError'
  }
}

// ── Core fetch wrapper ───────────────────────────────────────────────────────

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(path, options)
  if (!res.ok) {
    const text = await res.text().catch(() => `HTTP ${res.status}`)
    throw new ApiError(res.status, text)
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
  amount:     number
  timestamp:  string  // RFC3339
  status:     string  // ACCEPTED | OUTBID
}

function toUserBid(b: BackendBid): UserBid {
  return {
    bid_id:     b.bid_id,
    auction_id: b.auction_id,
    item_title: '',
    shop_name:  '',
    amount:     b.amount,
    timestamp:  new Date(b.timestamp).getTime(),
    status:     b.status === 'ACCEPTED' ? 'WINNING' : 'OUTBID',
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
  image_url:           string
  shop_logo_url:       string
  description:         string
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
    end_time:            new Date(b.end_time).getTime(),
    status:              (b.status as AuctionStatus) ?? 'OPEN',
    bid_count:           b.bid_count           ?? 0,
    image_url:           b.image_url           ?? '',
    shop_logo_url:       b.shop_logo_url       ?? '',
    description:         b.description         ?? '',
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
    register: (username: string, email: string, password: string, role: 'buyer' | 'seller' = 'buyer') =>
      request<User>('/users', {
        method: 'POST',
        headers: jsonHeaders(),
        body: JSON.stringify({ username, email, password, role }),
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
    create: (payload: { item_id: string; item_title: string; shop_id: string; shop_name: string; retail_price: number; image_url: string; shop_logo_url: string; description: string; duration_minutes: number; start_bid: number }, token: string) =>
      request<BackendAuction>('/auctions', {
        method: 'POST',
        headers: jsonHeaders(token),
        body: JSON.stringify(payload),
      }).then(toAuction),

    /** POST /auctions/:id/bid → { success, new_highest_bid } */
    placeBid: (id: string, userId: string, amount: number, token: string) =>
      request<{ success: boolean; new_highest_bid: number }>(`/auctions/${id}/bid`, {
        method: 'POST',
        headers: jsonHeaders(token),
        body: JSON.stringify({ user_id: userId, amount }),
      }),
  },

  users: {
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

    /** GET /shops/:shopId/items → { items: Item[] } */
    items: (shopId: string) =>
      request<{ items: Item[] }>(`/shops/${shopId}/items`).then((r) => r.items ?? []),

    /** POST /shops/:shopId/items → Item */
    createItem: (shopId: string, payload: { title: string; description: string; retail_value: number; image_url?: string }, token: string) =>
      request<Item>(`/shops/${shopId}/items`, {
        method: 'POST',
        headers: jsonHeaders(token),
        body: JSON.stringify(payload),
      }),
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
