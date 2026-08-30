import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { Router } from '../../router';
import { NotFoundPage } from '../NotFoundPage';

describe('NotFoundPage', () => {
  it('renders 404 heading and return link', () => {
    window.history.replaceState(null, '', '/unknown');
    render(
      <Router>
        <NotFoundPage />
      </Router>
    );

    expect(screen.getByText('Page Not Found')).toBeInTheDocument();
    expect(screen.getByTestId('not-found-page')).toBeInTheDocument();
    expect(screen.getByText('Return to Landing Page')).toBeInTheDocument();
  });

  it('displays the invalid path', () => {
    window.history.replaceState(null, '', '/invalid/path');
    render(
      <Router>
        <NotFoundPage />
      </Router>
    );

    expect(screen.getByText('/invalid/path')).toBeInTheDocument();
  });
});
