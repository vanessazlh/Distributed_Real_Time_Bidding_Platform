import { formatCurrency } from '@/lib/utils'

interface PriceDisplayProps {
  amount: number
  retail?: number
  /** 'card' uses DM Serif Display (text-2xl); 'detail' uses Fraunces (text-5xl) with optional flash */
  size?: 'card' | 'detail'
  flash?: boolean
}

export function PriceDisplay({ amount, retail, size = 'card', flash = false }: PriceDisplayProps) {
  const isDetail = size === 'detail'

  const priceCls = isDetail
    ? `font-display text-5xl ${flash ? 'animate-flash-bid' : 'text-text-primary'}`
    : 'font-serif text-2xl text-text-primary'

  return (
    <div className={`flex items-baseline gap-2.5 ${priceCls}`}>
      {formatCurrency(amount)}
      {retail !== undefined && (
        <span className="text-sm font-sans font-medium text-text-secondary line-through">
          {formatCurrency(retail)}
        </span>
      )}
    </div>
  )
}
