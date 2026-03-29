import { UserIcon } from '@/components/icons'

type AvatarSize = 'sm' | 'md' | 'lg' | 'xl'

const SIZE_CLASSES: Record<AvatarSize, string> = {
  sm: 'w-5 h-5',
  md: 'w-8 h-8',
  lg: 'w-10 h-10',
  xl: 'w-24 h-24',
}

const ICON_SIZE: Record<AvatarSize, number> = {
  sm: 12, md: 16, lg: 18, xl: 32,
}

interface AvatarProps {
  src?: string
  alt?: string
  size?: AvatarSize
}

export function Avatar({ src, alt = '', size = 'sm' }: AvatarProps) {
  const cls = `${SIZE_CLASSES[size]} rounded-full border border-border flex-shrink-0`

  if (src) {
    return <img src={src} alt={alt} className={`${cls} object-cover`} />
  }

  return (
    <div className={`${cls} bg-surface flex items-center justify-center text-text-secondary`}>
      <UserIcon width={ICON_SIZE[size]} height={ICON_SIZE[size]} />
    </div>
  )
}
