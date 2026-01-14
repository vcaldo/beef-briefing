/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        // Telegram theme colors (use CSS variables for runtime theming)
        tg: {
          bg: 'var(--tg-theme-bg-color)',
          text: 'var(--tg-theme-text-color)',
          hint: 'var(--tg-theme-hint-color)',
          link: 'var(--tg-theme-link-color)',
          button: 'var(--tg-theme-button-color)',
          'button-text': 'var(--tg-theme-button-text-color)',
          'secondary-bg': 'var(--tg-theme-secondary-bg-color)',
        },
        // Arena game-specific colors
        arena: {
          dark: '#1a1625',
          purple: '#7c3aed',
          pink: '#ec4899',
          cyan: '#06b6d4',
          gold: '#eab308',
          orange: '#f97316',
        },
        // HP bar colors
        hp: {
          high: '#22c55e',
          medium: '#eab308',
          low: '#ef4444',
        },
        // Medal colors
        medal: {
          gold: '#ffd700',
          silver: '#c0c0c0',
          bronze: '#cd7f32',
        },
      },
      fontFamily: {
        display: ['Outfit', 'system-ui', 'sans-serif'],
        body: ['Inter', 'system-ui', 'sans-serif'],
        mono: ['JetBrains Mono', 'monospace'],
      },
      animation: {
        'bounce-slow': 'bounce 3s infinite',
        'pulse-slow': 'pulse 3s cubic-bezier(0.4, 0, 0.6, 1) infinite',
        'shake': 'shake 0.5s cubic-bezier(.36,.07,.19,.97) both',
        'slide-up': 'slideUp 0.3s ease-out',
        'fade-in': 'fadeIn 0.2s ease-out',
        'glow-pulse': 'glowPulse 2s ease-in-out infinite',
        'hp-drain': 'hpDrain 0.5s ease-out',
      },
      keyframes: {
        shake: {
          '10%, 90%': { transform: 'translate3d(-1px, 0, 0)' },
          '20%, 80%': { transform: 'translate3d(2px, 0, 0)' },
          '30%, 50%, 70%': { transform: 'translate3d(-4px, 0, 0)' },
          '40%, 60%': { transform: 'translate3d(4px, 0, 0)' },
        },
        slideUp: {
          from: { opacity: '0', transform: 'translateY(20px)' },
          to: { opacity: '1', transform: 'translateY(0)' },
        },
        fadeIn: {
          from: { opacity: '0' },
          to: { opacity: '1' },
        },
        glowPulse: {
          '0%, 100%': { opacity: '0.5', transform: 'scale(1)' },
          '50%': { opacity: '0.8', transform: 'scale(1.05)' },
        },
        hpDrain: {
          from: { width: 'var(--hp-from)' },
          to: { width: 'var(--hp-to)' },
        },
      },
      backgroundImage: {
        'arena-gradient': 'linear-gradient(135deg, #1a1625 0%, #2d1f47 50%, #1a1625 100%)',
        'card-shine': 'linear-gradient(135deg, transparent 40%, rgba(255,255,255,0.1) 50%, transparent 60%)',
      },
      boxShadow: {
        'card': '0 4px 12px rgba(0, 0, 0, 0.15)',
        'card-hover': '0 8px 24px rgba(0, 0, 0, 0.25)',
        'glow-orange': '0 0 20px rgba(249, 115, 22, 0.4)',
        'glow-purple': '0 0 20px rgba(124, 58, 237, 0.4)',
        'glow-cyan': '0 0 20px rgba(6, 182, 212, 0.4)',
      },
      spacing: {
        'safe-bottom': 'env(safe-area-inset-bottom)',
      },
    },
  },
  plugins: [],
}
