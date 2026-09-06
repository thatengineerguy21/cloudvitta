// web/src/components/forms/InstanceTypeAutocomplete.tsx
import React, { useState, useMemo, useRef, useEffect } from 'react';
import { useComputeCatalogInstances } from '../../api/queries/useCatalogQueries';
import type { ComputeCatalogItem } from '../../types/api';
import { Search, ChevronDown, Check } from 'lucide-react';
import { cn } from '../../lib/utils';

export interface InstanceTypeAutocompleteProps {
  onSelectInstance: (instance: ComputeCatalogItem) => void;
  selectedInstanceId?: string;
}

export const InstanceTypeAutocomplete: React.FC<InstanceTypeAutocompleteProps> = ({
  onSelectInstance,
  selectedInstanceId,
}) => {
  const [isOpen, setIsOpen] = useState(false);
  const [searchTerm, setSearchTerm] = useState('');
  const wrapperRef = useRef<HTMLDivElement>(null);

  // Fetch top catalog instances
  const { data: catalogData, isLoading } = useComputeCatalogInstances({
    limit: 100,
  });

  const filteredInstances = useMemo(() => {
    const instances = catalogData?.instances || [];
    if (!searchTerm.trim()) {
      return instances.slice(0, 15);
    }
    const term = searchTerm.toLowerCase();
    return instances
      .filter(
        (inst) =>
          inst.instance_type_id.toLowerCase().includes(term) ||
          inst.display_name.toLowerCase().includes(term) ||
          inst.provider.toLowerCase().includes(term) ||
          inst.instance_family.toLowerCase().includes(term)
      )
      .slice(0, 15);
  }, [catalogData?.instances, searchTerm]);

  // Close dropdown on outside click
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (wrapperRef.current && !wrapperRef.current.contains(event.target as Node)) {
        setIsOpen(false);
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  return (
    <div className="relative space-y-1" ref={wrapperRef} data-testid="instance-autocomplete">
      <label className="block text-xs font-mono uppercase tracking-wider text-text-secondary">
        Catalog Preset Autocomplete
      </label>

      <div className="relative">
        <div
          className={cn(
            'w-full flex items-center justify-between border rounded-xl bg-stone-50/80 dark:bg-surface-raised px-3 py-2 text-xs font-mono cursor-pointer transition-all',
            isOpen
              ? 'border-brand-500 ring-2 ring-brand-500/10'
              : 'border-border-default/80 hover:border-text-secondary/60 hover:bg-stone-100/70 dark:hover:bg-surface-card'
          )}
          onClick={() => setIsOpen(!isOpen)}
          data-testid="instance-autocomplete-trigger"
        >
          <span className="truncate text-text-primary">
            {selectedInstanceId ? `Preset: ${selectedInstanceId}` : 'Search / Auto-fill Specs...'}
          </span>
          <ChevronDown className={cn('w-3.5 h-3.5 text-text-secondary transition-transform', isOpen && 'rotate-180')} />
        </div>

        {isOpen && (
          <div
            className="absolute z-50 mt-1.5 w-full bg-surface-card border border-border-default rounded-xl shadow-xl max-h-64 overflow-y-auto backdrop-blur-md"
            data-testid="instance-autocomplete-dropdown"
          >
            <div className="p-2 border-b border-border-default sticky top-0 bg-surface-card flex items-center gap-2">
              <Search className="w-3.5 h-3.5 text-text-secondary" />
              <input
                type="text"
                className="w-full bg-transparent text-xs font-mono text-text-primary outline-none"
                placeholder="Type instance e.g. m6i, c5, D4s..."
                value={searchTerm}
                onChange={(e) => setSearchTerm(e.target.value)}
                autoFocus
                data-testid="instance-autocomplete-input"
              />
            </div>

            {isLoading && (
              <div className="p-3 text-xs text-text-secondary font-mono text-center">
                Loading catalog...
              </div>
            )}

            {!isLoading && filteredInstances.length === 0 && (
              <div className="p-3 text-xs text-text-secondary font-mono text-center">
                No matching instances found
              </div>
            )}

            {!isLoading &&
              filteredInstances.map((inst) => {
                const isSelected = inst.instance_type_id === selectedInstanceId;
                return (
                  <div
                    key={`${inst.provider}-${inst.instance_type_id}`}
                    className={cn(
                      'px-3 py-2 text-xs font-mono cursor-pointer flex items-center justify-between hover:bg-bg-raised border-b border-border-default/40 last:border-b-0',
                      isSelected && 'bg-bg-raised font-bold text-border-accent'
                    )}
                    onClick={() => {
                      onSelectInstance(inst);
                      setIsOpen(false);
                      setSearchTerm('');
                    }}
                    data-testid={`autocomplete-option-${inst.instance_type_id}`}
                  >
                    <div>
                      <div className="flex items-center gap-1.5">
                        <span className="text-[10px] uppercase font-bold text-border-accent border border-border-accent/40 px-1.5 py-0.5 rounded">
                          {inst.provider}
                        </span>
                        <span className="text-text-primary font-semibold">{inst.instance_type_id}</span>
                      </div>
                      <div className="text-[10px] text-text-secondary mt-0.5">
                        {inst.vcpu} vCPU • {inst.memory_gib} GiB RAM • {inst.cpu_architecture}
                      </div>
                    </div>
                    {isSelected && <Check className="w-3.5 h-3.5 text-border-accent" />}
                  </div>
                );
              })}
          </div>
        )}
      </div>
    </div>
  );
};
