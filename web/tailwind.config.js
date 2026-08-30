/** @type {import('tailwindcss').Config} */
export default {
  darkMode: 'class',
  content: ['./index.html', './src/**/*.{js,ts,jsx,tsx}'],
  theme: {
    extend: {
      colors: {
        surface: {
          page: 'var(--color-surface-page)',
          card: 'var(--color-surface-card)',
          raised: 'var(--color-surface-raised)',
        },
        border: {
          default: 'var(--color-border-default)',
          accent: 'var(--color-border-accent)',
        },
        text: {
          primary: 'var(--color-text-primary)',
          secondary: 'var(--color-text-secondary)',
        },
        status: {
          matchExact: 'var(--color-status-match-exact)',
          matchClose: 'var(--color-status-match-close)',
          matchApproximate: 'var(--color-status-match-approximate)',
          stale: 'var(--color-status-stale)',
          partial: 'var(--color-status-partial)',
          anomaly: 'var(--color-status-anomaly)',
        },
      },
      borderRadius: {
        none: '0px',
        sm: '0px',
        DEFAULT: '0px',
        md: '0px',
        lg: '0px',
        xl: '0px',
        '2xl': '0px',
        '3xl': '0px',
        full: '0px',
      },
      fontFamily: {
        display: ['"EB Garamond"', 'serif'],
        sans: ['"Manrope"', '"Hanken Grotesk"', 'sans-serif'],
      },
      boxShadow: {
        none: 'none',
      },
    },
  },
  plugins: [],
};
