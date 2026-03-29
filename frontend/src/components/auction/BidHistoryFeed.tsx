import type { BidHistoryEntry } from '@/types'
import { Card } from '@/components/ui'
import { Avatar } from '@/components/ui'
import { formatCurrency, timeAgo } from '@/lib/utils'
import { maskUsername } from '@/lib/utils'

interface BidHistoryItemProps {
  bid: BidHistoryEntry
}

function BidHistoryItem({ bid }: BidHistoryItemProps) {
  return (
    <div className="flex justify-between items-center py-2.5 border-b border-border/50 animate-slide-down last:border-0">
      <div className="flex items-center gap-3">
        <Avatar size="md" />
        <span className="font-sans font-medium text-sm text-text-primary">
          {maskUsername(bid.user)}
        </span>
      </div>
      <div className="text-right">
        <p className="font-display font-semibold text-text-primary">{formatCurrency(bid.amount)}</p>
        <p className="text-text-secondary text-xs">{timeAgo(bid.time)}</p>
      </div>
    </div>
  )
}

interface BidHistoryFeedProps {
  bids: BidHistoryEntry[]
}

export function BidHistoryFeed({ bids }: BidHistoryFeedProps) {
  return (
    <Card padding="p-6">
      <h3 className="font-sans font-semibold text-xl text-text-primary border-b border-border pb-3 mb-1">
        Recent Bids
      </h3>
      <div>
        {bids.slice(0, 8).map((bid) => (
          <BidHistoryItem key={bid.id} bid={bid} />
        ))}
      </div>
    </Card>
  )
}
