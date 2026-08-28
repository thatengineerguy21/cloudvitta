// web/src/router/__tests__/Router.test.tsx
import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, act } from '@testing-library/react';
import { Router, Link, useLocation, useNavigate } from '../index';

const TestLocationConsumer: React.FC = () => {
  const { pathname, search } = useLocation();
  const navigate = useNavigate();

  return (
    <div>
      <span data-testid="pathname">{pathname}</span>
      <span data-testid="search">{search}</span>
      <button onClick={() => navigate('/compare/storage?size_gb=100')}>
        Go to Storage
      </button>
      <button onClick={() => navigate('/compare/network', { replace: true })}>
        Replace with Network
      </button>
    </div>
  );
};

describe('Router and Link Component', () => {
  beforeEach(() => {
    window.history.pushState(null, '', '/');
  });

  it('provides default location state at root path', () => {
    render(
      <Router>
        <TestLocationConsumer />
      </Router>
    );

    expect(screen.getByTestId('pathname')).toHaveTextContent('/');
    expect(screen.getByTestId('search')).toHaveTextContent('');
  });

  it('navigates to new route via pushState on navigate() call', () => {
    render(
      <Router>
        <TestLocationConsumer />
      </Router>
    );

    fireEvent.click(screen.getByText('Go to Storage'));

    expect(screen.getByTestId('pathname')).toHaveTextContent('/compare/storage');
    expect(screen.getByTestId('search')).toHaveTextContent('?size_gb=100');
  });

  it('navigates with replaceState when replace option is true', () => {
    const replaceStateSpy = vi.spyOn(window.history, 'replaceState');

    render(
      <Router>
        <TestLocationConsumer />
      </Router>
    );

    fireEvent.click(screen.getByText('Replace with Network'));

    expect(screen.getByTestId('pathname')).toHaveTextContent('/compare/network');
    expect(replaceStateSpy).toHaveBeenCalled();
    replaceStateSpy.mockRestore();
  });

  it('intercepts Link clicks and updates location without full page reload', () => {
    render(
      <Router>
        <div>
          <Link to="/compare/compute?vcpu=8" data-testid="compute-link">
            Compute
          </Link>
          <TestLocationConsumer />
        </div>
      </Router>
    );

    fireEvent.click(screen.getByTestId('compute-link'));

    expect(screen.getByTestId('pathname')).toHaveTextContent('/compare/compute');
    expect(screen.getByTestId('search')).toHaveTextContent('?vcpu=8');
  });

  it('applies activeClassName when Link matches current path', () => {
    window.history.pushState(null, '', '/compare/compute');

    render(
      <Router>
        <div>
          <Link
            to="/compare/compute"
            className="base-class"
            activeClassName="is-active"
            data-testid="active-link"
          >
            Compute
          </Link>
          <Link
            to="/compare/storage"
            className="base-class"
            activeClassName="is-active"
            data-testid="inactive-link"
          >
            Storage
          </Link>
        </div>
      </Router>
    );

    expect(screen.getByTestId('active-link')).toHaveClass('base-class is-active');
    expect(screen.getByTestId('inactive-link')).toHaveClass('base-class');
    expect(screen.getByTestId('inactive-link')).not.toHaveClass('is-active');
  });

  it('responds to browser popstate events', () => {
    render(
      <Router>
        <TestLocationConsumer />
      </Router>
    );

    act(() => {
      window.history.pushState(null, '', '/compare/database?engine=postgresql');
      window.dispatchEvent(new PopStateEvent('popstate'));
    });

    expect(screen.getByTestId('pathname')).toHaveTextContent('/compare/database');
    expect(screen.getByTestId('search')).toHaveTextContent('?engine=postgresql');
  });
});
