// web/src/hooks/__tests__/useUrlParams.test.tsx
import React from 'react';
import { describe, it, expect, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { Router } from '../../router';
import { useUrlParams, UrlParamValue } from '../useUrlParams';

interface SampleParams {
  vcpu: number;
  region: string;
  strict: boolean;
  [key: string]: UrlParamValue;
}

const DEFAULT_SAMPLE: SampleParams = {
  vcpu: 4,
  region: 'us-east',
  strict: true,
};

const TestUrlParamsConsumer: React.FC = () => {
  const [params, setParams] = useUrlParams<SampleParams>(DEFAULT_SAMPLE);

  return (
    <div>
      <span data-testid="vcpu-val">{params.vcpu}</span>
      <span data-testid="region-val">{params.region}</span>
      <span data-testid="strict-val">{String(params.strict)}</span>

      <button onClick={() => setParams({ vcpu: 8 })}>Set vCPU 8</button>
      <button onClick={() => setParams({ region: 'eu-central' })}>Set EU</button>
      <button onClick={() => setParams({ strict: false })}>Disable Strict</button>
    </div>
  );
};

describe('useUrlParams Hook', () => {
  beforeEach(() => {
    window.history.pushState(null, '', '/compare/compute');
  });

  it('initializes with default values when query string is empty', () => {
    render(
      <Router>
        <TestUrlParamsConsumer />
      </Router>
    );

    expect(screen.getByTestId('vcpu-val')).toHaveTextContent('4');
    expect(screen.getByTestId('region-val')).toHaveTextContent('us-east');
    expect(screen.getByTestId('strict-val')).toHaveTextContent('true');
  });

  it('parses and type-coerces numbers and booleans from initial URL search params', () => {
    window.history.pushState(null, '', '/compare/compute?vcpu=16&region=ap-south&strict=false');

    render(
      <Router>
        <TestUrlParamsConsumer />
      </Router>
    );

    expect(screen.getByTestId('vcpu-val')).toHaveTextContent('16');
    expect(screen.getByTestId('region-val')).toHaveTextContent('ap-south');
    expect(screen.getByTestId('strict-val')).toHaveTextContent('false');
  });

  it('updates URL search parameters and hook state on setParams call', () => {
    render(
      <Router>
        <TestUrlParamsConsumer />
      </Router>
    );

    fireEvent.click(screen.getByText('Set vCPU 8'));

    expect(screen.getByTestId('vcpu-val')).toHaveTextContent('8');
    expect(window.location.search).toContain('vcpu=8');

    fireEvent.click(screen.getByText('Set EU'));

    expect(screen.getByTestId('region-val')).toHaveTextContent('eu-central');
    expect(window.location.search).toContain('region=eu-central');

    fireEvent.click(screen.getByText('Disable Strict'));

    expect(screen.getByTestId('strict-val')).toHaveTextContent('false');
    expect(window.location.search).toContain('strict=false');
  });
});
