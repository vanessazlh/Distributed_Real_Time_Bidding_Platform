import type { HTMLAttributes, ReactNode } from 'react'

interface CardProps extends HTMLAttributes<HTMLDivElement> {
  children: ReactNode
  padding?: string
}

export function Card({ children, padding = '', className = '', ...props }: CardProps) {
  return (
    <div
      className={`bg-surface-alt rounded-xl border border-border shadow-sm ${padding} ${className}`}
      {...props}
    >
      {children}
    </div>
  )
}
