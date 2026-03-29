import { useCountdown } from '@/hooks/useCountdown'
import { formatCountdown } from '@/lib/utils'
import type { CountdownState } from '@/lib/utils'
import { ClockIcon } from '@/components/icons'

const TIMER_COLOR: Record<CountdownState, string> = {
  '1m':    'text-timer-1m',
  '2m':    'text-timer-2m',
  '3m':    'text-timer-3m',
  normal:  'text-brand',
  closed:  'text-text-secondary',
}

interface CountdownTimerProps {
  endTime: number
  className?: string
}

export function CountdownTimer({ endTime, className = '' }: CountdownTimerProps) {
  const remaining = useCountdown(endTime)
  const { display, state } = formatCountdown(remaining)

  return (
    <div
      className={[
        'flex items-center gap-1.5 font-display font-semibold',
        TIMER_COLOR[state],
        state === '1m' ? 'animate-pulse' : '',
        className,
      ].join(' ')}
    >
      <ClockIcon />
      <span>{remaining <= 0 ? 'CLOSED' : display}</span>
    </div>
  )
}
