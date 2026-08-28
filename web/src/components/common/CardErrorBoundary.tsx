// web/src/components/common/CardErrorBoundary.tsx
import { Component, ErrorInfo, ReactNode } from 'react';
import { cn } from '../../lib/utils';

interface CardErrorBoundaryProps {
  children: ReactNode;
  onRetry?: () => void;
  className?: string;
}

interface CardErrorBoundaryState {
  hasError: boolean;
  error: Error | null;
}

/**
 * Lightweight per-card error boundary for fault isolation.
 * Prevents rendering errors in individual bento cards from crashing the whole page.
 */
export class CardErrorBoundary extends Component<CardErrorBoundaryProps, CardErrorBoundaryState> {
  public override state: CardErrorBoundaryState = {
    hasError: false,
    error: null,
  };

  public static getDerivedStateFromError(error: Error): CardErrorBoundaryState {
    return { hasError: true, error };
  }

  public override componentDidCatch(error: Error, errorInfo: ErrorInfo): void {
    console.error('CardErrorBoundary caught render exception:', error, errorInfo);
  }

  private handleRetry = (): void => {
    this.setState({ hasError: false, error: null });
    this.props.onRetry?.();
  };

  public override render(): ReactNode {
    if (this.state.hasError) {
      return (
        <div
          className={cn(
            'border border-status-anomaly bg-status-anomaly/5 p-4',
            this.props.className
          )}
          data-testid="card-error-boundary"
        >
          <span className="font-mono text-xs uppercase tracking-wider text-status-anomaly">
            Card Rendering Error
          </span>
          {this.state.error && (
            <p className="text-xs font-mono text-text-secondary mt-2">
              {this.state.error.message}
            </p>
          )}
          {this.props.onRetry && (
            <button
              onClick={this.handleRetry}
              className="border border-border-default bg-surface-card px-3 py-1.5 text-xs uppercase font-bold tracking-wider text-text-primary hover:border-border-accent transition-colors mt-3"
            >
              Retry
            </button>
          )}
        </div>
      );
    }

    return this.props.children;
  }
}
