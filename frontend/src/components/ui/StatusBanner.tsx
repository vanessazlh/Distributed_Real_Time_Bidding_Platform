export type BannerType = 'outbid' | 'winning' | 'success' | 'error'

interface BannerStyle {
  border: string
  bg: string
  textColor: string
  icon: string
}

const BANNER_STYLES: Record<BannerType, BannerStyle> = {
  outbid:  { border: 'border-alert',    bg: 'bg-orange-50', textColor: 'text-alert',    icon: '⚠️' },
  winning: { border: 'border-brand',    bg: 'bg-teal-50',   textColor: 'text-brand',    icon: '🎉' },
  success: { border: 'border-brand',    bg: 'bg-teal-50',   textColor: 'text-brand',    icon: '✓'  },
  error:   { border: 'border-critical', bg: 'bg-red-50',    textColor: 'text-critical', icon: '✕'  },
}

interface StatusBannerProps {
  type: BannerType
  message: string
  detail?: string
}

export function StatusBanner({ type, message, detail }: StatusBannerProps) {
  const s = BANNER_STYLES[type]
  return (
    <div className={`border-l-4 ${s.border} ${s.bg} px-4 py-3 rounded-lg flex items-start gap-3`}>
      <span className={`${s.textColor} mt-0.5 text-base`}>{s.icon}</span>
      <div>
        <p className={`font-sans font-medium ${s.textColor} text-sm`}>{message}</p>
        {detail && <p className="text-text-primary text-xs mt-0.5">{detail}</p>}
      </div>
    </div>
  )
}
