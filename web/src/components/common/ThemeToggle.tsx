import React, { useEffect, useState } from 'react';
import { Sun, Moon } from 'lucide-react';

export const ThemeToggle: React.FC = () => {
  const [isDark, setIsDark] = useState<boolean>(() => {
    if (typeof window !== 'undefined') {
      const stored = localStorage.getItem('cv_theme');
      if (stored) {
        return stored === 'dark';
      }
      return window.matchMedia('(prefers-color-scheme: dark)').matches;
    }
    return false;
  });

  useEffect(() => {
    const root = document.documentElement;
    if (isDark) {
      root.classList.add('dark');
      localStorage.setItem('cv_theme', 'dark');
    } else {
      root.classList.remove('dark');
      localStorage.setItem('cv_theme', 'light');
    }
  }, [isDark]);

  return (
    <button
      onClick={() => setIsDark((prev) => !prev)}
      className="p-2 border border-border-default hover:border-border-accent bg-surface-card text-text-primary transition-colors flex items-center justify-center cursor-pointer"
      aria-label="Toggle visual theme"
      title={isDark ? 'Switch to Light Editorial' : 'Switch to Dark Obsidian'}
    >
      {isDark ? (
        <Sun className="w-4 h-4 text-border-accent" />
      ) : (
        <Moon className="w-4 h-4 text-text-secondary" />
      )}
    </button>
  );
};
