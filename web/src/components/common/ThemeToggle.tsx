import React, { useEffect, useState } from 'react';
import { Sun, Moon } from 'lucide-react';
import { Theme } from '../../types';

export const ThemeToggle: React.FC = () => {
  const [theme, setTheme] = useState<Theme>(() => {
    if (typeof window !== 'undefined') {
      const stored = window.localStorage?.getItem('cv_theme');
      if (stored === 'dark' || stored === 'light') {
        return stored;
      }
      return window.matchMedia?.('(prefers-color-scheme: dark)')?.matches ? 'dark' : 'light';
    }
    return 'light';
  });

  useEffect(() => {
    const root = document.documentElement;
    if (theme === 'dark') {
      root.classList.add('dark');
      localStorage.setItem('cv_theme', 'dark');
    } else {
      root.classList.remove('dark');
      localStorage.setItem('cv_theme', 'light');
    }
  }, [theme]);

  const toggleTheme = () => {
    setTheme((prev) => (prev === 'dark' ? 'light' : 'dark'));
  };

  return (
    <button
      onClick={toggleTheme}
      className="p-2 border border-border-default hover:border-border-accent bg-surface-card text-text-primary transition-colors flex items-center justify-center cursor-pointer focus-visible:ring-1 focus-visible:ring-border-accent focus-visible:outline-none"
      aria-label="Toggle visual theme"
      title={theme === 'dark' ? 'Switch to Light Editorial' : 'Switch to Dark Obsidian'}
    >
      {theme === 'dark' ? (
        <Sun className="w-4 h-4 text-border-accent" aria-hidden="true" />
      ) : (
        <Moon className="w-4 h-4 text-text-secondary" aria-hidden="true" />
      )}
    </button>
  );
};
