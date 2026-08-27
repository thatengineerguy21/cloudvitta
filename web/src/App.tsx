import React from 'react';
import { Header } from './components/layout/Header';
import { Footer } from './components/layout/Footer';
import { AuthProvider } from './auth/AuthContext';

export const App: React.FC = () => {
  return (
    <AuthProvider>
      <div className="min-h-screen flex flex-col bg-surface-page text-text-primary">
        <Header />
      
      <main className="flex-1 max-w-7xl w-full mx-auto px-4 sm:px-6 lg:px-8 py-8">
        <div className="border border-border-default bg-surface-card p-8">
          <div className="border-b border-border-default pb-6 mb-6">
            <span className="text-xs uppercase tracking-widest text-border-accent font-bold">
              Sub-Stage 5.1 &bull; Active Shell
            </span>
            <h1 className="font-display text-4xl font-medium text-text-primary mt-2">
              Cloud Pricing Normalization Engine
            </h1>
            <p className="text-sm text-text-secondary mt-2 max-w-2xl">
              Editorial Bento and Obsidian Editorial design system active. High-fidelity comparisons across compute, storage, network, databases, containers, and serverless.
            </p>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            <div className="border border-border-default bg-surface-raised p-6">
              <span className="text-xs font-mono uppercase text-text-secondary">Geometry</span>
              <div className="font-display text-2xl text-text-primary mt-2">0px Border Radius</div>
              <p className="text-xs text-text-secondary mt-1">Harsh rectilinear containers without ambient blurs or soft corners.</p>
            </div>

            <div className="border border-border-default bg-surface-raised p-6">
              <span className="text-xs font-mono uppercase text-text-secondary">Typography</span>
              <div className="font-display text-2xl text-text-primary mt-2">EB Garamond + Manrope</div>
              <p className="text-xs text-text-secondary mt-1">Classical editorial serif headings paired with engineered tabular sans.</p>
            </div>

            <div className="border border-border-default bg-surface-raised p-6">
              <span className="text-xs font-mono uppercase text-text-secondary">Honesty Contract</span>
              <div className="font-display text-2xl text-status-matchExact mt-2">WCAG AA Verified</div>
              <p className="text-xs text-text-secondary mt-1">Distinct high-contrast signals for exact, close, approximate, stale, and partial states.</p>
            </div>
          </div>
        </div>
      </main>

      <Footer />
    </div>
  </AuthProvider>
  );
};
