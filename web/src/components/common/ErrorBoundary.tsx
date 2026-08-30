import { Component, ErrorInfo, ReactNode } from 'react';

interface Props {
  children: ReactNode;
}

interface State {
  hasError: boolean;
  error: Error | null;
}

export class ErrorBoundary extends Component<Props, State> {
  public override state: State = {
    hasError: false,
    error: null,
  };

  public static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error };
  }

  public override componentDidCatch(error: Error, errorInfo: ErrorInfo): void {
    console.error('Unhandled UI Exception caught by ErrorBoundary:', error, errorInfo);
  }

  public handleReload = (): void => {
    window.location.reload();
  };

  public override render(): ReactNode {
    if (this.state.hasError) {
      return (
        <div className="min-h-screen bg-surface-page text-text-primary flex items-center justify-center p-6">
          <div className="max-w-md w-full bg-surface-card border border-border-default p-8 text-center">
            <h1 className="font-display text-3xl font-medium text-text-primary mb-3">
              Application Exception
            </h1>
            <p className="text-text-secondary text-sm mb-6">
              An unhandled rendering error occurred. The system isolated the exception to protect state integrity.
            </p>
            {this.state.error && (
              <div className="bg-surface-raised border border-border-default p-3 mb-6 text-left overflow-auto max-h-32 text-xs font-mono text-status-anomaly">
                {this.state.error.message}
              </div>
            )}
            <button
              onClick={this.handleReload}
              className="w-full px-4 py-2.5 bg-border-accent text-white font-bold text-sm tracking-wider uppercase hover:opacity-90 transition-opacity"
            >
              Reload View
            </button>
          </div>
        </div>
      );
    }

    return this.props.children;
  }
}
