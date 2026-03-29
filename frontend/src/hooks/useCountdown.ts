import { useState, useEffect } from 'react'

/** Returns the number of milliseconds remaining until endTime, updating every second. */
export function useCountdown(endTime: number): number {
  const [remaining, setRemaining] = useState(endTime - Date.now())

  useEffect(() => {
    const id = setInterval(() => {
      const r = endTime - Date.now()
      setRemaining(r)
      if (r <= 0) clearInterval(id)
    }, 1000)
    return () => clearInterval(id)
  }, [endTime])

  return remaining
}
