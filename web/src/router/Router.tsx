// web/src/router/Router.tsx
import React, { useEffect, useState, useMemo, useCallback } from 'react';
import { RouterContext, useLocation, LocationState, NavigateOptions } from './RouterContext';

export const Router: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [location, setLocation] = useState<LocationState>(() => ({
    pathname: typeof window !== 'undefined' ? window.location.pathname : '/',
    search: typeof window !== 'undefined' ? window.location.search : '',
  }));

  useEffect(() => {
    const handlePopState = () => {
      setLocation({
        pathname: window.location.pathname,
        search: window.location.search,
      });
    };

    window.addEventListener('popstate', handlePopState);
    return () => window.removeEventListener('popstate', handlePopState);
  }, []);

  const navigate = useCallback((to: string, options: NavigateOptions = {}) => {
    if (typeof window === 'undefined') return;

    const url = new URL(to, window.location.origin);
    const newPathname = url.pathname;
    const newSearch = url.search;

    if (options.replace) {
      window.history.replaceState(null, '', url.pathname + url.search + url.hash);
    } else {
      window.history.pushState(null, '', url.pathname + url.search + url.hash);
    }

    setLocation({
      pathname: newPathname,
      search: newSearch,
    });
  }, []);

  const value = useMemo(
    () => ({
      pathname: location.pathname,
      search: location.search,
      navigate,
    }),
    [location.pathname, location.search, navigate]
  );

  return <RouterContext.Provider value={value}>{children}</RouterContext.Provider>;
};

export interface LinkProps extends React.AnchorHTMLAttributes<HTMLAnchorElement> {
  to: string;
  replace?: boolean;
  activeClassName?: string;
  exact?: boolean;
}

export const Link: React.FC<LinkProps> = ({
  to,
  replace = false,
  className = '',
  activeClassName = '',
  exact = false,
  onClick,
  children,
  ...rest
}) => {
  const { pathname, navigate } = useLocation();

  const targetPath = to.split('?')[0].split('#')[0];
  const isActive = exact
    ? pathname === targetPath
    : targetPath === '/'
      ? pathname === '/'
      : pathname.startsWith(targetPath);

  const handleClick = (e: React.MouseEvent<HTMLAnchorElement>) => {
    if (onClick) {
      onClick(e);
    }

    if (
      !e.defaultPrevented &&
      e.button === 0 &&
      !e.metaKey &&
      !e.ctrlKey &&
      !e.altKey &&
      !e.shiftKey &&
      (!rest.target || rest.target === '_self')
    ) {
      e.preventDefault();
      navigate(to, { replace });
    }
  };

  const combinedClassName = `${className} ${isActive && activeClassName ? activeClassName : ''}`.trim();

  return (
    <a href={to} onClick={handleClick} className={combinedClassName} {...rest}>
      {children}
    </a>
  );
};
