import type { FormEvent } from 'react'
import type { Auction, User } from '@/types'
import { Card, Button, FormField, TextInput, StatusBanner } from '@/components/ui'
import { formatCurrency } from '@/lib/utils'
import { CountdownTimer } from './CountdownTimer'
import { PriceDisplay } from './PriceDisplay'

type BidBannerState = 'WINNING' | 'OUTBID' | null

interface BiddingPanelProps {
  auction: Auction
  highestBid: number
  bidCount: number
  flash: boolean
  banner: BidBannerState
  isClosed: boolean
  user: User | null
  bidInput: string
  onBidInputChange: (value: string) => void
  onPlaceBid: (e: FormEvent) => void
  onSignIn: () => void
}

export function BiddingPanel({
  auction,
  highestBid,
  bidCount,
  flash,
  banner,
  isClosed,
  user,
  bidInput,
  onBidInputChange,
  onPlaceBid,
  onSignIn,
}: BiddingPanelProps) {
  const minNextBid = highestBid + 50

  return (
    <Card padding="p-8">
      {/* Status banner */}
      {banner === 'OUTBID' && (
        <div className="mb-6">
          <StatusBanner
            type="outbid"
            message="You've been outbid!"
            detail={`Current bid is now ${formatCurrency(highestBid)}`}
          />
        </div>
      )}
      {banner === 'WINNING' && (
        <div className="mb-6">
          <StatusBanner type="winning" message="You're currently winning!" detail="Keep an eye on the timer." />
        </div>
      )}

      {/* Current bid */}
      <p className="text-text-secondary text-sm font-medium uppercase tracking-wide mb-1">
        Current Highest Bid
      </p>
      <div className="mb-6">
        <PriceDisplay
          amount={highestBid}
          retail={auction.retail_price}
          size="detail"
          flash={flash}
        />
      </div>

      {/* Time remaining */}
      <div className="bg-surface rounded-lg px-4 py-3 flex items-center justify-between mb-8 border border-border">
        <span className="text-text-secondary text-sm font-medium">Time Remaining</span>
        <CountdownTimer endTime={auction.end_time} className="text-xl" />
      </div>

      <hr className="border-border mb-8" />

      {/* Bid form */}
      <form onSubmit={onPlaceBid} className="flex flex-col gap-4">
        <FormField label={`Your bid (min ${formatCurrency(minNextBid)})`}>
          <TextInput
            type="number"
            step="0.01"
            min={minNextBid / 100}
            required
            disabled={isClosed}
            value={bidInput}
            onChange={(e) => onBidInputChange(e.target.value)}
            placeholder={(minNextBid / 100).toFixed(2)}
            prefix="$"
          />
        </FormField>

        {user ? (
          <Button variant="action" size="lg" disabled={isClosed} type="submit" fullWidth>
            {isClosed ? 'Auction Closed' : 'Place Bid'}
          </Button>
        ) : (
          <Button variant="dark" size="lg" fullWidth type="button" onClick={onSignIn}>
            Sign in to bid
          </Button>
        )}
      </form>

      <p className="text-center text-text-secondary text-xs mt-4">
        {bidCount} total bids placed so far
      </p>
    </Card>
  )
}
