// web/src/lib/queryClient.ts
import { QueryClient } from '@tanstack/react-query';

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 5 * 60 * 1000, // 5 minutes: matches backend observation cache cadence
      gcTime: 15 * 60 * 1000,   // 15 minutes: garbage collection for inactive views
      refetchOnWindowFocus: false, // Prevents aggressive network spam during review
      retry: 1, // Retries once on transient network failure; fails fast on 4xx RFC 7807 errors
    },
  },
});
