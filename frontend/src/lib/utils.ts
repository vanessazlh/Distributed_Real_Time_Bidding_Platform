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

/** Mask a username for privacy: "yuxin_w" → "yux***" */
export function maskUsername(username: string): string {
  if (!username) return 'anon***'
  return username.substring(0, 3) + '***'
}
