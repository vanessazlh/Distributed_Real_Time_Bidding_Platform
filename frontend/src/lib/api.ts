import type { Auction, AuctionStatus, User, UserBid, Item, Shop } from '@/types'

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

// ── Auction response transform ───────────────────────────────────────────────

/** Shape the auction service actually returns (flat, end_time as RFC3339 string). */
interface BackendAuction {
  auction_id:          string
  item_id:             string
  item_title:          string
  shop_id:             string
  end_time:            string   // RFC3339
  current_highest_bid: number
  status:              string
}

function toAuction(b: BackendAuction): Auction {
  return {
    auction_id:          b.auction_id,
    item: {
      title:     b.item_title ?? '',
      shop_name: '',
      shop_id:   b.shop_id   ?? '',
    },
    current_highest_bid: b.current_highest_bid ?? 0,
    retail_price:        0,
    end_time:            new Date(b.end_time).getTime(),
    status:              (b.status as AuctionStatus) ?? 'OPEN',
    bid_count:           0,
    image_url:           '',
    shop_logo_url:       '',
    description:         '',
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
    create: (payload: { item_id: string; item_title: string; shop_id: string; duration_minutes: number; start_bid: number }, token: string) =>
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
      request<{ bids: UserBid[] }>(`/users/${userId}/bids`, {
        headers: { Authorization: `Bearer ${token}` },
      }).then((r) => r.bids ?? []),
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
