// web/src/pages/MCPPlaygroundPage.tsx
import React, { useState, useMemo, useCallback } from 'react';
import { useAuth } from '../auth/AuthContext';
import {
  mcpApi,
  DEFAULT_MCP_TOOL_ARGUMENTS,
} from '../api/client';
import type { McpToolDefinition, McpJsonRpcResponse } from '../types';
import { BentoCard } from '../components/bento/BentoCard';
import { BentoGrid } from '../components/bento/BentoGrid';
import { AuthModal } from '../components/auth/AuthModal';
import { MatchQualityBadge } from '../components/honesty/MatchQualityBadge';
import { WarningsBanner } from '../components/honesty/WarningsBanner';
import { ProviderIcon } from '../components/common/ProviderIcon';
import { cn } from '../lib/utils';
import {
  Bot,
  Play,
  RefreshCw,
  Copy,
  Check,
  Terminal,
  Code,
  ShieldCheck,
  Clock,
  Key,
  Layers,
  Eye,
  EyeOff,
  Cpu,
  HardDrive,
  Network,
  Database,
  Server,
  Box,
  Calculator,
  Activity,
  FolderTree,
} from 'lucide-react';

interface ToolMeta {
  name: string;
  category: string;
  label: string;
  description: string;
  icon: typeof Cpu;
}

const MCP_TOOLS_META: ToolMeta[] = [
  {
    name: 'compare_compute',
    category: 'Infrastructure',
    label: 'Compute Instances',
    description: 'Compare virtual machine pricing across 7 cloud providers matching vCPU, RAM, and instance family.',
    icon: Cpu,
  },
  {
    name: 'compare_storage',
    category: 'Infrastructure',
    label: 'Storage Classes',
    description: 'Compare standard, infrequent access, and archive object storage unit costs.',
    icon: HardDrive,
  },
  {
    name: 'compare_network',
    category: 'Infrastructure',
    label: 'Network Egress',
    description: 'Compare outbound internet and intra/inter-region data transfer egress rates.',
    icon: Network,
  },
  {
    name: 'compare_database',
    category: 'Databases',
    label: 'Relational DBs (RDBMS)',
    description: 'Compare managed PostgreSQL, MySQL, and SQL Server instances joining compute and storage candidates.',
    icon: Database,
  },
  {
    name: 'compare_database_nosql',
    category: 'Databases',
    label: 'NoSQL Databases',
    description: 'Compare DynamoDB, Cosmos DB, and Firestore based on storage, read units, and write units.',
    icon: Server,
  },
  {
    name: 'compare_kubernetes',
    category: 'Modern Applications',
    label: 'Kubernetes Control-Plane',
    description: 'Compare EKS, AKS, and GKE cluster management fees with conditional cloud credit netting.',
    icon: Box,
  },
  {
    name: 'compare_serverless',
    category: 'Modern Applications',
    label: 'Serverless Functions (FaaS)',
    description: 'Compare AWS Lambda, Azure Functions, and GCP Cloud Functions invocation and compute durations.',
    icon: Cpu,
  },
  {
    name: 'calculate_workload',
    category: 'Workload & Sizing',
    label: 'Composite Workload Calculator',
    description: 'Run multi-category workload cost estimations across all 7 cloud service categories simultaneously.',
    icon: Calculator,
  },
  {
    name: 'get_provider_status',
    category: 'System Health',
    label: 'Provider Operational Status',
    description: 'Inspect freshness, observation counts, and sync issues for a specific cloud provider.',
    icon: Activity,
  },
  {
    name: 'get_compute_catalog',
    category: 'System Health',
    label: 'Compute Instance Catalog',
    description: 'Query and filter the catalog of normalized VM instance types across providers and regions.',
    icon: FolderTree,
  },
];

