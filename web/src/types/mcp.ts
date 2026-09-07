// web/src/types/mcp.ts

export interface McpJsonRpcRequest {
  jsonrpc: '2.0';
  id: string | number;
  method: string;
  params?: Record<string, unknown>;
}

export interface McpJsonRpcError {
  code: number;
  message: string;
  data?: unknown;
}

export interface McpTextContent {
  type: 'text';
  text: string;
}

export interface McpCallToolResult {
  content?: Array<McpTextContent | { type: string; [key: string]: unknown }>;
  isError?: boolean;
}

export interface McpJsonRpcResponse<T = unknown> {
  jsonrpc: '2.0';
  id: string | number;
  result?: T;
  error?: McpJsonRpcError;
}

export interface McpToolDefinition {
  name: string;
  description?: string;
  inputSchema?: {
    type: string;
    properties?: Record<string, unknown>;
    required?: string[];
  };
}

export interface McpToolsListResult {
  tools: McpToolDefinition[];
}
