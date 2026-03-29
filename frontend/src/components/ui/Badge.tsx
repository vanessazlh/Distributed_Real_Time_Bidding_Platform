import type { BidStatus, AuctionStatus } from '@/types'

type BadgeStatus = BidStatus | AuctionStatus

const BADGE_CLASSES: Record<BadgeStatus, string> = {
  WINNING: 'border-brand text-brand',
  OUTBID:  'border-alert text-alert',
  WON:     'border-green-600 text-green-700',
  LOST:    'border-text-secondary text-text-secondary',
  OPEN:    'border-brand text-brand',
  CLOSED:  'border-text-secondary text-text-secondary',
}

interface BadgeProps {
  status: BadgeStatus
}

export function Badge({ status }: BadgeProps) {
  return (
    <span className={`px-3 py-1 text-xs font-semibold rounded-full border ${BADGE_CLASSES[status]}`}>
      {status}
    </span>
  )
}
