import type { ReactNode } from 'react'

interface PageContainerProps {
  children: ReactNode
  /** Constrain to max-w-4xl instead of max-w-7xl (used for My Bids, Auth pages) */
  narrow?: boolean
}

export function PageContainer({ children, narrow = false }: PageContainerProps) {
  return (
    <div className={`${narrow ? 'max-w-4xl' : 'max-w-7xl'} w-full mx-auto px-8 py-10`}>
      {children}
    </div>
  )
}
