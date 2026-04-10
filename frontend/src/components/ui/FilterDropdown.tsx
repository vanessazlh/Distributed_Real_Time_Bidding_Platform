import { useState, useRef, useEffect } from 'react'

export interface FilterOption {
  value: string
  label: string
}

interface FilterDropdownProps {
  label: string
  options: FilterOption[]
  value: string
  onChange: (value: string) => void
}

export function FilterDropdown({ label, options, value, onChange }: FilterDropdownProps) {
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', handleClick)
    return () => document.removeEventListener('mousedown', handleClick)
  }, [])

  const selected = options.find((o) => o.value === value)
  const isDefault = value === options[0]?.value

  return (
    <div ref={ref} className="relative">
      <button
        onClick={() => setOpen((o) => !o)}
        className={[
          'flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-sm font-medium transition-colors',
          'border border-border hover:border-brand',
          isDefault ? 'text-text-secondary' : 'text-brand border-brand bg-brand/5',
        ].join(' ')}
      >
        <span className="text-xs">{label}</span>
        <span className={isDefault ? '' : 'font-semibold'}>{selected?.label ?? value}</span>
        <svg className={`w-3.5 h-3.5 transition-transform ${open ? 'rotate-180' : ''}`} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
          <path strokeLinecap="round" strokeLinejoin="round" d="M19 9l-7 7-7-7" />
        </svg>
      </button>

      {open && (
        <div className="absolute right-0 mt-1 min-w-[140px] bg-surface-alt rounded-lg border border-border shadow-lg z-20 py-1">
          {options.map((opt) => (
            <button
              key={opt.value}
              onClick={() => { onChange(opt.value); setOpen(false) }}
              className={[
                'w-full text-left px-3 py-2 text-sm transition-colors',
                opt.value === value
                  ? 'text-brand font-semibold bg-brand/5'
                  : 'text-text-primary hover:bg-surface',
              ].join(' ')}
            >
              {opt.label}
            </button>
          ))}
        </div>
      )}
    </div>
  )
}
