/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        // Primary: Vibrant orange for main actions and highlights
        arena: {
          50: '#fff7ed',
          100: '#ffedd5',
          200: '#fed7aa',
          300: '#fdba74',
          400: '#fb923c',
          500: '#f97316', // Primary orange
          600: '#ea580c',
          700: '#c2410c',
          800: '#9a3412',
          900: '#7c2d12',
          950: '#431407',
        },
        // Secondary: Deep blue for balance and contrast
        arena_blue: {
          50: '#eff6ff',
          100: '#dbeafe',
          200: '#bfdbfe',
          300: '#93c5fd',
          400: '#60a5fa',
          500: '#3b82f6', // Secondary blue
          600: '#2563eb',
          700: '#1d4ed8',
          800: '#1e40af',
          900: '#1e3a8a',
          950: '#172554',
        },
        // Accent: Electric purple for special elements
        arena_purple: {
          50: '#faf5ff',
          100: '#f3e8ff',
          200: '#e9d5ff',
          300: '#d8b4fe',
          400: '#c084fc',
          500: '#a855f7', // Purple accent
          600: '#9333ea',
          700: '#7c3aed',
          800: '#6b21a8',
          900: '#581c87',
          950: '#3b0764',
        },
        // Accent: Hot pink for victories and highlights
        arena_pink: {
          50: '#fdf2f8',
          100: '#fce7f3',
          200: '#fbcfe8',
          300: '#f9a8d4',
          400: '#f472b6',
          500: '#ec4899', // Pink accent
          600: '#db2777',
          700: '#be185d',
          800: '#9d174d',
          900: '#831843',
          950: '#500724',
        },
        // Accent: Cyber cyan for energy and HP
        arena_cyan: {
          50: '#ecfeff',
          100: '#cffafe',
          200: '#a5f3fc',
          300: '#67e8f9',
          400: '#22d3ee',
          500: '#06b6d4', // Cyan accent
          600: '#0891b2',
          700: '#0e7490',
          800: '#155e75',
          900: '#164e63',
          950: '#083344',
        },
        // Battle UI colors
        hp: {
          full: '#22c55e',    // Green when HP > 66%
          medium: '#eab308',  // Yellow when HP 33-66%
          low: '#ef4444',     // Red when HP < 33%
        },
        // Coin display
        coin: {
          gold: '#fbbf24',
          shine: '#fef3c7',
        },
        // Background tones for dark theme
        arena_dark: {
          900: '#0f0f1a',
          800: '#1a1a2e',
          700: '#252542',
          600: '#2d2d56',
          500: '#3d3d6b',
        },
      },
      fontFamily: {
        // Display font for headings and important text
        display: ['Outfit', 'system-ui', 'sans-serif'],
        // Body font for general text
        body: ['Inter', 'system-ui', 'sans-serif'],
        // Monospace for stats and numbers
        mono: ['JetBrains Mono', 'Fira Code', 'monospace'],
      },
      animation: {
        'bounce-slow': 'bounce 2s infinite',
        'pulse-slow': 'pulse 3s cubic-bezier(0.4, 0, 0.6, 1) infinite',
        'shake': 'shake 0.5s cubic-bezier(0.36, 0.07, 0.19, 0.97) both',
        'glow-pulse': 'glow-pulse 2s ease-in-out infinite',
        'slide-up': 'slide-up 0.3s ease-out',
        'slide-down': 'slide-down 0.3s ease-out',
        'fade-in': 'fade-in 0.2s ease-out',
        'scale-in': 'scale-in 0.2s ease-out',
        'hp-drain': 'hp-drain 0.3s ease-out',
      },
      keyframes: {
        shake: {
          '10%, 90%': { transform: 'translate3d(-1px, 0, 0)' },
          '20%, 80%': { transform: 'translate3d(2px, 0, 0)' },
          '30%, 50%, 70%': { transform: 'translate3d(-4px, 0, 0)' },
          '40%, 60%': { transform: 'translate3d(4px, 0, 0)' },
        },
        'glow-pulse': {
          '0%, 100%': {
            boxShadow: '0 0 10px rgba(249, 115, 22, 0.3)',
            opacity: '1',
          },
          '50%': {
            boxShadow: '0 0 25px rgba(249, 115, 22, 0.6)',
            opacity: '0.9',
          },
        },
        'slide-up': {
          '0%': { transform: 'translateY(100%)', opacity: '0' },
          '100%': { transform: 'translateY(0)', opacity: '1' },
        },
        'slide-down': {
          '0%': { transform: 'translateY(-100%)', opacity: '0' },
          '100%': { transform: 'translateY(0)', opacity: '1' },
        },
        'fade-in': {
          '0%': { opacity: '0' },
          '100%': { opacity: '1' },
        },
        'scale-in': {
          '0%': { transform: 'scale(0.9)', opacity: '0' },
          '100%': { transform: 'scale(1)', opacity: '1' },
        },
        'hp-drain': {
          '0%': { transform: 'scaleX(1)' },
          '100%': { transform: 'scaleX(var(--hp-percent, 1))' },
        },
      },
      spacing: {
        // Safe area for tab bar
        'tab-bar': '80px',
        // Card dimensions
        'card-w': '300px',
        'card-h': '450px',
        'card-full-w': '400px',
        'card-full-h': '600px',
      },
      borderRadius: {
        'card': '12px',
        'btn': '20px',
      },
      boxShadow: {
        'card': '0 4px 20px rgba(0, 0, 0, 0.15)',
        'card-hover': '0 8px 30px rgba(0, 0, 0, 0.25)',
        'glow-orange': '0 0 20px rgba(249, 115, 22, 0.4)',
        'glow-blue': '0 0 20px rgba(59, 130, 246, 0.4)',
        'glow-purple': '0 0 20px rgba(168, 85, 247, 0.4)',
        'glow-cyan': '0 0 20px rgba(6, 182, 212, 0.4)',
        'inner-glow': 'inset 0 0 20px rgba(249, 115, 22, 0.2)',
      },
      backgroundImage: {
        // Gradient backgrounds for card game aesthetic
        'arena-gradient': 'linear-gradient(135deg, #0f0f1a 0%, #1a1a2e 50%, #252542 100%)',
        'card-back': 'linear-gradient(145deg, #2d2d44 0%, #1a1a2e 50%, #0f0f1a 100%)',
        'victory-glow': 'radial-gradient(ellipse at center, rgba(249, 115, 22, 0.3) 0%, transparent 70%)',
        'battle-bg': 'radial-gradient(ellipse at center, #1a1a2e 0%, #0f0f1a 100%)',
      },
      transitionProperty: {
        'hp': 'width, background-color',
      },
      transitionDuration: {
        'hp': '300ms',
      },
    },
  },
  plugins: [],
}
