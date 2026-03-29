import { useNavigate } from 'react-router-dom'
import type { Auction } from '@/types'
import { Button } from '@/components/ui'
import { Avatar } from '@/components/ui'
import { ArrowRightIcon } from '@/components/icons'
import { CountdownTimer } from './CountdownTimer'
import { PriceDisplay } from './PriceDisplay'

interface AuctionCardProps {
  auction: Auction
}

export function AuctionCard({ auction }: AuctionCardProps) {
  const navigate  = useNavigate()
  const isClosed  = auction.status === 'CLOSED' || auction.end_time < Date.now()
  const detailUrl = `/auction/${auction.auction_id}`

  return (
    <div
      onClick={() => navigate(detailUrl)}
      className="bg-surface-alt rounded-xl border border-border overflow-hidden cursor-pointer transition-transform hover:-translate-y-1 hover:shadow-lg relative group"
    >
      {/* Closed overlay */}
      {isClosed && (
        <div className="absolute inset-0 bg-white/60 z-10 flex items-center justify-center backdrop-blur-[1px]">
          <span className="bg-text-primary text-white text-sm px-4 py-2 rounded-lg font-sans font-medium tracking-wide">
            AUCTION CLOSED
          </span>
        </div>
      )}

      {/* Image */}
      <div className="h-48 overflow-hidden bg-surface">
        <img
          src={auction.image_url}
          alt={auction.item.title}
          className="w-full h-full object-cover mix-blend-multiply opacity-90 transition-transform duration-500 group-hover:scale-105"
        />
      </div>

      {/* Body */}
      <div className="p-5">
        <div className="flex items-center gap-2 mb-2">
          <Avatar src={auction.shop_logo_url} alt={auction.item.shop_name} size="sm" />
          <span className="text-brand text-xs font-semibold uppercase tracking-wider">
            {auction.item.shop_name}
          </span>
        </div>

        <h3 className="font-sans font-semibold text-lg text-text-primary leading-tight mb-3">
          {auction.item.title}
        </h3>

        <div className="flex justify-between items-end mb-4">
          <div>
            <p className="text-text-secondary text-xs mb-1">Current Bid</p>
            <PriceDisplay amount={auction.current_highest_bid} retail={auction.retail_price} size="card" />
          </div>
          <div className="text-right">
            <CountdownTimer endTime={auction.end_time} />
            <p className="text-text-secondary text-xs mt-1">{auction.bid_count} bids</p>
          </div>
        </div>

        <Button
          variant={isClosed ? 'ghost' : 'action'}
          disabled={isClosed}
          fullWidth
          onClick={(e) => { e.stopPropagation(); navigate(detailUrl) }}
        >
          {isClosed ? 'View Details' : 'Bid Now'}
          <ArrowRightIcon />
        </Button>
      </div>
    </div>
  )
}
