// web/src/App.tsx
import React from 'react';
import { QueryClientProvider } from '@tanstack/react-query';
import { queryClient } from './lib/queryClient';
import { Router, useLocation, Link } from './router';
import { AuthProvider } from './auth/AuthContext';
import { Header } from './components/layout/Header';
import { Footer } from './components/layout/Footer';
import { BentoCard, BentoCardProps } from './components/bento/BentoCard';
import { BentoGrid } from './components/bento/BentoGrid';
import {
  ComputeCompare,
  StorageCompare,
  NetworkCompare,
  DatabaseCompare,
  DatabaseNoSQLCompare,
  KubernetesCompare,
  ServerlessCompare,
} from './pages/compare';
import { ArrowRight, Server, Database, Network, Box, Cpu, HardDrive } from 'lucide-react';

interface CategoryCardItem {
  title: string;
  desc: string;
  href: string;
  icon: typeof Cpu;
  colSpan: BentoCardProps['colSpan'];
}

const CATEGORY_CARDS: CategoryCardItem[] = [
  {
    title: 'Compute Instances',
    desc: 'Compare virtual machines by vCPU, RAM, and instance families.',
    href: '/compare/compute',
    icon: Cpu,
    colSpan: 4,
  },
  {
    title: 'Storage Classes',
    desc: 'Compare standard, infrequent, and archive object storage.',
    href: '/compare/storage',
    icon: HardDrive,
    colSpan: 4,
  },
  {
    title: 'Network Egress',
    desc: 'Compare internet outbound and inter-region data transfer.',
    href: '/compare/network',
    icon: Network,
    colSpan: 4,
  },
  {
    title: 'Relational DBs (RDBMS)',
    desc: 'Compare PostgreSQL, MySQL, and SQL Server instances with storage & IOPS.',
    href: '/compare/database',
    icon: Database,
    colSpan: 6,
  },
  {
    title: 'NoSQL Databases',
    desc: 'Compare DynamoDB, Cosmos DB, and Firestore throughput & storage.',
    href: '/compare/database-nosql',
    icon: Server,
    colSpan: 6,
  },
  {
    title: 'Kubernetes Control-Plane',
    desc: 'Compare EKS, AKS, and GKE management fees and cluster credit policies.',
    href: '/compare/kubernetes',
    icon: Box,
    colSpan: 6,
  },
  {
    title: 'Serverless Compute (FaaS)',
    desc: 'Compare AWS Lambda, Azure Functions, and GCP Cloud Functions.',
    href: '/compare/serverless',
    icon: Cpu,
    colSpan: 6,
  },
];

export const AppRoutes: React.FC = () => {
  const { pathname } = useLocation();

  switch (pathname) {
    case '/compare/compute':
      return <ComputeCompare />;
    case '/compare/storage':
      return <StorageCompare />;
    case '/compare/network':
      return <NetworkCompare />;
    case '/compare/database':
      return <DatabaseCompare />;
    case '/compare/database-nosql':
      return <DatabaseNoSQLCompare />;
    case '/compare/kubernetes':
      return <KubernetesCompare />;
    case '/compare/serverless':
      return <ServerlessCompare />;
    case '/calculate':
      return (
        <div className="border border-border-default bg-surface-card p-8 space-y-4">
          <span className="text-xs uppercase tracking-widest text-border-accent font-bold">
            Sub-Stage 5.5 &bull; Composite Calculator
          </span>
          <h1 className="font-display text-3xl font-medium text-text-primary">
            Composite Workload Calculator
          </h1>
          <p className="text-xs text-text-secondary max-w-2xl">
            Configure full multi-category architectures across compute, storage, databases, and containers in a single request with strict ADR 0022 Honesty Contract enforcement.
          </p>
        </div>
      );
    case '/status':
      return (
        <div className="border border-border-default bg-surface-card p-8 space-y-4">
          <span className="text-xs uppercase tracking-widest text-border-accent font-bold">
            Sub-Stage 5.6 &bull; Provider Health Status
          </span>
          <h1 className="font-display text-3xl font-medium text-text-primary">
            Provider Freshness & Telemetry
          </h1>
          <p className="text-xs text-text-secondary max-w-2xl">
            Live health telemetry across all 7 supported cloud providers (AWS, Azure, GCP, Oracle, IBM, Alibaba, DigitalOcean).
          </p>
        </div>
      );
    case '/':
      return (
        <div className="space-y-8">
          <div className="border border-border-default bg-surface-card p-8">
            <span className="text-xs uppercase tracking-widest text-border-accent font-bold">
              Sub-Stage 5.4 Active &bull; Compare Engine
            </span>
            <h1 className="font-display text-4xl font-medium text-text-primary mt-2">
              Cloud Pricing Normalization Engine
            </h1>
            <p className="text-sm text-text-secondary mt-2 max-w-2xl">
              Normalized cloud infrastructure pricing across 7 providers. Select a category below or configure query parameters for instant comparisons.
            </p>
          </div>

          <BentoGrid columns={12} gap="md">
            {CATEGORY_CARDS.map((cat) => {
              const Icon = cat.icon;
              return (
                <BentoCard
                  key={cat.title}
                  colSpan={cat.colSpan}
                  header={
                    <div className="flex items-center space-x-2">
                      <Icon className="w-4 h-4 text-border-accent" />
                      <span className="font-display text-lg font-medium text-text-primary">
                        {cat.title}
                      </span>
                    </div>
                  }
                  footer={
                    <Link
                      to={cat.href}
                      className="inline-flex items-center space-x-1.5 text-xs uppercase font-bold tracking-wider text-border-accent hover:text-text-primary transition-colors"
                    >
                      <span>Open Comparison</span>
                      <ArrowRight className="w-3.5 h-3.5" />
                    </Link>
                  }
                >
                  <p className="text-xs text-text-secondary pt-1">{cat.desc}</p>
                </BentoCard>
              );
            })}
          </BentoGrid>
        </div>
      );
    default:
      return (
        <div className="border border-border-default bg-surface-card p-12 text-center space-y-4">
          <span className="text-xs font-mono uppercase text-status-anomaly">404 &bull; Not Found</span>
          <h1 className="font-display text-3xl font-medium text-text-primary">Page Not Found</h1>
          <p className="text-xs text-text-secondary max-w-md mx-auto">
            The requested path <code className="text-border-accent font-mono">{pathname}</code> does not exist.
          </p>
          <div className="pt-2">
            <Link
              to="/"
              className="inline-flex items-center space-x-2 px-4 py-2 text-xs uppercase font-bold tracking-wider text-text-primary border border-border-default bg-surface-raised hover:border-border-accent transition-colors"
            >
              <span>Return Home</span>
            </Link>
          </div>
        </div>
      );
  }
};

export const App: React.FC = () => {
  return (
    <QueryClientProvider client={queryClient}>
      <Router>
        <AuthProvider>
          <div className="min-h-screen flex flex-col bg-surface-page text-text-primary">
            <Header />
            <main className="flex-1 max-w-7xl w-full mx-auto px-4 sm:px-6 lg:px-8 py-8">
              <AppRoutes />
            </main>
            <Footer />
          </div>
        </AuthProvider>
      </Router>
    </QueryClientProvider>
  );
};
