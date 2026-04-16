/** Format cents as a dollar string: 1200 → "$12.00" */
export function formatCurrency(cents: number): string {
  return `$${(cents / 100).toFixed(2)}`
}

export type CountdownState = '1m' | '2m' | '3m' | 'normal' | 'closed'

export interface CountdownResult {
  display: string
  state: CountdownState
}

/** Convert a remaining-ms value into a display string and urgency state */
export function formatCountdown(ms: number): CountdownResult {
  if (ms <= 0) return { display: '00s', state: 'closed' }
  const totalSecs = Math.floor(ms / 1000)
  const mins = Math.floor(totalSecs / 60)
  const secs = totalSecs % 60
  const display = mins > 0
    ? `${mins}m ${String(secs).padStart(2, '0')}s`
    : `${secs}s`
  const state: CountdownState =
    totalSecs < 60  ? '1m'    :
    totalSecs < 120 ? '2m'    :
    totalSecs < 180 ? '3m'    : 'normal'
  return { display, state }
}

/** Human-readable relative time: "3s ago", "4m ago" */
export function timeAgo(ts: number): string {
  const secs = Math.round((Date.now() - ts) / 1000)
  if (secs < 60)   return `${secs}s ago`
  if (secs < 3600) return `${Math.floor(secs / 60)}m ago`
  return `${Math.floor(secs / 3600)}h ago`
}

/** Format a pickup window from epoch ms: "Today, 5:00 – 6:00 PM" or "Apr 10, 5:00 – 6:00 PM" */
export function formatPickupWindow(startMs: number, endMs: number): string {
  const s = new Date(startMs)
  const e = new Date(endMs)
  const today = new Date()
  const isToday =
    s.getFullYear() === today.getFullYear() &&
    s.getMonth()    === today.getMonth()    &&
    s.getDate()     === today.getDate()
  const dateStr = isToday
    ? 'Today'
    : s.toLocaleDateString(undefined, { month: 'short', day: 'numeric' })
  const startTime = s.toLocaleTimeString(undefined, { hour: 'numeric', minute: '2-digit' })
  const endTime   = e.toLocaleTimeString(undefined, { hour: 'numeric', minute: '2-digit' })
  return `${dateStr}, ${startTime} – ${endTime}`
}

/** Check if a pickup window overlaps a time-of-day range (hours 0–23) */
export function pickupOverlaps(startMs: number, endMs: number, rangeStart: number, rangeEnd: number): boolean {
  const startHour = new Date(startMs).getHours()
  const endDate = new Date(endMs)
  // If end is exactly on the hour boundary (e.g. 17:00), treat as previous hour
  const endHour = endDate.getMinutes() === 0 && endDate.getSeconds() === 0
    ? endDate.getHours() - 1
    : endDate.getHours()
  // Overlap: start <= rangeEnd AND end >= rangeStart
  return startHour <= rangeEnd && endHour >= rangeStart
}

/** Distance between two lat/lng points in kilometres (Haversine formula) */
export function haversineKm(lat1: number, lng1: number, lat2: number, lng2: number): number {
  const R = 6371
  const dLat = ((lat2 - lat1) * Math.PI) / 180
  const dLng = ((lng2 - lng1) * Math.PI) / 180
  const a =
    Math.sin(dLat / 2) * Math.sin(dLat / 2) +
    Math.cos((lat1 * Math.PI) / 180) *
      Math.cos((lat2 * Math.PI) / 180) *
      Math.sin(dLng / 2) *
      Math.sin(dLng / 2)
  return R * 2 * Math.atan2(Math.sqrt(a), Math.sqrt(1 - a))
}

/** Mask a username for privacy: "yuxin_w" → "yux***" */
export function maskUsername(username: string): string {
  if (!username) return 'anon***'
  return username.substring(0, 3) + '***'
}
