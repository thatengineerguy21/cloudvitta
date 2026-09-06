// web/src/pages/LandingPage.tsx
import React from 'react';
import { Link } from '../router';
import { BentoCard, BentoCardProps } from '../components/bento/BentoCard';
import { BentoGrid } from '../components/bento/BentoGrid';
import {
  useProviderHealthSummary,
  HEALTH_BADGE_COLORS,
} from '../api/queries/useProviderStatusQueries';
import { cn } from '../lib/utils';
import {
  ArrowRight,
  Server,
  Database,
  Network,
  Box,
  Cpu,
  HardDrive,
  Calculator,
  ExternalLink,
  Github,
  Activity,
} from 'lucide-react';

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

/**
 * Landing Page (`/`).
 * Instant load time with zero blocking API dependencies.
 * Presents dual workload entry choices and 7 category comparison cards.
 */
export const LandingPage: React.FC = () => {
  const { summaryState, summaryLabel, isLoading } = useProviderHealthSummary();

  return (
    <div className="space-y-6 relative" data-testid="landing-page">
      {/* Ambient Warm Glow Orbs (Soft Warm Modern / Obsidian Terracotta) */}
      <div className="absolute -top-10 left-1/4 w-96 h-96 bg-brand-500/10 rounded-full blur-3xl pointer-events-none -z-10" />
      <div className="absolute top-20 right-1/4 w-80 h-80 bg-status-matchExact/10 rounded-full blur-3xl pointer-events-none -z-10" />

      {/* Hero Section */}
      <BentoCard colSpan={12} className="relative overflow-hidden">
        <span className="text-xs uppercase tracking-widest text-border-accent font-bold px-2 py-0.5 rounded-full bg-brand-500/10 inline-block w-fit">
          Multi-Cloud Pricing Intelligence
        </span>
        <h1 className="font-display text-4xl sm:text-5xl font-extrabold text-text-primary mt-3 tracking-tight">
          Transparent Cloud Infrastructure Pricing
        </h1>
        <p className="text-sm sm:text-base text-text-secondary mt-3 max-w-2xl leading-relaxed">
          Compare and calculate cloud infrastructure costs across 7 providers. Honest pricing
          with match quality transparency, automatic updates, and zero hidden fees.
        </p>

        {/* Links row */}
        <div className="flex flex-wrap items-center gap-3 mt-6">
          <a
            href="/docs/"
            target="_blank"
            rel="noreferrer"
            className="inline-flex items-center space-x-1.5 px-3.5 py-1.5 text-xs font-semibold text-text-primary border border-border-default/80 bg-surface-raised hover:border-border-accent rounded-xl transition-all shadow-2xs"
          >
            <span>Swagger API Docs</span>
            <ExternalLink className="w-3 h-3" />
          </a>
          <a
            href="https://github.com/thatengineerguy21/cloudvitta"
            target="_blank"
            rel="noreferrer"
            className="inline-flex items-center space-x-1.5 px-3.5 py-1.5 text-xs font-semibold text-text-primary border border-border-default/80 bg-surface-raised hover:border-border-accent rounded-xl transition-all shadow-2xs"
          >
            <Github className="w-3.5 h-3.5" />
            <span>GitHub</span>
          </a>
          <Link
            to="/status"
            className="inline-flex items-center space-x-1.5 px-3.5 py-1.5 text-xs font-semibold border border-border-default/80 bg-surface-raised hover:border-border-accent rounded-xl transition-all shadow-2xs"
          >
            <Activity
              className={cn('w-3.5 h-3.5', HEALTH_BADGE_COLORS[summaryState])}
            />
            <span className={cn(isLoading && 'animate-pulse')}>{summaryLabel}</span>
          </Link>
        </div>
      </BentoCard>

      {/* Dual Workload CTA Cards */}
      <BentoGrid columns={12} gap="md">
        <BentoCard
          colSpan={6}
          isHoverable
          header={
            <div className="flex items-center space-x-2">
              <Cpu className="w-4 h-4 text-border-accent" />
              <span className="font-display text-xl font-medium text-text-primary">
                Compare Individual Category
              </span>
            </div>
          }
          footer={
            <a
              href="#category-grid"
              className="inline-flex items-center space-x-1.5 text-xs uppercase font-bold tracking-wider text-border-accent hover:text-text-primary transition-colors"
            >
              <span>Select Category</span>
              <ArrowRight className="w-3.5 h-3.5" />
            </a>
          }
        >
          <p className="text-xs text-text-secondary pt-1">
            Pick 1 of 7 categories for a cross-cloud pricing matrix across all providers.
          </p>
        </BentoCard>

        <BentoCard
          colSpan={6}
          isHoverable
          header={
            <div className="flex items-center space-x-2">
              <Calculator className="w-4 h-4 text-border-accent" />
              <span className="font-display text-xl font-medium text-text-primary">
                Calculate Composite Workload
              </span>
            </div>
          }
          footer={
            <Link
              to="/calculate"
              className="inline-flex items-center space-x-1.5 text-xs uppercase font-bold tracking-wider text-border-accent hover:text-text-primary transition-colors"
            >
              <span>Build Workload</span>
              <ArrowRight className="w-3.5 h-3.5" />
            </Link>
          }
        >
          <p className="text-xs text-text-secondary pt-1">
            Build a full multi-tier architecture specification and get total hourly cost
            across providers.
          </p>
        </BentoCard>
      </BentoGrid>

      {/* 7 Category Cards */}
      <div id="category-grid" className="scroll-mt-20">
        <BentoGrid columns={12} gap="md">
          {CATEGORY_CARDS.map((cat) => {
            const Icon = cat.icon;
            return (
              <BentoCard
                key={cat.title}
                colSpan={cat.colSpan}
                isHoverable
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
    </div>
  );
};
