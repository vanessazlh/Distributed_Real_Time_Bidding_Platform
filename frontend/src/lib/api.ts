import type { Auction, User, UserBid } from '@/types'

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
    register: (username: string, email: string, password: string) =>
      request<User>('/users', {
        method: 'POST',
        headers: jsonHeaders(),
        body: JSON.stringify({ username, email, password }),
      }),
  },

  auctions: {
    /** GET /auctions → { auctions: Auction[] } */
    list: () => request<{ auctions: Auction[] }>('/auctions').then((r) => r.auctions ?? []),

    /** GET /auctions/:id → Auction */
    get: (id: string) => request<Auction>(`/auctions/${id}`),

    /** POST /auctions/:id/bid → { success, new_highest_bid } */
    placeBid: (id: string, userId: string, amount: number, token: string) =>
      request<{ success: boolean; new_highest_bid: number }>(`/auctions/${id}/bid`, {
        method: 'POST',
        headers: jsonHeaders(token),
        body: JSON.stringify({ user_id: userId, amount }),
      }),
  },

  users: {
    /** GET /users/:userId/bids → UserBid[] */
    bids: (userId: string, token: string) =>
      request<UserBid[]>(`/users/${userId}/bids`, {
        headers: { Authorization: `Bearer ${token}` },
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
      user_id: (decoded['sub'] ?? decoded['user_id']) as string,
      username: decoded['username'] as string,
      email:    decoded['email']    as string,
    }
  } catch {
    return null
  }
}
