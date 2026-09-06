// web/src/components/forms/__tests__/InstanceTypeAutocomplete.test.tsx
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import React from 'react';
import { catalogApi } from '../../../api/client';
import { InstanceTypeAutocomplete } from '../InstanceTypeAutocomplete';
import type { CatalogInstancesResponse, ComputeCatalogItem } from '../../../types/api';

const createWrapper = () => {
  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={qc}>{children}</QueryClientProvider>
  );
};

const mockInstances: ComputeCatalogItem[] = [
  {
    id: 1,
    provider: 'aws',
    instance_type_id: 'c6i.large',
    display_name: 'Compute Optimized c6i.large',
    instance_family: 'c6i',
    category: 'compute_optimized',
    vcpu: 2,
    memory_gib: 4,
    cpu_architecture: 'x86_64',
    gpu_count: 0,
    is_burstable: false,
    is_current_gen: true,
    first_seen_at: '2026-01-01T00:00:00Z',
    last_seen_at: '2026-09-04T00:00:00Z',
  },
  {
    id: 2,
    provider: 'aws',
    instance_type_id: 'm6i.xlarge',
    display_name: 'General Purpose m6i.xlarge',
    instance_family: 'm6i',
    category: 'general_purpose',
    vcpu: 4,
    memory_gib: 16,
    cpu_architecture: 'x86_64',
    gpu_count: 0,
    is_burstable: false,
    is_current_gen: true,
    first_seen_at: '2026-01-01T00:00:00Z',
    last_seen_at: '2026-09-04T00:00:00Z',
  },
  {
    id: 3,
    provider: 'azure',
    instance_type_id: 'Standard_D4s_v5',
    display_name: 'General Purpose Standard_D4s_v5',
    instance_family: 'D4s_v5',
    category: 'general_purpose',
    vcpu: 4,
    memory_gib: 16,
    cpu_architecture: 'x86_64',
    gpu_count: 0,
    is_burstable: false,
    is_current_gen: true,
    first_seen_at: '2026-01-01T00:00:00Z',
    last_seen_at: '2026-09-04T00:00:00Z',
  },
];

const mockCatalogResponse: CatalogInstancesResponse = {
  count: 3,
  total: 3,
  instances: mockInstances,
};

describe('InstanceTypeAutocomplete', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('renders trigger with placeholder when no instance is selected', () => {
    vi.spyOn(catalogApi, 'getComputeInstances').mockResolvedValue(mockCatalogResponse);

    render(<InstanceTypeAutocomplete onSelectInstance={vi.fn()} />, { wrapper: createWrapper() });

    expect(screen.getByTestId('instance-autocomplete-trigger')).toHaveTextContent('Search / Auto-fill Specs...');
  });

  it('renders selected instance preset label when provided', () => {
    vi.spyOn(catalogApi, 'getComputeInstances').mockResolvedValue(mockCatalogResponse);

    render(
      <InstanceTypeAutocomplete
        onSelectInstance={vi.fn()}
        selectedInstanceId="c6i.large"
      />,
      { wrapper: createWrapper() }
    );

    expect(screen.getByTestId('instance-autocomplete-trigger')).toHaveTextContent('Preset: c6i.large');
  });

  it('opens dropdown on click and filters options based on search query', async () => {
    vi.spyOn(catalogApi, 'getComputeInstances').mockResolvedValue(mockCatalogResponse);

    render(<InstanceTypeAutocomplete onSelectInstance={vi.fn()} />, { wrapper: createWrapper() });

    const trigger = screen.getByTestId('instance-autocomplete-trigger');
    fireEvent.click(trigger);

    expect(screen.getByTestId('instance-autocomplete-dropdown')).toBeInTheDocument();
    expect(await screen.findByTestId('autocomplete-option-c6i.large')).toBeInTheDocument();
    expect(screen.getByTestId('autocomplete-option-m6i.xlarge')).toBeInTheDocument();

    const input = screen.getByTestId('instance-autocomplete-input');
    fireEvent.change(input, { target: { value: 'Standard_D4s' } });

    expect(screen.getByTestId('autocomplete-option-Standard_D4s_v5')).toBeInTheDocument();
    expect(screen.queryByTestId('autocomplete-option-c6i.large')).not.toBeInTheDocument();
  });

  it('calls onSelectInstance and closes dropdown when an option is clicked', async () => {
    const handleSelect = vi.fn();
    vi.spyOn(catalogApi, 'getComputeInstances').mockResolvedValue(mockCatalogResponse);

    render(<InstanceTypeAutocomplete onSelectInstance={handleSelect} />, { wrapper: createWrapper() });

    fireEvent.click(screen.getByTestId('instance-autocomplete-trigger'));

    const option = await screen.findByTestId('autocomplete-option-c6i.large');
    fireEvent.click(option);

    expect(handleSelect).toHaveBeenCalledWith(mockInstances[0]);
    expect(screen.queryByTestId('instance-autocomplete-dropdown')).not.toBeInTheDocument();
  });
});
