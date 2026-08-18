package mcp

import (
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/thatengineerguy21/CloudVitta/internal/service"
)

// NewServer initializes the MCP server and registers all 5 CloudVitta comparison and calculation tools.
func NewServer(pricingSvc *service.PricingService, freshnessSvc *service.FreshnessService) *sdk.Server {
	impl := &sdk.Implementation{
		Name:        "cloudvitta",
		Version:     "1.0.0",
		Title:       "CloudVitta Cloud Pricing Engine",
		Description: "Multi-cloud normalized pricing comparison and workload cost calculation engine.",
	}
	server := sdk.NewServer(impl, nil)

	RegisterTools(server, pricingSvc, freshnessSvc)

	return server
}
