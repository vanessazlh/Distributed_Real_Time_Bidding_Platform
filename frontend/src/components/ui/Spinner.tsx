interface SpinnerProps {
  className?: string
}

export function Spinner({ className = '' }: SpinnerProps) {
  return (
    <div className={`flex items-center justify-center ${className}`}>
      <div className="w-8 h-8 border-2 border-border border-t-brand rounded-full animate-spin" />
    </div>
  )
}
