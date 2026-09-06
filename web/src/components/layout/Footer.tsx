import React from 'react';
import { Github, ExternalLink } from 'lucide-react';
import { Link } from '../../router';

export const Footer: React.FC = () => {
  return (
    <footer className="mt-14 border-t border-border-default/80 bg-surface-card transition-colors">
      <div className="max-w-[1440px] mx-auto px-4 sm:px-6 lg:px-8 py-10">
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-8">
          {/* Brand Summary */}
          <div>
            <div className="flex items-center gap-2.5">
              <div className="w-8 h-8 rounded-xl bg-gradient-to-br from-brand-500 to-amber-600 flex items-center justify-center text-white shadow-sm shadow-brand-500/20">
                <svg className="w-4 h-4 fill-current" viewBox="0 0 24 24" aria-hidden="true">
                  <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 17.93c-3.95-.49-7-3.85-7-7.93 0-.62.08-1.21.21-1.79L9 15v1c0 1.1.9 2 2 2v1.93zm6.9-2.54c-.26-.81-1-1.39-1.9-1.39h-1v-3c0-.55-.45-1-1-1H8v-2h2c.55 0 1-.45 1-1V7h2c1.1 0 2-.9 2-2v-.41c2.93 1.19 5 4.06 5 7.41 0 2.08-.8 3.97-2.1 5.39z" />
                </svg>
              </div>
              <span className="font-brand text-xl font-bold tracking-tight text-text-primary">
                CloudVitta
              </span>
            </div>
            <p className="text-xs text-text-secondary mt-3 leading-relaxed">
              Unified real-time cloud pricing architecture, multi-provider benchmarking, and automated cost arbitration.
            </p>
          </div>

          {/* Benchmark Engines */}
          <div>
            <h4 className="text-xs font-bold uppercase tracking-wider text-text-primary mb-3">
              Benchmark Engines
            </h4>
            <ul className="space-y-2 text-xs text-text-secondary">
              <li>
                <Link to="/compare/compute" className="hover:text-text-primary transition-colors">
                  AWS EC2 vs Azure VMs vs GCP Compute
                </Link>
              </li>
              <li>
                <Link to="/compare/serverless" className="hover:text-text-primary transition-colors">
                  Lambda vs Cloud Functions vs Azure Functions
                </Link>
              </li>
              <li>
                <Link to="/compare/kubernetes" className="hover:text-text-primary transition-colors">
                  Managed Kubernetes (EKS / AKS / GKE)
                </Link>
              </li>
              <li>
                <Link to="/compare/network" className="hover:text-text-primary transition-colors">
                  Egress &amp; Transit Bandwidth
                </Link>
              </li>
            </ul>
          </div>

          {/* Supported Clouds */}
          <div>
            <h4 className="text-xs font-bold uppercase tracking-wider text-text-primary mb-3">
              Supported Clouds
            </h4>
            <ul className="space-y-2 text-xs text-text-secondary">
              <li>Amazon Web Services (AWS)</li>
              <li>Microsoft Azure</li>
              <li>Google Cloud Platform (GCP)</li>
              <li>Oracle Cloud (OCI), IBM, Alibaba, DigitalOcean</li>
            </ul>
          </div>

          {/* Provider Sync Telemetry Card */}
          <div>
            <h4 className="text-xs font-bold uppercase tracking-wider text-text-primary mb-3">
              Provider Sync
            </h4>
            <div className="bg-surface-raised rounded-xl border border-border-default/80 p-3.5 text-xs space-y-2">
              <div className="flex justify-between items-center text-text-secondary">
                <span>Sync Frequency</span>
                <span className="font-semibold text-status-matchExact">Every 15m</span>
              </div>
              <div className="flex justify-between items-center text-text-secondary">
                <span>Global Regions</span>
                <span className="font-semibold text-text-primary">142 Tracked</span>
              </div>
              <div className="flex justify-between items-center text-text-secondary">
                <span>Accuracy SLA</span>
                <span className="font-semibold text-text-primary">99.98%</span>
              </div>
            </div>
          </div>
        </div>

        {/* Bottom Bar */}
        <div className="mt-8 pt-6 border-t border-border-default/60 flex flex-col sm:flex-row items-center justify-between text-xs text-text-secondary gap-3">
          <p>&copy; {new Date().getFullYear()} CloudVitta Technologies Inc. Real-time rates subject to region-specific variances.</p>
          <div className="flex items-center space-x-4">
            <Link to="/status" className="hover:text-text-primary transition-colors">
              Status Page
            </Link>
            <a
              href="/docs/"
              target="_blank"
              rel="noreferrer"
              className="inline-flex items-center gap-1 hover:text-text-primary transition-colors"
            >
              <span>API Access</span>
              <ExternalLink className="w-3 h-3" />
            </a>
            <a
              href="https://github.com/thatengineerguy21/cloudvitta"
              target="_blank"
              rel="noreferrer"
              className="inline-flex items-center gap-1 hover:text-text-primary transition-colors"
            >
              <Github className="w-3.5 h-3.5" />
              <span>GitHub</span>
            </a>
          </div>
        </div>
      </div>
    </footer>
  );
};
