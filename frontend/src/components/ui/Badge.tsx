import type { BidStatus, AuctionStatus } from '@/types'

type BadgeStatus = BidStatus | AuctionStatus | 'BUYER' | 'SELLER'

const BADGE_CLASSES: Record<BadgeStatus, string> = {
  WINNING: 'border-brand text-brand',
  OUTBID:  'border-alert text-alert',
  WON:     'border-green-600 text-green-700',
  LOST:    'border-text-secondary text-text-secondary',
  PENDING: 'border-blue-400 text-blue-600',
  OPEN:    'border-brand text-brand',
  CLOSED:  'border-text-secondary text-text-secondary',
  BUYER:   'border-brand text-brand',
  SELLER:  'border-green-600 text-green-700',
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
