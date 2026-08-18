package mcp

import (
	"net/http"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// NewStreamableHandler creates a net/http.Handler wrapping the MCP server over Streamable HTTP transport.
func NewStreamableHandler(server *sdk.Server) http.Handler {
	return sdk.NewStreamableHTTPHandler(
		func(r *http.Request) *sdk.Server {
			return server
		},
		&sdk.StreamableHTTPOptions{
			Stateless: true,
		},
	)
}
