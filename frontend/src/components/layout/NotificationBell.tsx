import { useState, useRef, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { useNotifications } from '@/context/NotificationContext'
import { BellIcon } from '@/components/icons'
import { formatCurrency } from '@/lib/utils'

export function NotificationBell() {
  const { notifications, unreadCount, markAllRead } = useNotifications()
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)
  const navigate = useNavigate()

  // Close dropdown on outside click
  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false)
      }
    }
    if (open) document.addEventListener('mousedown', handleClick)
    return () => document.removeEventListener('mousedown', handleClick)
  }, [open])

  return (
    <div ref={ref} className="relative">
      <button
        onClick={() => setOpen((prev) => !prev)}
        className="relative p-2 rounded-lg hover:bg-gray-100 transition-colors"
        aria-label="Notifications"
      >
        <BellIcon width={20} height={20} />
        {unreadCount > 0 && (
          <span className="absolute -top-0.5 -right-0.5 bg-red-500 text-white text-[10px] font-bold rounded-full min-w-[18px] h-[18px] flex items-center justify-center px-1">
            {unreadCount > 9 ? '9+' : unreadCount}
          </span>
        )}
      </button>

      {open && (
        <div className="absolute right-0 top-full mt-2 w-80 bg-white rounded-xl border border-border shadow-lg z-50 overflow-hidden">
          <div className="px-4 py-3 border-b border-border flex items-center justify-between">
            <span className="font-sans font-semibold text-sm text-text-primary">Notifications</span>
            {unreadCount > 0 && (
              <button
                onClick={() => markAllRead()}
                className="text-xs text-brand hover:underline font-medium"
              >
                Mark all as read
              </button>
            )}
          </div>

          <div className="max-h-80 overflow-y-auto">
            {notifications.length === 0 ? (
              <p className="text-center text-text-secondary text-sm py-8">No notifications yet</p>
            ) : (
              notifications.map((n) => (
                <button
                  key={n.id + n.created_at}
                  onClick={() => {
                    setOpen(false)
                    navigate(n.link)
                  }}
                  className={`w-full text-left px-4 py-3 hover:bg-gray-50 transition-colors border-b border-border last:border-b-0 ${
                    !n.read ? 'bg-brand/5' : ''
                  }`}
                >
                  <div className="flex items-start gap-3">
                    <div className={`mt-1 w-2 h-2 rounded-full flex-shrink-0 ${
                      n.type === 'outbid' ? 'bg-amber-500' :
                      n.type === 'won'    ? 'bg-green-500' :
                      'bg-gray-400'
                    }`} />
                    <div className="flex-1 min-w-0">
                      <p className="text-sm text-text-primary leading-snug">{n.message}</p>
                      {n.amount > 0 && (
                        <p className="text-xs text-text-secondary mt-0.5">{formatCurrency(n.amount)}</p>
                      )}
                      <p className="text-xs text-text-secondary mt-1">
                        {formatRelativeTime(n.created_at)}
                      </p>
                    </div>
                  </div>
                </button>
              ))
            )}
          </div>
        </div>
      )}
    </div>
  )
}

function formatRelativeTime(timestampMs: number): string {
  const diff = Date.now() - timestampMs
  const minutes = Math.floor(diff / 60000)
  if (minutes < 1) return 'Just now'
  if (minutes < 60) return `${minutes}m ago`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours}h ago`
  const days = Math.floor(hours / 24)
  return `${days}d ago`
}
