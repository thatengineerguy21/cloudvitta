// web/src/api/queries/useCalculateQuery.ts
import { useQuery } from '@tanstack/react-query';
import { calculateApi } from '../client';
import type { CalculateRequestBody, CalculateResponse } from '../../types/api';

/**
 * Checks if the request body contains at least one active category payload.
 */
export function hasActiveCategories(body?: CalculateRequestBody | null): boolean {
  if (!body) return false;
  return Boolean(
    body.compute ||
      body.storage ||
      body.network ||
      body.database_rdbms ||
      body.database ||
      body.database_nosql ||
      body.kubernetes ||
      body.serverless
  );
}

export function useCalculateWorkload(body: CalculateRequestBody, enabled = true) {
  const hasCategories = hasActiveCategories(body);
  const isQueryEnabled = enabled && hasCategories;

  return useQuery<CalculateResponse, Error>({
    queryKey: ['calculate', body],
    queryFn: ({ signal }) => calculateApi.calculate(body, { signal }),
    enabled: isQueryEnabled,
    placeholderData: (previousData) => previousData,
  });
}
