import type { ReactNode } from 'react'

interface PageContainerProps {
  children: ReactNode
  /** Constrain to max-w-4xl instead of max-w-7xl (used for My Bids, Auth pages) */
  narrow?: boolean
}

export function PageContainer({ children, narrow = false }: PageContainerProps) {
  return (
    <div className={`${narrow ? 'max-w-4xl' : 'max-w-7xl'} w-full mx-auto px-4 sm:px-6 md:px-8 pt-6 pb-10`}>
      {children}
    </div>
  )
}
