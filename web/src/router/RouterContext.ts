// web/src/router/RouterContext.ts
import { createContext, useContext } from 'react';

export interface LocationState {
  pathname: string;
  search: string;
}

export interface NavigateOptions {
  replace?: boolean;
}

export interface RouterContextValue {
  pathname: string;
  search: string;
  navigate: (to: string, options?: NavigateOptions) => void;
}

export const RouterContext = createContext<RouterContextValue | null>(null);

export function useLocation(): RouterContextValue {
  const context = useContext(RouterContext);
  if (!context) {
    return {
      pathname: typeof window !== 'undefined' ? window.location.pathname : '/',
      search: typeof window !== 'undefined' ? window.location.search : '',
      navigate: (to: string, options?: NavigateOptions) => {
        if (typeof window === 'undefined') return;
        const url = new URL(to, window.location.origin);
        if (options?.replace) {
          window.history.replaceState(null, '', url.pathname + url.search + url.hash);
        } else {
          window.history.pushState(null, '', url.pathname + url.search + url.hash);
        }
      },
    };
  }
  return context;
}

export function useNavigate() {
  const { navigate } = useLocation();
  return navigate;
}
