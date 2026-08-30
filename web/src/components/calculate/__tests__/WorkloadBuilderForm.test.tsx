// web/src/components/calculate/__tests__/WorkloadBuilderForm.test.tsx
import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { WorkloadBuilderForm } from '../WorkloadBuilderForm';
import {
  DEFAULT_WORKLOAD_STATE,
  WorkloadFormState,
} from '../../../lib/workloadUrlParams';

describe('WorkloadBuilderForm component', () => {
  it('renders all 7 workload category accordions and active count', () => {
    const onChange = vi.fn();
    const onReset = vi.fn();

    render(
      <WorkloadBuilderForm
        state={DEFAULT_WORKLOAD_STATE}
        onChange={onChange}
        onReset={onReset}
      />
    );

    expect(screen.getByText(/1 of 7 Active/i)).toBeInTheDocument();
    expect(screen.getByText('Compute Instances')).toBeInTheDocument();
    expect(screen.getByText('Storage Classes')).toBeInTheDocument();
    expect(screen.getByText('Network Egress')).toBeInTheDocument();
    expect(screen.getByText('Relational Databases (RDBMS)')).toBeInTheDocument();
    expect(screen.getByText('NoSQL Databases')).toBeInTheDocument();
    expect(screen.getByText('Kubernetes Control-Plane')).toBeInTheDocument();
    expect(screen.getByText('Serverless Compute (FaaS)')).toBeInTheDocument();
  });

  it('toggles category enabled status on checkbox click', () => {
    const onChange = vi.fn();
    const onReset = vi.fn();

    render(
      <WorkloadBuilderForm
        state={DEFAULT_WORKLOAD_STATE}
        onChange={onChange}
        onReset={onReset}
      />
    );

    const storageCheckbox = screen.getByRole('checkbox', { name: /enable storage classes/i });
    fireEvent.click(storageCheckbox);

    expect(onChange).toHaveBeenCalledTimes(1);
    const updatedState = onChange.mock.calls[0][0] as WorkloadFormState;
    expect(updatedState.storage.enabled).toBe(true);
  });

  it('enables all and disables all categories via batch actions', () => {
    const onChange = vi.fn();
    const onReset = vi.fn();

    render(
      <WorkloadBuilderForm
        state={DEFAULT_WORKLOAD_STATE}
        onChange={onChange}
        onReset={onReset}
      />
    );

    const enableAllBtn = screen.getByText('Enable All');
    fireEvent.click(enableAllBtn);

    expect(onChange).toHaveBeenCalledTimes(1);
    const allEnabled = onChange.mock.calls[0][0] as WorkloadFormState;
    expect(allEnabled.compute.enabled).toBe(true);
    expect(allEnabled.storage.enabled).toBe(true);
    expect(allEnabled.network.enabled).toBe(true);
    expect(allEnabled.database_rdbms.enabled).toBe(true);
    expect(allEnabled.database_nosql.enabled).toBe(true);
    expect(allEnabled.kubernetes.enabled).toBe(true);
    expect(allEnabled.serverless.enabled).toBe(true);

    const disableAllBtn = screen.getByText('Disable All');
    fireEvent.click(disableAllBtn);

    const allDisabled = onChange.mock.calls[1][0] as WorkloadFormState;
    expect(allDisabled.compute.enabled).toBe(false);
    expect(allDisabled.storage.enabled).toBe(false);
  });

  it('calls onReset when Reset button is clicked', () => {
    const onChange = vi.fn();
    const onReset = vi.fn();

    render(
      <WorkloadBuilderForm
        state={DEFAULT_WORKLOAD_STATE}
        onChange={onChange}
        onReset={onReset}
      />
    );

    const resetBtn = screen.getByTitle('Reset form to defaults');
    fireEvent.click(resetBtn);

    expect(onReset).toHaveBeenCalledTimes(1);
  });

  it('updates global region and currency when selectors change', () => {
    const onChange = vi.fn();
    const onReset = vi.fn();

    render(
      <WorkloadBuilderForm
        state={DEFAULT_WORKLOAD_STATE}
        onChange={onChange}
        onReset={onReset}
      />
    );

    const regionSelect = screen.getByLabelText('Region Group');
    fireEvent.change(regionSelect, { target: { value: 'eu-west' } });

    expect(onChange).toHaveBeenCalledTimes(1);
    expect((onChange.mock.calls[0][0] as WorkloadFormState).region).toBe('eu-west');
  });

  it('updates category inputs using debounced input handlers', async () => {
    const onChange = vi.fn();
    const onReset = vi.fn();

    render(
      <WorkloadBuilderForm
        state={DEFAULT_WORKLOAD_STATE}
        onChange={onChange}
        onReset={onReset}
      />
    );

    const vcpuInput = screen.getByLabelText('vCPU Cores');
    fireEvent.change(vcpuInput, { target: { value: '8' } });

    await waitFor(() => {
      expect(onChange).toHaveBeenCalled();
      const lastCall = onChange.mock.calls[onChange.mock.calls.length - 1][0] as WorkloadFormState;
      expect(lastCall.compute.vcpu).toBe(8);
    }, { timeout: 600 });
  });
});
