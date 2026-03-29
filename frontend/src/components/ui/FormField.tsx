import type { InputHTMLAttributes, TextareaHTMLAttributes, ReactNode } from 'react'

// ── FormField ────────────────────────────────────────────────────────────────

interface FormFieldProps {
  label?: string
  error?: string
  children: ReactNode
}

export function FormField({ label, error, children }: FormFieldProps) {
  return (
    <div>
      {label && (
        <label className="block text-sm font-medium text-text-primary mb-1.5">
          {label}
        </label>
      )}
      {children}
      {error && <p className="text-xs text-critical mt-1">{error}</p>}
    </div>
  )
}

// ── TextInput ────────────────────────────────────────────────────────────────

interface TextInputProps extends InputHTMLAttributes<HTMLInputElement> {
  prefix?: string
}

export function TextInput({ prefix, disabled, className = '', ...props }: TextInputProps) {
  return (
    <div className="relative">
      {prefix && (
        <span className="absolute left-4 top-1/2 -translate-y-1/2 text-text-secondary font-medium select-none pointer-events-none">
          {prefix}
        </span>
      )}
      <input
        disabled={disabled}
        className={[
          'w-full bg-surface-alt border-2 border-border rounded-lg py-3 pr-4 font-sans text-text-primary',
          prefix ? 'pl-8' : 'pl-4',
          'focus:outline-none focus:border-brand focus:ring-1 focus:ring-brand',
          'disabled:bg-surface disabled:cursor-not-allowed',
          'transition-all',
          className,
        ].join(' ')}
        {...props}
      />
    </div>
  )
}

// ── TextArea ──────────────────────────────────────────────────────────────────

type TextAreaProps = TextareaHTMLAttributes<HTMLTextAreaElement>

export function TextArea({ disabled, className = '', ...props }: TextAreaProps) {
  return (
    <textarea
      disabled={disabled}
      className={[
        'w-full bg-surface-alt border-2 border-border rounded-lg py-3 px-4 font-sans text-text-primary resize-none',
        'focus:outline-none focus:border-brand focus:ring-1 focus:ring-brand',
        'disabled:bg-surface disabled:cursor-not-allowed',
        'transition-all',
        className,
      ].join(' ')}
      {...props}
    />
  )
}
