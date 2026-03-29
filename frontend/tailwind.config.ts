import type { Config } from 'tailwindcss'

export default {
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        brand:            '#309898',
        'brand-dark':     '#257A7A',
        surface:          '#FFFCF5',
        'surface-alt':    '#FFFFFF',
        alert:            '#FF9B50',
        critical:         '#C63D2F',
        action:           '#EA5455',
        'action-hover':   '#D84748',
        'timer-3m':       '#FF9F00',
        'timer-2m':       '#F4631E',
        'timer-1m':       '#CB0404',
        'text-primary':   '#1A1A1A',
        'text-secondary': '#6B7280',
        border:           '#E5E7EB',
      },
      fontFamily: {
        sans:    ['"DM Sans"',          'sans-serif'],
        serif:   ['"DM Serif Display"', 'serif'],
        display: ['Fraunces',            'serif'],
      },
      animation: {
        'flash-bid':  'flashBid 1s ease-out',
        'slide-down': 'slideDown 0.3s ease-out forwards',
      },
      keyframes: {
        flashBid: {
          '0%':   { color: '#FF9B50', textShadow: '0 0 12px rgba(255,155,80,0.5)' },
          '100%': { color: '#1A1A1A', textShadow: 'none' },
        },
        slideDown: {
          '0%':   { opacity: '0', transform: 'translateY(-10px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' },
        },
      },
    },
  },
} satisfies Config
