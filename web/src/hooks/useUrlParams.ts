// web/src/hooks/useUrlParams.ts
import { useCallback, useMemo } from 'react';
import { useLocation } from '../router';

export type UrlParamValue = string | number | boolean | undefined | null;

export function parseParamValue<V>(raw: string | null, defaultValue: V): V {
  if (raw === null || raw === undefined || raw === '') {
    return defaultValue;
  }

  if (typeof defaultValue === 'boolean') {
    if (raw === 'true' || raw === '1') return true as unknown as V;
    if (raw === 'false' || raw === '0') return false as unknown as V;
    return defaultValue;
  }

  if (typeof defaultValue === 'number') {
    const num = Number(raw);
    return (isNaN(num) ? defaultValue : num) as unknown as V;
  }

  return raw as unknown as V;
}

export function useUrlParams<T extends Record<string, UrlParamValue>>(
  defaultValues: T
): [T, (updates: Partial<T>, options?: { replace?: boolean }) => void] {
  const { pathname, search, navigate } = useLocation();

  const params = useMemo(() => {
    const searchParams = new URLSearchParams(search);
    const result = { ...defaultValues } as Record<string, UrlParamValue>;

    Object.keys(defaultValues).forEach((key) => {
      const rawVal = searchParams.get(key);
      result[key] = parseParamValue(rawVal, defaultValues[key]);
    });

    // Also include any extra search parameters that might not be in defaultValues
    searchParams.forEach((val, key) => {
      if (!(key in defaultValues)) {
        result[key] = val;
      }
    });

    return result as T;
  }, [search, defaultValues]);

  const setParams = useCallback(
    (updates: Partial<T>, options: { replace?: boolean } = { replace: true }) => {
      const currentSearch = typeof window !== 'undefined' ? window.location.search : search;
      const searchParams = new URLSearchParams(currentSearch);

      const merged = { ...params, ...updates };

      Object.entries(merged).forEach(([key, val]) => {
        if (val === undefined || val === null || val === '') {
          searchParams.delete(key);
        } else {
          searchParams.set(key, String(val));
        }
      });

      const newSearchStr = searchParams.toString();
      const newUrl = newSearchStr ? `${pathname}?${newSearchStr}` : pathname;

      navigate(newUrl, { replace: options.replace ?? true });
    },
    [pathname, search, params, navigate]
  );

  return [params, setParams];
}
