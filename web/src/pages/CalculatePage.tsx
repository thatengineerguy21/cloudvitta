// web/src/pages/CalculatePage.tsx
import React, { useMemo, useCallback } from 'react';
import { useLocation } from '../router';
import {
  parseWorkloadFromUrl,
  serializeWorkloadToUrl,
  buildCalculateRequestBody,
  getEnabledCategoryKeys,
  WorkloadFormState,
} from '../lib/workloadUrlParams';
import { useCalculateWorkload } from '../api/queries/useCalculateQuery';
import { WorkloadBuilderForm } from '../components/calculate/WorkloadBuilderForm';
import { CalculateResultsMatrix } from '../components/calculate/CalculateResultsMatrix';
import { BentoGrid } from '../components/bento/BentoGrid';
import { Calculator } from 'lucide-react';

export const CalculatePage: React.FC = () => {
  const { search, navigate } = useLocation();

  const state = useMemo(() => parseWorkloadFromUrl(search), [search]);

  const handleStateChange = useCallback(
    (newState: WorkloadFormState) => {
      const newQuery = serializeWorkloadToUrl(newState);
      navigate(`/calculate${newQuery}`, { replace: true });
    },
    [navigate]
  );

  const handleReset = useCallback(() => {
    navigate('/calculate', { replace: false });
  }, [navigate]);

  const requestedCategories = useMemo(() => getEnabledCategoryKeys(state), [state]);

  const requestBody = useMemo(() => buildCalculateRequestBody(state), [state]);

  const { data, isLoading, isError, error, refetch } = useCalculateWorkload(
    requestBody,
    requestedCategories.length > 0
  );

  return (
    <div className="space-y-6">
      {/* Top Page Header */}
      <div className="border border-border-default/80 bg-surface-card rounded-2xl shadow-sm p-6 lg:p-8 space-y-3">
        <div className="flex items-center space-x-2">
          <Calculator className="w-4 h-4 text-border-accent" />
          <span className="text-xs uppercase tracking-widest text-border-accent font-bold px-2 py-0.5 rounded-full bg-brand-500/10">
            Composite Workload Engine
          </span>
        </div>
        <h1 className="font-display text-3xl sm:text-4xl font-extrabold text-text-primary tracking-tight">
          Composite Workload Calculator
        </h1>
        <p className="text-sm text-text-secondary max-w-3xl leading-relaxed">
          Design a complete multi-tier cloud architecture across compute, storage, databases, networking, Kubernetes, and serverless. Real-time pricing clearly distinguishes complete quotes from partial estimates so you never face hidden infrastructure costs.
        </p>
      </div>

      {/* 12-Column Responsive Layout */}
      <BentoGrid columns={12} gap="md">
        {/* Left Column: Workload Builder Form (4 columns) */}
        <div className="col-span-12 lg:col-span-4 space-y-4">
          <WorkloadBuilderForm
            state={state}
            onChange={handleStateChange}
            onReset={handleReset}
          />
        </div>

        {/* Right Column: Composite Results Matrix (8 columns) */}
        <div className="col-span-12 lg:col-span-8 space-y-4">
          <CalculateResultsMatrix
            results={data?.results}
            warnings={data?.warnings}
            requestedCategories={requestedCategories}
            currency={state.currency}
            isLoading={isLoading}
            isError={isError}
            error={error}
            refetch={refetch}
            onReset={handleReset}
          />
        </div>
      </BentoGrid>
    </div>
  );
};
