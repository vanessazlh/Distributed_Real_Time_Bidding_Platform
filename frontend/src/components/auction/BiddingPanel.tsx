import type { FormEvent } from 'react'
import type { Auction, User } from '@/types'
import { Card, Button, FormField, TextInput, StatusBanner } from '@/components/ui'
import { formatCurrency } from '@/lib/utils'
import { CountdownTimer } from './CountdownTimer'
import { PriceDisplay } from './PriceDisplay'

type BidBannerState = 'WINNING' | 'OUTBID' | 'WON' | 'CLOSED' | null

interface BiddingPanelProps {
  auction: Auction
  highestBid: number
  bidCount: number
  flash: boolean
  banner: BidBannerState
  isClosed: boolean
  isPending?: boolean
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
  isPending = false,
  user,
  bidInput,
  onBidInputChange,
  onPlaceBid,
  onSignIn,
}: BiddingPanelProps) {
  // Minimum next bid: if min_increment is set and there is an existing bid, enforce it.
  // Otherwise allow any amount strictly above current (1 cent above).
  const minNextBid = auction.min_increment > 0 && highestBid > 0
    ? highestBid + auction.min_increment
    : highestBid + 1

  return (
    <Card padding="p-8">
      {/* Status banner */}
      {isPending && (
        <div className="mb-6">
          <StatusBanner type="info" message="This auction hasn't started yet." detail="Bidding will open at the scheduled time." />
        </div>
      )}
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
      {banner === 'WON' && (
        <div className="mb-6">
          <StatusBanner type="success" message="You won this auction!" detail={`Final price: ${formatCurrency(highestBid)}`} />
        </div>
      )}
      {banner === 'CLOSED' && (
        <div className="mb-6">
          <StatusBanner type="error" message="This auction has ended." detail="Better luck next time!" />
        </div>
      )}

      {/* Quantity badge */}
      {auction.quantity > 1 && (
        <div className="flex items-center gap-2 mb-4">
          <span className="inline-flex items-center gap-1 bg-brand/10 text-brand text-sm font-semibold px-3 py-1 rounded-full">
            {auction.quantity} winners
          </span>
          <span className="text-text-secondary text-sm">
            Top {auction.quantity} bids win
          </span>
        </div>
      )}

      {/* Current bid */}
      <p className="text-text-secondary text-sm font-medium uppercase tracking-wide mb-1">
        {auction.quantity > 1 ? 'Minimum Bid to Win' : 'Current Highest Bid'}
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

      {/* Bid form — hidden when the user is already winning a single-winner auction */}
      {banner === 'WINNING' && auction.quantity === 1 ? (
        <p className="text-center text-text-secondary text-sm">
          You are the highest bidder. No action needed until the auction closes.
        </p>
      ) : (
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
            {auction.min_increment > 0 && (
              <p className="text-xs text-text-secondary mt-1">
                Minimum raise: {formatCurrency(auction.min_increment)} per bid
              </p>
            )}
          </FormField>

          {user ? (
            <Button variant="action" size="lg" disabled={isClosed} type="submit" fullWidth>
              {isPending ? 'Starting Soon' : isClosed ? 'Auction Closed' : 'Place Bid'}
            </Button>
          ) : (
            <Button variant="dark" size="lg" fullWidth type="button" onClick={onSignIn}>
              Sign in to bid
            </Button>
          )}
        </form>
      )}

      <p className="text-center text-text-secondary text-xs mt-4">
        {bidCount} total bids placed so far
      </p>
    </Card>
  )
}
