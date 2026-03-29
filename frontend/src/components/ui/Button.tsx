import type { ButtonHTMLAttributes, ReactNode } from 'react'

export type ButtonVariant = 'primary' | 'action' | 'dark' | 'ghost' | 'outline'
export type ButtonSize    = 'sm' | 'md' | 'lg'

const VARIANT_CLASSES: Record<ButtonVariant, string> = {
  primary: 'bg-brand text-white hover:bg-brand-dark',
  action:  'bg-action text-white hover:bg-action-hover hover:shadow-lg hover:shadow-action/30 active:scale-[0.98]',
  dark:    'bg-text-primary text-white hover:bg-black',
  ghost:   'text-text-primary hover:text-brand bg-transparent',
  outline: 'border border-border bg-transparent text-text-primary hover:border-brand hover:text-brand',
}

const SIZE_CLASSES: Record<ButtonSize, string> = {
  sm: 'px-3 py-1.5 text-sm',
  md: 'px-5 py-2.5 text-base',
  lg: 'py-4 text-lg',
}

const DISABLED_CLASSES = 'bg-border text-text-secondary cursor-not-allowed hover:bg-border hover:shadow-none'

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant
  size?: ButtonSize
  fullWidth?: boolean
  children: ReactNode
}

export function Button({
  variant = 'primary',
  size = 'md',
  fullWidth = false,
  disabled = false,
  children,
  className = '',
  ...props
}: ButtonProps) {
  const variantCls = disabled ? DISABLED_CLASSES : VARIANT_CLASSES[variant]
  const sizeCls    = SIZE_CLASSES[size]

  return (
    <button
      disabled={disabled}
      className={[
        'font-sans font-semibold rounded-lg transition-all',
        'flex items-center justify-center gap-2',
        variantCls,
        sizeCls,
        fullWidth ? 'w-full' : '',
        className,
      ].join(' ')}
      {...props}
    >
      {children}
    </button>
  )
}
