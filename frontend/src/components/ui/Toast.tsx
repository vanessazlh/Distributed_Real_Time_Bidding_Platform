import { useNavigate } from 'react-router-dom'
import { useNotifications } from '@/context/NotificationContext'

export function NotificationToast() {
  const { latestToast, dismissToast } = useNotifications()
  const navigate = useNavigate()

  if (!latestToast) return null

  const bgColor =
    latestToast.type === 'outbid' ? 'bg-amber-50 border-amber-200' :
    latestToast.type === 'won'    ? 'bg-green-50 border-green-200' :
    'bg-gray-50 border-gray-200'

  const dotColor =
    latestToast.type === 'outbid' ? 'bg-amber-500' :
    latestToast.type === 'won'    ? 'bg-green-500' :
    'bg-gray-400'

  return (
    <div className="fixed bottom-6 right-6 z-[100] animate-slide-up">
      <div
        className={`${bgColor} border rounded-xl shadow-lg p-4 max-w-sm cursor-pointer transition-transform hover:scale-[1.02]`}
        onClick={() => {
          dismissToast()
          navigate(latestToast.link)
        }}
      >
        <div className="flex items-start gap-3">
          <div className={`mt-1 w-2.5 h-2.5 rounded-full flex-shrink-0 ${dotColor}`} />
          <div className="flex-1 min-w-0">
            <p className="text-sm font-medium text-text-primary leading-snug">{latestToast.message}</p>
            {latestToast.item_title && (
              <p className="text-xs text-text-secondary mt-0.5">{latestToast.item_title}</p>
            )}
          </div>
          <button
            onClick={(e) => {
              e.stopPropagation()
              dismissToast()
            }}
            className="text-text-secondary hover:text-text-primary text-lg leading-none flex-shrink-0 -mt-0.5"
          >
            &times;
          </button>
        </div>
      </div>
    </div>
  )
}