export const MCPPlaygroundPage: React.FC = () => {
  const { user, tokens, isAuthenticated } = useAuth();

  // Tabs & Modal state
  const [activeTab, setActiveTab] = useState<'runner' | 'config'>('runner');
  const [isAuthModalOpen, setIsAuthModalOpen] = useState(false);

  // Runner state
  const [selectedTool, setSelectedTool] = useState<string>('compare_compute');
  const [jsonArgs, setJsonArgs] = useState<string>(() =>
    JSON.stringify(DEFAULT_MCP_TOOL_ARGUMENTS['compare_compute'] || {}, null, 2)
  );
  const [jsonError, setJsonError] = useState<string | null>(null);
  const [isExecuting, setIsExecuting] = useState<boolean>(false);
  const [isListingTools, setIsListingTools] = useState<boolean>(false);
  const [liveTools, setLiveTools] = useState<McpToolDefinition[] | null>(null);

  const [latencyMs, setLatencyMs] = useState<number | null>(null);
  const [httpStatus, setHttpStatus] = useState<number | null>(null);
  const [rawResponse, setRawResponse] = useState<McpJsonRpcResponse | null>(null);
  const [parsedToolResult, setParsedToolResult] = useState<unknown>(null);
  const [executionError, setExecutionError] = useState<string | null>(null);
  const [viewMode, setViewMode] = useState<'formatted' | 'raw'>('formatted');

  // Config tab copy states
  const [tokenCopied, setTokenCopied] = useState<boolean>(false);
  const [configCopied, setConfigCopied] = useState<string | null>(null);
  const [showToken, setShowToken] = useState<boolean>(false);

  // When tool selection changes, update preset JSON arguments
  const handleSelectTool = useCallback((toolName: string) => {
    setSelectedTool(toolName);
    const defaultArgs = DEFAULT_MCP_TOOL_ARGUMENTS[toolName] || {};
    setJsonArgs(JSON.stringify(defaultArgs, null, 2));
    setJsonError(null);
  }, []);

  // Validate JSON on change
  const handleJsonChange = (val: string) => {
    setJsonArgs(val);
    try {
      JSON.parse(val);
      setJsonError(null);
    } catch (err: unknown) {
      setJsonError((err as Error).message);
    }
  };

  // Reset arguments to preset
  const handleResetArgs = () => {
    const defaultArgs = DEFAULT_MCP_TOOL_ARGUMENTS[selectedTool] || {};
    setJsonArgs(JSON.stringify(defaultArgs, null, 2));
    setJsonError(null);
  };

  // Execute tools/list
  const handleFetchToolsList = async () => {
    setIsListingTools(true);
    setExecutionError(null);
    const start = performance.now();
    try {
      const res = await mcpApi.listTools();
      const elapsed = Math.round(performance.now() - start);
      setLatencyMs(elapsed);
      setHttpStatus(200);
      setRawResponse(res);
      setParsedToolResult(res.result?.tools || null);
      if (res.result?.tools) {
        setLiveTools(res.result.tools);
      }
    } catch (err: unknown) {
      const elapsed = Math.round(performance.now() - start);
      setLatencyMs(elapsed);
      const errMsg = (err as Error).message || 'Failed to list MCP tools';
      setExecutionError(errMsg);
      setHttpStatus(500);
    } finally {
      setIsListingTools(false);
    }
  };

  // Execute tool call
  const handleExecuteTool = async () => {
    let parsedArgs: Record<string, unknown>;
    try {
      parsedArgs = JSON.parse(jsonArgs);
    } catch {
      setJsonError('Invalid JSON arguments payload. Please correct syntax.');
      return;
    }

    setIsExecuting(true);
    setExecutionError(null);
    const start = performance.now();

    try {
      const res = await mcpApi.callTool(selectedTool, parsedArgs);
      const elapsed = Math.round(performance.now() - start);
      setLatencyMs(elapsed);
      setHttpStatus(200);
      setRawResponse(res);

      if (res.error) {
        setExecutionError(res.error.message || 'Tool returned JSON-RPC error');
        setParsedToolResult(null);
      } else if (res.result?.content && res.result.content.length > 0) {
        const firstItem = res.result.content[0];
        if (firstItem && 'text' in firstItem && typeof firstItem.text === 'string') {
          try {
            const innerParsed = JSON.parse(firstItem.text);
            setParsedToolResult(innerParsed);
          } catch {
            setParsedToolResult(firstItem.text);
          }
        } else {
          setParsedToolResult(res.result);
        }
      } else {
        setParsedToolResult(res.result || null);
      }
    } catch (err: unknown) {
      const elapsed = Math.round(performance.now() - start);
      setLatencyMs(elapsed);
      const errMsg = (err as Error).message || 'Tool execution failed';
      setExecutionError(errMsg);
      setHttpStatus(500);
    } finally {
      setIsExecuting(false);
    }
  };

  // Copy helper
  const handleCopy = (text: string, type: string) => {
    navigator.clipboard.writeText(text);
    if (type === 'token') {
      setTokenCopied(true);
      setTimeout(() => setTokenCopied(false), 2000);
    } else {
      setConfigCopied(type);
      setTimeout(() => setConfigCopied(null), 2000);
    }
  };

  // Generate connection snippets
  const origin = typeof window !== 'undefined' ? window.location.origin : 'https://cloudvitta.dev';
  const mcpEndpoint = `${origin}/mcp`;
  const accessToken = tokens?.accessToken || '<YOUR_ACCESS_TOKEN>';

  const claudeDesktopConfig = useMemo(() => {
    return JSON.stringify(
      {
        mcpServers: {
          cloudvitta: {
            command: 'npx',
            args: [
              '-y',
              'mcp-remote',
              mcpEndpoint,
              '--header',
              `Authorization: Bearer ${accessToken}`,
            ],
          },
        },
      },
      null,
      2
    );
  }, [mcpEndpoint, accessToken]);

  const cursorConfig = useMemo(() => {
    return JSON.stringify(
      {
        name: 'cloudvitta',
        type: 'sse',
        url: mcpEndpoint,
        headers: {
          Authorization: `Bearer ${accessToken}`,
        },
      },
      null,
      2
    );
  }, [mcpEndpoint, accessToken]);

  const curlSnippet = useMemo(() => {
    return `curl -X POST "${mcpEndpoint}" \\
  -H "Authorization: Bearer ${accessToken}" \\
  -H "Content-Type: application/json" \\
  -H "Accept: application/json, text/event-stream" \\
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "tools/call",
    "params": {
      "name": "${selectedTool}",
      "arguments": ${JSON.stringify(DEFAULT_MCP_TOOL_ARGUMENTS[selectedTool] || {}, null, 6).replace(
        /\n/g,
        '\n      '
      )}
    }
  }'`;
  }, [mcpEndpoint, accessToken, selectedTool]);

  const activeToolMeta = useMemo(() => {
    return (
      MCP_TOOLS_META.find((t) => t.name === selectedTool) || {
        name: selectedTool,
        category: 'Custom',
        label: selectedTool,
        description: 'Selected MCP tool',
        icon: Bot,
      }
    );
  }, [selectedTool]);

  return (
    <div className="space-y-6 relative" data-testid="mcp-playground-page">
      {/* Ambient background blur */}
      <div className="absolute inset-0 overflow-hidden pointer-events-none -z-10" aria-hidden="true">
        <div className="absolute -top-10 left-1/3 w-96 h-96 bg-brand-500/10 rounded-full blur-3xl" />
      </div>

      {/* Page Title & Status Header */}
      <div className="flex flex-col md:flex-row md:items-center justify-between gap-4 border-b border-border-default pb-6">
        <div>
          <div className="flex items-center gap-2 mb-1.5">
            <span className="text-[11px] uppercase tracking-widest text-border-accent font-bold px-2 py-0.5 rounded-full bg-brand-500/10 inline-flex items-center gap-1.5">
              <Bot className="w-3 h-3 text-border-accent" />
              Streamable HTTP • JSON-RPC 2.0
            </span>
            <span className="text-[11px] text-text-muted font-mono">POST /mcp</span>
          </div>
          <h1 className="font-display text-3xl sm:text-4xl font-extrabold text-text-primary tracking-tight">
            MCP Playground & AI Agent Console
          </h1>
          <p className="text-sm text-text-secondary mt-1 max-w-2xl">
            Test and inspect CloudVitta&apos;s 10 Model Context Protocol tools directly in the browser,
            or export live agent configurations for Claude Desktop, Cursor, and terminal scripts.
          </p>
        </div>

        {/* Auth status badge */}
        <div className="flex items-center gap-3 shrink-0">
          {isAuthenticated && user ? (
            <div className="flex items-center space-x-2 px-3 py-1.5 rounded-xl border border-emerald-500/30 bg-emerald-500/10 text-xs font-semibold text-text-primary">
              <span className="relative flex h-2 w-2">
                <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"></span>
                <span className="relative inline-flex rounded-full h-2 w-2 bg-emerald-500"></span>
              </span>
              <span className="truncate max-w-[160px]">{user.email}</span>
              <span className="text-[10px] uppercase font-mono px-1.5 py-0.2 rounded bg-emerald-500/20 text-emerald-800 dark:text-emerald-300 font-bold">
                Active
              </span>
            </div>
          ) : (
            <button
              type="button"
              onClick={() => setIsAuthModalOpen(true)}
              className="flex items-center space-x-1.5 text-xs font-bold text-white bg-brand-500 hover:bg-brand-600 px-3.5 py-2 rounded-xl shadow-sm transition-all focus-visible:ring-1 focus-visible:ring-border-accent focus-visible:outline-none"
            >
              <Key className="w-3.5 h-3.5" />
              <span>Sign In to Launch Console</span>
            </button>
          )}
        </div>
      </div>

      {/* Unauthenticated Notification & Overview Gate */}
      {!isAuthenticated && (
        <div className="space-y-6" data-testid="unauthenticated-mcp-gate">
          <BentoCard colSpan={12} className="border-border-accent/40 bg-surface-card relative overflow-hidden">
            <div className="flex flex-col md:flex-row items-start md:items-center justify-between gap-6 p-2">
              <div className="space-y-2">
                <div className="flex items-center gap-2">
                  <ShieldCheck className="w-5 h-5 text-border-accent" />
                  <h2 className="text-lg font-bold text-text-primary">
                    Authentication Required for Model Context Protocol
                  </h2>
                </div>
                <p className="text-xs sm:text-sm text-text-secondary max-w-2xl leading-relaxed">
                  CloudVitta protects the <code className="px-1 py-0.5 rounded bg-surface-raised font-mono text-border-accent">/mcp</code> endpoint
                  with standard-tier token authentication to prevent infinite agent recursion loops, ensure predictable rate limits (120 req/min),
                  and maintain deterministic pricing cache guarantees.
                </p>
              </div>
              <button
                type="button"
                onClick={() => setIsAuthModalOpen(true)}
                className="shrink-0 flex items-center space-x-2 px-4 py-2.5 rounded-xl bg-brand-500 hover:bg-brand-600 text-white text-xs font-bold shadow-md shadow-brand-500/20 transition-all focus-visible:ring-1 focus-visible:ring-border-accent focus-visible:outline-none cursor-pointer"
              >
                <Key className="w-4 h-4" />
                <span>Sign In / Create Account</span>
              </button>
            </div>
          </BentoCard>

          {/* 10 MCP Tools Grid Overview */}
          <div>
            <div className="flex items-center justify-between mb-4">
              <h3 className="text-sm uppercase tracking-wider font-bold text-text-secondary flex items-center gap-2">
                <Layers className="w-4 h-4 text-border-accent" />
                Available MCP Agent Tools (10 Tools)
              </h3>
              <span className="text-xs text-text-muted">Streamable HTTP Transport</span>
            </div>

            <BentoGrid columns={12} gap="md">
              {MCP_TOOLS_META.map((t) => {
                const IconComponent = t.icon;
                return (
                  <BentoCard
                    key={t.name}
                    colSpan={6}
                    isHoverable
                    className="p-4"
                  >
                    <div className="flex items-start space-x-3">
                      <div className="p-2 rounded-xl bg-surface-raised border border-border-default shrink-0 text-border-accent">
                        <IconComponent className="w-4 h-4" />
                      </div>
                      <div className="space-y-1">
                        <div className="flex items-center gap-2">
                          <h4 className="text-xs font-bold text-text-primary">{t.label}</h4>
                          <span className="text-[10px] font-mono text-border-accent px-1.5 py-0.2 rounded bg-surface-raised border border-border-default/60">
                            {t.name}
                          </span>
                        </div>
                        <p className="text-[11px] text-text-secondary leading-normal">{t.description}</p>
                      </div>
                    </div>
                  </BentoCard>
                );
              })}
            </BentoGrid>
          </div>
        </div>
      )}

      {/* Authenticated Workspace */}
      {isAuthenticated && (
        <div className="space-y-6" data-testid="authenticated-mcp-workspace">
          {/* Navigation Mode Switcher */}
          <div className="flex items-center justify-between border-b border-border-default pb-3">
            <div className="flex items-center space-x-2">
              <button
                type="button"
                onClick={() => setActiveTab('runner')}
                className={cn(
                  'flex items-center space-x-2 px-3.5 py-1.5 rounded-xl text-xs font-bold transition-all focus-visible:ring-1 focus-visible:ring-border-accent focus-visible:outline-none',
                  activeTab === 'runner'
                    ? 'bg-brand-500 text-white shadow-sm shadow-brand-500/20'
                    : 'text-text-secondary hover:text-text-primary hover:bg-surface-card'
                )}
              >
                <Terminal className="w-3.5 h-3.5" />
                <span>Interactive Tool Runner</span>
              </button>

              <button
                type="button"
                onClick={() => setActiveTab('config')}
                className={cn(
                  'flex items-center space-x-2 px-3.5 py-1.5 rounded-xl text-xs font-bold transition-all focus-visible:ring-1 focus-visible:ring-border-accent focus-visible:outline-none',
                  activeTab === 'config'
                    ? 'bg-brand-500 text-white shadow-sm shadow-brand-500/20'
                    : 'text-text-secondary hover:text-text-primary hover:bg-surface-card'
                )}
              >
                <Code className="w-3.5 h-3.5" />
                <span>Agent Setup & Configs</span>
              </button>
            </div>

            <button
              type="button"
              onClick={handleFetchToolsList}
              disabled={isListingTools}
              className="flex items-center space-x-1.5 text-xs font-semibold text-text-secondary hover:text-text-primary px-3 py-1.5 rounded-xl border border-border-default bg-surface-card hover:border-border-accent transition-colors disabled:opacity-50"
              title="Query tools/list to verify tool registry"
            >
              <RefreshCw className={cn('w-3.5 h-3.5', isListingTools && 'animate-spin')} />
              <span>{isListingTools ? 'Querying...' : 'Ping tools/list'}</span>
            </button>
          </div>

          {/* TAB 1: INTERACTIVE TOOL RUNNER */}
          {activeTab === 'runner' && (
            <div className="grid grid-cols-1 lg:grid-cols-12 gap-6 items-start">
              {/* Left Column: Tool Selector & Request Builder (5 cols) */}
              <div className="lg:col-span-5 space-y-4">
                <BentoCard colSpan={12} className="p-4 space-y-4">
                  {/* Tool Selection Dropdown */}
                  <div>
                    <label htmlFor="tool-select" className="block text-xs font-bold uppercase tracking-wider text-text-secondary mb-1.5">
                      Select MCP Tool
                    </label>
                    <select
                      id="tool-select"
                      value={selectedTool}
                      onChange={(e) => handleSelectTool(e.target.value)}
                      className="w-full text-xs font-semibold rounded-xl bg-surface-raised border border-border-default px-3 py-2 text-text-primary focus:border-border-accent focus-visible:ring-1 focus-visible:ring-border-accent focus-visible:outline-none transition-colors cursor-pointer"
                    >
                      {MCP_TOOLS_META.map((t) => (
                        <option key={t.name} value={t.name}>
                          {t.name} ({t.label})
                        </option>
                      ))}
                    </select>
                  </div>

                  {/* Active Tool Info Banner */}
                  <div className="p-3 rounded-xl bg-surface-raised border border-border-default/80 space-y-1.5">
                    <div className="flex items-center justify-between">
                      <div className="flex items-center space-x-2">
                        {React.createElement(activeToolMeta.icon, {
                          className: 'w-4 h-4 text-border-accent',
                        })}
                        <span className="text-xs font-bold text-text-primary">{activeToolMeta.label}</span>
                      </div>
                      <span className="text-[10px] uppercase font-mono px-1.5 py-0.5 rounded bg-surface-card border border-border-default text-text-muted">
                        {activeToolMeta.category}
                      </span>
                    </div>
                    <p className="text-[11px] text-text-secondary leading-relaxed">
                      {activeToolMeta.description}
                    </p>
                  </div>

                  {/* JSON Arguments Editor */}
                  <div>
                    <div className="flex items-center justify-between mb-1.5">
                      <label htmlFor="mcp-args" className="text-xs font-bold uppercase tracking-wider text-text-secondary">
                        JSON Arguments (<code className="lowercase font-mono text-border-accent">arguments</code>)
                      </label>
                      <button
                        type="button"
                        onClick={handleResetArgs}
                        className="text-[11px] font-semibold text-border-accent hover:underline flex items-center gap-1"
                      >
                        <RefreshCw className="w-3 h-3" />
                        <span>Reset Preset</span>
                      </button>
                    </div>

                    <textarea
                      id="mcp-args"
                      rows={9}
                      value={jsonArgs}
                      onChange={(e) => handleJsonChange(e.target.value)}
                      className={cn(
                        'w-full font-mono text-xs p-3 rounded-xl bg-surface-raised border text-text-primary focus-visible:outline-none transition-colors resize-y',
                        jsonError
                          ? 'border-status-anomaly focus-visible:ring-1 focus-visible:ring-status-anomaly'
                          : 'border-border-default focus:border-border-accent focus-visible:ring-1 focus-visible:ring-border-accent'
                      )}
                      placeholder='{ "region": "us-east-1" }'
                    />

                    {jsonError && (
                      <div className="mt-1.5 text-xs text-status-anomaly flex items-center gap-1 font-semibold">
                        <span>Syntax Error: {jsonError}</span>
                      </div>
                    )}
                  </div>

                  {/* Execute Button */}
                  <button
                    type="button"
                    onClick={handleExecuteTool}
                    disabled={isExecuting || !!jsonError}
                    className="w-full flex items-center justify-center space-x-2 py-2.5 rounded-xl bg-brand-500 hover:bg-brand-600 text-white font-bold text-xs shadow-sm shadow-brand-500/20 transition-all disabled:opacity-50 disabled:cursor-not-allowed focus-visible:ring-1 focus-visible:ring-border-accent focus-visible:outline-none cursor-pointer"
                  >
                    {isExecuting ? (
                      <>
                        <RefreshCw className="w-4 h-4 animate-spin" />
                        <span>Invoking {selectedTool}...</span>
                      </>
                    ) : (
                      <>
                        <Play className="w-4 h-4 fill-current" />
                        <span>Execute Tool Call</span>
                      </>
                    )}
                  </button>
                </BentoCard>

                {/* Discovered Tools List (from tools/list if available) */}
                {liveTools && liveTools.length > 0 && (
                  <BentoCard colSpan={12} className="p-3.5">
                    <span className="text-[11px] font-bold uppercase tracking-wider text-text-secondary block mb-2">
                      Live Server Tool Registry ({liveTools.length} tools)
                    </span>
                    <div className="flex flex-wrap gap-1.5 max-h-36 overflow-y-auto pr-1">
                      {liveTools.map((lt) => (
                        <button
                          key={lt.name}
                          type="button"
                          onClick={() => handleSelectTool(lt.name)}
                          className={cn(
                            'text-[10px] font-mono px-2 py-1 rounded-lg border transition-all cursor-pointer',
                            selectedTool === lt.name
                              ? 'border-border-accent bg-brand-500/10 text-border-accent font-bold'
                              : 'border-border-default bg-surface-raised text-text-secondary hover:text-text-primary'
                          )}
                        >
                          {lt.name}
                        </button>
                      ))}
                    </div>
                  </BentoCard>
                )}
              </div>

              {/* Right Column: Execution Output & Response Viewer (7 cols) */}
              <div className="lg:col-span-7 space-y-4">
                <BentoCard colSpan={12} className="p-4 space-y-4">
                  {/* Response Meta Header Bar */}
                  <div className="flex flex-wrap items-center justify-between gap-3 border-b border-border-default pb-3">
                    <div className="flex items-center space-x-3">
                      {httpStatus !== null ? (
                        <span
                          className={cn(
                            'text-xs font-bold px-2 py-0.5 rounded-full border',
                            httpStatus === 200
                              ? 'bg-emerald-500/10 text-emerald-800 dark:text-emerald-300 border-emerald-500/30'
                              : 'bg-status-anomaly/10 text-status-anomaly border-status-anomaly/30'
                          )}
                        >
                          HTTP {httpStatus}
                        </span>
                      ) : (
                        <span className="text-xs font-semibold text-text-muted px-2 py-0.5 rounded-full bg-surface-raised border border-border-default">
                          Idle / Ready
                        </span>
                      )}

                      {latencyMs !== null && (
                        <span className="flex items-center space-x-1 text-xs text-text-secondary font-mono">
                          <Clock className="w-3 h-3 text-text-muted" />
                          <span>{latencyMs}ms</span>
                        </span>
                      )}
                    </div>

                    {/* View Switcher & Copy */}
                    <div className="flex items-center space-x-2">
                      <div className="flex items-center bg-surface-raised p-0.5 rounded-xl border border-border-default text-xs">
                        <button
                          type="button"
                          onClick={() => setViewMode('formatted')}
                          className={cn(
                            'px-2.5 py-1 rounded-lg font-semibold transition-all',
                            viewMode === 'formatted'
                              ? 'bg-surface-card text-text-primary shadow-2xs'
                              : 'text-text-muted hover:text-text-primary'
                          )}
                        >
                          Formatted
                        </button>
                        <button
                          type="button"
                          onClick={() => setViewMode('raw')}
                          className={cn(
                            'px-2.5 py-1 rounded-lg font-semibold transition-all',
                            viewMode === 'raw'
                              ? 'bg-surface-card text-text-primary shadow-2xs'
                              : 'text-text-muted hover:text-text-primary'
                          )}
                        >
                          Raw JSON
                        </button>
                      </div>

                      <button
                        type="button"
                        onClick={() => handleCopy(JSON.stringify(rawResponse, null, 2), 'raw_response')}
                        disabled={!rawResponse}
                        className="flex items-center space-x-1 text-xs font-semibold px-2.5 py-1.5 rounded-xl border border-border-default bg-surface-raised hover:border-border-accent text-text-secondary hover:text-text-primary transition-colors disabled:opacity-40"
                        title="Copy Response JSON"
                      >
                        {configCopied === 'raw_response' ? (
                          <>
                            <Check className="w-3.5 h-3.5 text-emerald-500" />
                            <span>Copied</span>
                          </>
                        ) : (
                          <>
                            <Copy className="w-3.5 h-3.5" />
                            <span>Copy</span>
                          </>
                        )}
                      </button>
                    </div>
                  </div>

                  {/* Error Notification */}
                  {executionError && (
                    <div className="p-3 rounded-xl bg-status-anomaly/10 border border-status-anomaly/30 text-status-anomaly text-xs space-y-1">
                      <span className="font-bold flex items-center gap-1.5">
                        <ShieldCheck className="w-4 h-4" /> Tool Execution Error
                      </span>
                      <p className="font-mono">{executionError}</p>
                    </div>
                  )}

                  {/* Empty state before invocation */}
                  {!rawResponse && !executionError && (
                    <div className="py-16 text-center space-y-3">
                      <div className="w-12 h-12 rounded-2xl bg-surface-raised border border-border-default flex items-center justify-center text-text-muted mx-auto">
                        <Bot className="w-6 h-6 text-border-accent" />
                      </div>
                      <div className="space-y-1">
                        <h4 className="text-sm font-bold text-text-primary">No Response Received Yet</h4>
                        <p className="text-xs text-text-secondary max-w-sm mx-auto">
                          Select an MCP tool on the left, edit its JSON arguments, and click &quot;Execute Tool Call&quot;
                          to inspect live results.
                        </p>
                      </div>
                    </div>
                  )}

                  {/* Formatted Response View */}
                  {viewMode === 'formatted' && Boolean(parsedToolResult) && (
                    <div className="space-y-4 text-xs">
                      {/* Honesty Warnings Banner if present */}
                      {typeof parsedToolResult === 'object' &&
                        parsedToolResult !== null &&
                        'warnings' in parsedToolResult &&
                        Array.isArray((parsedToolResult as { warnings?: unknown[] }).warnings) &&
                        (parsedToolResult as { warnings: unknown[] }).warnings.length > 0 && (
                          <WarningsBanner
                            warnings={
                              (parsedToolResult as { warnings: Array<{ provider: string; code: string; message: string }> })
                                .warnings
                            }
                          />
                        )}

                      {/* If response contains a results array (Compute, Storage, Network, Database, etc.) */}
                      {typeof parsedToolResult === 'object' &&
                        parsedToolResult !== null &&
                        'results' in parsedToolResult &&
                        Array.isArray((parsedToolResult as { results?: unknown[] }).results) && (
                          <div className="space-y-2">
                            <div className="flex items-center justify-between text-text-muted text-[11px]">
                              <span>
                                Matches found: {(parsedToolResult as { results: unknown[] }).results.length} providers
                              </span>
                              {(parsedToolResult as { results: Array<{ normalized_hourly_usd?: unknown }> }).results.length > 0 && (
                                <span>Sorted by normalized hourly USD</span>
                              )}
                            </div>

                            <div className="divide-y divide-border-default border border-border-default rounded-xl bg-surface-raised overflow-hidden">
                              {(
                                parsedToolResult as {
                                  results: Array<{
                                    provider: string;
                                    sku_id?: string;
                                    match_quality?: string;
                                    normalized_hourly_usd?: string | number;
                                    price?: { amount?: string | number; currency?: string; unit?: string };
                                  }>;
                                }
                              ).results.map((item, idx) => (
                                <div key={idx} className="p-3 flex items-center justify-between gap-3">
                                  <div className="flex items-center space-x-2.5 min-w-0">
                                    <ProviderIcon provider={item.provider} className="w-5 h-5 shrink-0" />
                                    <div className="min-w-0">
                                      <span className="font-bold text-text-primary uppercase tracking-wide">
                                        {item.provider}
                                      </span>
                                      {item.sku_id && (
                                        <p className="text-[10px] font-mono text-text-muted truncate max-w-[200px]">
                                          {item.sku_id}
                                        </p>
                                      )}
                                    </div>
                                  </div>

                                  <div className="flex items-center space-x-3 shrink-0">
                                    {item.match_quality && item.match_quality !== 'none' && (
                                      <MatchQualityBadge
                                        quality={
                                          item.match_quality as 'exact' | 'close' | 'approximate'
                                        }
                                      />
                                    )}

                                    {item.normalized_hourly_usd !== undefined && (
                                      <div className="text-right">
                                        <span className="font-mono font-bold text-text-primary text-sm">
                                          ${Number(item.normalized_hourly_usd).toFixed(4)}
                                        </span>
                                        <span className="text-[10px] text-text-muted block">/ hour</span>
                                      </div>
                                    )}
                                  </div>
                                </div>
                              ))}
                            </div>
                          </div>
                        )}

                      {/* If response is composite calculate */}
                      {typeof parsedToolResult === 'object' &&
                        parsedToolResult !== null &&
                        'categories' in parsedToolResult && (
                          <div className="p-4 rounded-xl border border-border-default bg-surface-raised space-y-3">
                            <div className="flex items-center justify-between border-b border-border-default pb-2">
                              <span className="font-bold uppercase tracking-wider text-text-secondary text-[11px]">
                                Composite Calculation
                              </span>
                              {'partial' in parsedToolResult && (parsedToolResult as { partial: boolean }).partial && (
                                <span className="text-[10px] font-bold text-status-anomaly px-2 py-0.5 rounded-full border border-status-anomaly/30 bg-status-anomaly/10">
                                  Partial Workload
                                </span>
                              )}
                            </div>

                            {'total_normalized_hourly_usd' in parsedToolResult &&
                              (parsedToolResult as { total_normalized_hourly_usd?: string | number })
                                .total_normalized_hourly_usd && (
                                <div>
                                  <span className="text-[11px] text-text-muted">Total Hourly Cost (USD):</span>
                                  <p className="text-2xl font-mono font-extrabold text-text-primary">
                                    $
                                    {Number(
                                      (parsedToolResult as { total_normalized_hourly_usd: string | number })
                                        .total_normalized_hourly_usd
                                    ).toFixed(4)}
                                    <span className="text-xs font-normal text-text-muted"> / hr</span>
                                  </p>
                                </div>
                              )}
                          </div>
                        )}

                      {/* If response does not match the above tabular patterns, render structured inspect list */}
                      {typeof parsedToolResult === 'object' &&
                        parsedToolResult !== null &&
                        !('results' in parsedToolResult) &&
                        !('categories' in parsedToolResult) && (
                          <pre className="p-3 rounded-xl bg-surface-raised border border-border-default font-mono text-xs overflow-x-auto text-text-primary max-h-96">
                            {JSON.stringify(parsedToolResult, null, 2)}
                          </pre>
                        )}
                    </div>
                  )}

                  {/* Raw JSON-RPC View */}
                  {viewMode === 'raw' && Boolean(rawResponse) && (
                    <pre className="p-3.5 rounded-xl bg-surface-raised border border-border-default font-mono text-xs overflow-x-auto text-text-primary max-h-[480px] leading-relaxed">
                      {JSON.stringify(rawResponse, null, 2)}
                    </pre>
                  )}
                </BentoCard>
              </div>
            </div>
          )}

          {/* TAB 2: AGENT SETUP & CONFIGS */}
          {activeTab === 'config' && (
            <div className="space-y-6">
              {/* Active Token Card */}
              <BentoCard colSpan={12} className="p-4 space-y-3">
                <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 border-b border-border-default pb-3">
                  <div className="flex items-center space-x-2">
                    <Key className="w-4 h-4 text-border-accent" />
                    <h3 className="text-xs font-bold uppercase tracking-wider text-text-primary">
                      Active Bearer Access Token
                    </h3>
                  </div>
                  <div className="flex items-center space-x-2">
                    <button
                      type="button"
                      onClick={() => setShowToken(!showToken)}
                      className="flex items-center space-x-1 text-xs font-semibold text-text-secondary hover:text-text-primary px-2.5 py-1 rounded-xl border border-border-default bg-surface-raised transition-colors"
                    >
                      {showToken ? <EyeOff className="w-3.5 h-3.5" /> : <Eye className="w-3.5 h-3.5" />}
                      <span>{showToken ? 'Hide' : 'Reveal'}</span>
                    </button>
                    <button
                      type="button"
                      onClick={() => handleCopy(accessToken, 'token')}
                      className="flex items-center space-x-1 text-xs font-bold text-white bg-brand-500 hover:bg-brand-600 px-3 py-1 rounded-xl shadow-xs transition-colors"
                    >
                      {tokenCopied ? (
                        <>
                          <Check className="w-3.5 h-3.5" />
                          <span>Copied!</span>
                        </>
                      ) : (
                        <>
                          <Copy className="w-3.5 h-3.5" />
                          <span>Copy Token</span>
                        </>
                      )}
                    </button>
                  </div>
                </div>

                <div className="p-3 rounded-xl bg-surface-raised border border-border-default font-mono text-xs break-all text-text-primary select-all">
                  {showToken
                    ? accessToken
                    : accessToken.slice(0, 16) + '••••••••••••••••••••••••••••••••••••••••••••••••'}
                </div>

                <p className="text-[11px] text-text-muted leading-relaxed">
                  JWT access tokens carry a 15-minute lifespan. In-browser requests refresh automatically via HTTP-only
                  rotation. For long-running local scripts or IDE agents, retrieve a fresh token here or invoke the{' '}
                  <code className="px-1 py-0.5 bg-surface-raised rounded text-border-accent">/api/v1/auth/login</code>{' '}
                  endpoint programmatically.
                </p>
              </BentoCard>

              {/* Claude Desktop & Cursor Snippets Grid */}
              <BentoGrid columns={12} gap="md">
                {/* Claude Desktop Config */}
                <BentoCard colSpan={6} className="p-4 space-y-3">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center space-x-2">
                      <Bot className="w-4 h-4 text-border-accent" />
                      <h4 className="text-xs font-bold text-text-primary">Claude Desktop Configuration</h4>
                    </div>
                    <button
                      type="button"
                      onClick={() => handleCopy(claudeDesktopConfig, 'claude')}
                      className="text-xs font-semibold text-border-accent hover:underline flex items-center gap-1"
                    >
                      {configCopied === 'claude' ? (
                        <>
                          <Check className="w-3.5 h-3.5 text-emerald-500" />
                          <span>Copied</span>
                        </>
                      ) : (
                        <>
                          <Copy className="w-3.5 h-3.5" />
                          <span>Copy</span>
                        </>
                      )}
                    </button>
                  </div>

                  <p className="text-[11px] text-text-secondary">
                    Add this to your <code className="font-mono text-text-primary">claude_desktop_config.json</code> to
                    equip Claude Desktop with CloudVitta pricing tools via Streamable HTTP remote bridge:
                  </p>

                  <pre className="p-3 rounded-xl bg-surface-raised border border-border-default font-mono text-[11px] overflow-x-auto text-text-primary leading-relaxed">
                    {claudeDesktopConfig}
                  </pre>
                </BentoCard>

                {/* Cursor IDE Config */}
                <BentoCard colSpan={6} className="p-4 space-y-3">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center space-x-2">
                      <Code className="w-4 h-4 text-border-accent" />
                      <h4 className="text-xs font-bold text-text-primary">Cursor Settings Configuration</h4>
                    </div>
                    <button
                      type="button"
                      onClick={() => handleCopy(cursorConfig, 'cursor')}
                      className="text-xs font-semibold text-border-accent hover:underline flex items-center gap-1"
                    >
                      {configCopied === 'cursor' ? (
                        <>
                          <Check className="w-3.5 h-3.5 text-emerald-500" />
                          <span>Copied</span>
                        </>
                      ) : (
                        <>
                          <Copy className="w-3.5 h-3.5" />
                          <span>Copy</span>
                        </>
                      )}
                    </button>
                  </div>

                  <p className="text-[11px] text-text-secondary">
                    Navigate to <strong>Cursor Settings &gt; Features &gt; MCP</strong> and add a new Server using SSE /
                    HTTP transport:
                  </p>

                  <pre className="p-3 rounded-xl bg-surface-raised border border-border-default font-mono text-[11px] overflow-x-auto text-text-primary leading-relaxed">
                    {cursorConfig}
                  </pre>
                </BentoCard>

                {/* cURL Terminal Test Command */}
                <BentoCard colSpan={12} className="p-4 space-y-3">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center space-x-2">
                      <Terminal className="w-4 h-4 text-border-accent" />
                      <h4 className="text-xs font-bold text-text-primary">cURL Terminal Test Command</h4>
                    </div>
                    <button
                      type="button"
                      onClick={() => handleCopy(curlSnippet, 'curl')}
                      className="text-xs font-semibold text-border-accent hover:underline flex items-center gap-1"
                    >
                      {configCopied === 'curl' ? (
                        <>
                          <Check className="w-3.5 h-3.5 text-emerald-500" />
                          <span>Copied</span>
                        </>
                      ) : (
                        <>
                          <Copy className="w-3.5 h-3.5" />
                          <span>Copy Command</span>
                        </>
                      )}
                    </button>
                  </div>

                  <p className="text-[11px] text-text-secondary">
                    Execute a direct JSON-RPC Streamable HTTP request from your terminal:
                  </p>

                  <pre className="p-3 rounded-xl bg-surface-raised border border-border-default font-mono text-[11px] overflow-x-auto text-text-primary leading-relaxed">
                    {curlSnippet}
                  </pre>
                </BentoCard>
              </BentoGrid>
            </div>
          )}
        </div>
      )}

      {/* Auth Modal Trigger */}
      <AuthModal
        isOpen={isAuthModalOpen}
        onClose={() => setIsAuthModalOpen(false)}
        initialMode="login"
      />
    </div>
  );
};
