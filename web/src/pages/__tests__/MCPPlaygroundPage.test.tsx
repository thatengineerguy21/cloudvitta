// web/src/pages/__tests__/MCPPlaygroundPage.test.tsx
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MCPPlaygroundPage } from '../MCPPlaygroundPage';
import { AuthContextType, useAuth } from '../../auth/AuthContext';
import { mcpApi } from '../../api/client';

// Mock useAuth
vi.mock('../../auth/AuthContext', async () => {
  const actual = await vi.importActual('../../auth/AuthContext');
  return {
    ...actual,
    useAuth: vi.fn(),
  };
});

describe('MCPPlaygroundPage', () => {
  const mockAuthUnauthenticated: AuthContextType = {
    user: null,
    tokens: null,
    isAuthenticated: false,
    isLoading: false,
    login: vi.fn(),
    signup: vi.fn(),
    logout: vi.fn(),
  };

  const mockAuthAuthenticated: AuthContextType = {
    user: { email: 'dev@cloudvitta.dev' },
    tokens: {
      accessToken: 'jwt_mock_token_1234567890',
      refreshToken: 'refresh_mock_token_9876543210',
      tokenType: 'Bearer',
      expiresIn: 900,
    },
    isAuthenticated: true,
    isLoading: false,
    login: vi.fn(),
    signup: vi.fn(),
    logout: vi.fn(),
  };

  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('renders unauthenticated gate when user is not signed in', () => {
    vi.mocked(useAuth).mockReturnValue(mockAuthUnauthenticated);

    render(<MCPPlaygroundPage />);

    expect(screen.getByTestId('unauthenticated-mcp-gate')).toBeInTheDocument();
    expect(
      screen.getByText('Authentication Required for Model Context Protocol')
    ).toBeInTheDocument();
    expect(screen.getByText('Sign In to Launch Console')).toBeInTheDocument();
    expect(screen.getByText('Available MCP Agent Tools (10 Tools)')).toBeInTheDocument();
  });

  it('renders authenticated workspace with user email and active session', () => {
    vi.mocked(useAuth).mockReturnValue(mockAuthAuthenticated);

    render(<MCPPlaygroundPage />);

    expect(screen.getByTestId('authenticated-mcp-workspace')).toBeInTheDocument();
    expect(screen.getByText('dev@cloudvitta.dev')).toBeInTheDocument();
    expect(screen.getByText('Active')).toBeInTheDocument();
    expect(screen.getByText('Interactive Tool Runner')).toBeInTheDocument();
    expect(screen.getByText('Agent Setup & Configs')).toBeInTheDocument();
  });

  it('updates JSON arguments when another tool is selected from dropdown', async () => {
    vi.mocked(useAuth).mockReturnValue(mockAuthAuthenticated);
    const user = userEvent.setup();

    render(<MCPPlaygroundPage />);

    const select = screen.getByRole('combobox');
    expect(select).toHaveValue('compare_compute');

    // Change to compare_storage
    await user.selectOptions(select, 'compare_storage');

    expect(select).toHaveValue('compare_storage');
    const textarea = screen.getByPlaceholderText('{ "region": "us-east-1" }') as HTMLTextAreaElement;
    expect(textarea.value).toContain('storage_class');
  });

  it('displays syntax error alert when invalid JSON arguments are entered', () => {
    vi.mocked(useAuth).mockReturnValue(mockAuthAuthenticated);

    render(<MCPPlaygroundPage />);

    const textarea = screen.getByPlaceholderText('{ "region": "us-east-1" }');
    fireEvent.change(textarea, { target: { value: '{ invalid_json ' } });

    expect(screen.getByText(/Syntax Error:/)).toBeInTheDocument();
    const executeBtn = screen.getByRole('button', { name: /Execute Tool Call/i });
    expect(executeBtn).toBeDisabled();
  });

  it('executes tool call and displays formatted provider comparison results', async () => {
    vi.mocked(useAuth).mockReturnValue(mockAuthAuthenticated);
    const user = userEvent.setup();

    const mockToolResult = {
      results: [
        {
          provider: 'aws',
          sku_id: 'c6i.xlarge',
          match_quality: 'exact',
          normalized_hourly_usd: 0.17,
        },
        {
          provider: 'azure',
          sku_id: 'Standard_D4s_v5',
          match_quality: 'exact',
          normalized_hourly_usd: 0.192,
        },
      ],
      warnings: [],
    };

    vi.spyOn(mcpApi, 'callTool').mockResolvedValue({
      jsonrpc: '2.0',
      id: 1,
      result: {
        content: [{ type: 'text', text: JSON.stringify(mockToolResult) }],
        isError: false,
      },
    });

    render(<MCPPlaygroundPage />);

    const executeBtn = screen.getByRole('button', { name: /Execute Tool Call/i });
    await user.click(executeBtn);

    await waitFor(() => {
      expect(screen.getByText('HTTP 200')).toBeInTheDocument();
      expect(screen.getByText('aws')).toBeInTheDocument();
      expect(screen.getByText('c6i.xlarge')).toBeInTheDocument();
      expect(screen.getByText('$0.1700')).toBeInTheDocument();
      expect(screen.getByText('azure')).toBeInTheDocument();
      expect(screen.getByText('$0.1920')).toBeInTheDocument();
    });
  });

  it('switches between formatted view and raw JSON view', async () => {
    vi.mocked(useAuth).mockReturnValue(mockAuthAuthenticated);
    const user = userEvent.setup();

    const mockToolResult = { status: 'healthy', provider: 'aws' };
    vi.spyOn(mcpApi, 'callTool').mockResolvedValue({
      jsonrpc: '2.0',
      id: 1,
      result: {
        content: [{ type: 'text', text: JSON.stringify(mockToolResult) }],
      },
    });

    render(<MCPPlaygroundPage />);

    await user.click(screen.getByRole('button', { name: /Execute Tool Call/i }));

    await waitFor(() => {
      expect(screen.getByText('HTTP 200')).toBeInTheDocument();
    });

    // Toggle to raw
    await user.click(screen.getByRole('button', { name: 'Raw JSON' }));
    expect(screen.getByText(/"jsonrpc": "2.0"/)).toBeInTheDocument();

    // Toggle back to formatted
    await user.click(screen.getByRole('button', { name: 'Formatted' }));
    expect(screen.queryByText(/"jsonrpc": "2.0"/)).not.toBeInTheDocument();
  });

  it('switches to Agent Setup & Configs tab and renders Claude Desktop and Cursor snippets', async () => {
    vi.mocked(useAuth).mockReturnValue(mockAuthAuthenticated);
    const user = userEvent.setup();

    render(<MCPPlaygroundPage />);

    await user.click(screen.getByRole('button', { name: /Agent Setup & Configs/i }));

    expect(screen.getByText('Active Bearer Access Token')).toBeInTheDocument();
    expect(screen.getByText('Claude Desktop Configuration')).toBeInTheDocument();
    expect(screen.getByText('Cursor Settings Configuration')).toBeInTheDocument();
    expect(screen.getByText('cURL Terminal Test Command')).toBeInTheDocument();
    expect(screen.getByText(/claude_desktop_config.json/)).toBeInTheDocument();
  });

  it('executes tools/list on "Ping tools/list" click', async () => {
    vi.mocked(useAuth).mockReturnValue(mockAuthAuthenticated);
    const user = userEvent.setup();

    vi.spyOn(mcpApi, 'listTools').mockResolvedValue({
      jsonrpc: '2.0',
      id: 'tools-list-test',
      result: {
        tools: [
          { name: 'compare_compute', description: 'Compute tool' },
          { name: 'compare_storage', description: 'Storage tool' },
        ],
      },
    });

    render(<MCPPlaygroundPage />);

    await user.click(screen.getByRole('button', { name: /Ping tools\/list/i }));

    await waitFor(() => {
      expect(screen.getByText(/Live Server Tool Registry/)).toBeInTheDocument();
    });
  });

  it('displays execution error banner when tool invocation fails', async () => {
    vi.mocked(useAuth).mockReturnValue(mockAuthAuthenticated);
    const user = userEvent.setup();

    vi.spyOn(mcpApi, 'callTool').mockRejectedValue(new Error('Network connection timeout'));

    render(<MCPPlaygroundPage />);

    await user.click(screen.getByRole('button', { name: /Execute Tool Call/i }));

    await waitFor(() => {
      expect(screen.getByText('Tool Execution Error')).toBeInTheDocument();
      expect(screen.getByText('Network connection timeout')).toBeInTheDocument();
      expect(screen.getByText('HTTP 500')).toBeInTheDocument();
    });
  });
});
