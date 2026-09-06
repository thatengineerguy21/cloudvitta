import React from 'react';
import { Github, ExternalLink } from 'lucide-react';
import { Link } from '../../router';

export const Footer: React.FC = () => {
  return (
    <footer className="border-t border-border-default bg-surface-card mt-auto">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        <div className="grid grid-cols-1 md:grid-cols-3 gap-6 items-center">
          {/* Col 1: Brand & Product Statement */}
          <div>
            <div className="font-display text-lg font-bold text-text-primary mb-1">
              CloudVitta
            </div>
            <p className="text-xs text-text-secondary leading-relaxed">
              Transparent, normalized multi-cloud pricing comparison across major hyperscalers and cloud infrastructure providers.
            </p>
          </div>

          {/* Col 2: Supported Providers */}
          <div className="text-center md:text-left">
            <span className="text-xs uppercase tracking-widest text-border-accent font-bold block mb-1">
              Supported Providers
            </span>
            <p className="text-xs text-text-secondary leading-relaxed">
              AWS, Microsoft Azure, Google Cloud Platform, Oracle Cloud, IBM Cloud, Alibaba Cloud, and DigitalOcean.
            </p>
          </div>

          {/* Col 3: Links */}
          <div className="flex items-center justify-start md:justify-end space-x-4">
            <Link
              to="/status"
              className="text-xs uppercase font-bold tracking-wider text-text-secondary hover:text-text-primary transition-colors"
            >
              Provider Status
            </Link>
            <a
              href="/docs/"
              target="_blank"
              rel="noreferrer"
              className="flex items-center space-x-1 text-xs uppercase font-bold tracking-wider text-text-secondary hover:text-text-primary transition-colors"
            >
              <span>API Specs</span>
              <ExternalLink className="w-3 h-3" />
            </a>
            <a
              href="https://github.com/thatengineerguy21/cloudvitta"
              target="_blank"
              rel="noreferrer"
              className="flex items-center space-x-1 text-xs uppercase font-bold tracking-wider text-text-secondary hover:text-text-primary transition-colors"
            >
              <Github className="w-3.5 h-3.5" />
              <span>GitHub</span>
            </a>
          </div>
        </div>

        <div className="mt-8 pt-4 border-t border-border-default flex flex-col sm:flex-row justify-between items-center text-[11px] text-text-secondary">
          <span>&copy; {new Date().getFullYear()} CloudVitta. All rights reserved.</span>
          <span className="mt-2 sm:mt-0">Transparent multi-cloud infrastructure pricing intelligence.</span>
        </div>
      </div>
    </footer>
  );
};
