import { useState } from 'react';
import { render, screen, fireEvent, act } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { DebouncedInput } from '../DebouncedInput';

describe('DebouncedInput Component', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('debounces input changes by the specified duration (default 400ms)', () => {
    const handleChange = vi.fn();
    render(<DebouncedInput label="Search" onChange={handleChange} debounceMs={400} />);

    const input = screen.getByLabelText('Search');
    fireEvent.change(input, { target: { value: 'compute' } });

    // Should not fire immediately
    expect(handleChange).not.toHaveBeenCalled();

    // Fast-forward by 200ms
    act(() => {
      vi.advanceTimersByTime(200);
    });
    expect(handleChange).not.toHaveBeenCalled();

    // Fast-forward the remaining 200ms (total 400ms)
    act(() => {
      vi.advanceTimersByTime(200);
    });
    expect(handleChange).toHaveBeenCalledTimes(1);
    expect(handleChange).toHaveBeenCalledWith('compute');
  });

  it('does NOT reset debounce timer on parent re-render with inline onChange arrow function (Task 2 acceptance criterion)', () => {
    const callbackMock = vi.fn();

    // Wrapper component that forces parent re-renders via unrelated counter state
    const ParentWrapper = () => {
      const [count, setCount] = useState(0);
      return (
        <div>
          <button data-testid="rerender-btn" onClick={() => setCount((c) => c + 1)}>
            Re-render Parent ({count})
          </button>
          {/* Inline arrow function creates a new reference on every render */}
          <DebouncedInput
            label="Search Query"
            debounceMs={400}
            onChange={(val) => callbackMock(val)}
          />
        </div>
      );
    };

    render(<ParentWrapper />);
    const input = screen.getByLabelText('Search Query');
    const rerenderBtn = screen.getByTestId('rerender-btn');

    // Type value
    fireEvent.change(input, { target: { value: 'storage' } });

    // Advance 200ms into the 400ms window
    act(() => {
      vi.advanceTimersByTime(200);
    });
    expect(callbackMock).not.toHaveBeenCalled();

    // Force parent re-render at t=200ms
    fireEvent.click(rerenderBtn);
    expect(callbackMock).not.toHaveBeenCalled();

    // Advance remaining 200ms (to t=400ms from keystroke)
    act(() => {
      vi.advanceTimersByTime(200);
    });

    // Callback MUST fire at t=400ms from the keystroke, NOT delayed to t=600ms by the re-render
    expect(callbackMock).toHaveBeenCalledTimes(1);
    expect(callbackMock).toHaveBeenCalledWith('storage');
  });
});
