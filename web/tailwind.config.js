/** @type {import('tailwindcss').Config} */
export default {
  darkMode: 'class',
  content: ['./index.html', './src/**/*.{js,ts,jsx,tsx}'],
  theme: {
    extend: {
      colors: {
        brand: {
          50: '#FDF7F5',
          100: '#FBECE8',
          200: '#F7D6CE',
          500: '#E06D53',
          600: '#D55539',
          700: '#BA432A',
        },
        warm: {
          50: '#FAF9F6',
          100: '#F4F2EC',
          200: '#ECE9E1',
          300: '#DFDAD0',
          400: '#C5BEB2',
          500: '#A39C8F',
          600: '#7B7468',
          700: '#5A544A',
          800: '#3D3831',
          900: '#23201C',
        },
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
          subtle: 'var(--color-text-subtle)',
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
        sm: '0.25rem',
        DEFAULT: '0.5rem',
        md: '0.75rem',
        lg: '1rem',
        xl: '1.25rem',
        '2xl': '1rem',
        '3xl': '1.5rem',
        full: '9999px',
      },
      fontFamily: {
        brand: ['"Qasira"', '"EB Garamond"', 'serif'],
        display: ['"Manrope"', '"Inter"', 'sans-serif'],
        sans: ['"Plus Jakarta Sans"', '"Inter"', '"Manrope"', 'sans-serif'],
      },
      boxShadow: {
        sm: '0 1px 2px 0 rgba(0, 0, 0, 0.05)',
        DEFAULT: '0 1px 3px 0 rgba(0, 0, 0, 0.1), 0 1px 2px -1px rgba(0, 0, 0, 0.1)',
        md: '0 4px 6px -1px rgba(0, 0, 0, 0.1), 0 2px 4px -2px rgba(0, 0, 0, 0.1)',
        lg: '0 10px 15px -3px rgba(0, 0, 0, 0.1), 0 4px 6px -4px rgba(0, 0, 0, 0.1)',
        xl: '0 20px 25px -5px rgba(0, 0, 0, 0.1), 0 8px 10px -6px rgba(0, 0, 0, 0.1)',
        'warm-sm': '0 1px 3px 0 rgba(28, 25, 23, 0.03), 0 6px 16px -4px rgba(28, 25, 23, 0.05)',
        'warm-md': '0 4px 6px -2px rgba(28, 25, 23, 0.04), 0 12px 24px -4px rgba(28, 25, 23, 0.08)',
        'brand-glow': '0 4px 16px -4px rgba(224, 109, 83, 0.35)',
      },
    },
  },
  plugins: [],
};
