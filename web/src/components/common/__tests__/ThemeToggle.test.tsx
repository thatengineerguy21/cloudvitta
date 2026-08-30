import { render, screen, fireEvent } from '@testing-library/react';
import { describe, it, expect, beforeEach, vi } from 'vitest';
import { ThemeToggle } from '../ThemeToggle';

describe('ThemeToggle Component', () => {
  beforeEach(() => {
    localStorage.clear();
    document.documentElement.classList.remove('dark');
    vi.clearAllMocks();
  });

  it('renders theme toggle button with light theme default', () => {
    render(<ThemeToggle />);
    const button = screen.getByRole('button', { name: /toggle visual theme/i });
    expect(button).toBeInTheDocument();
    expect(document.documentElement.classList.contains('dark')).toBe(false);
  });

  it('toggles theme to dark when clicked, adding dark class and updating localStorage', () => {
    render(<ThemeToggle />);
    const button = screen.getByRole('button', { name: /toggle visual theme/i });

    fireEvent.click(button);

    expect(document.documentElement.classList.contains('dark')).toBe(true);
    expect(localStorage.getItem('cv_theme')).toBe('dark');

    fireEvent.click(button);

    expect(document.documentElement.classList.contains('dark')).toBe(false);
    expect(localStorage.getItem('cv_theme')).toBe('light');
  });

  it('initializes from localStorage if dark theme was previously saved', () => {
    localStorage.setItem('cv_theme', 'dark');
    render(<ThemeToggle />);
    expect(document.documentElement.classList.contains('dark')).toBe(true);
  });
});
