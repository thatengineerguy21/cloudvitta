package transfertypemap

import "testing"

func TestNetworkGroups(t *testing.T) {
	groups := KnownNetworkGroups()
	if len(groups) != 14 {
		t.Fatalf("expected 14 network groups, got %d", len(groups))
	}

	tests := []struct {
		raw      string
		expected string
	}{
		{"Amazon Route 53 DNS Queries", NetworkGroupDNS},
		{"Azure DNS Zone", NetworkGroupDNS},
		{"Elastic Load Balancing Application Load Balancer", NetworkGroupLoadBalancing},
		{"Azure Application Gateway", NetworkGroupLoadBalancing},
		{"Amazon CloudFront Data Transfer Out", NetworkGroupCDNEdge},
		{"Azure Front Door Requests", NetworkGroupCDNEdge},
		{"AWS Site-to-Site VPN Connection", NetworkGroupVPN},
		{"AWS Direct Connect Port", NetworkGroupDedicatedPrivate},
		{"Azure ExpressRoute Circuit", NetworkGroupDedicatedPrivate},
		{"AWS Network Firewall", NetworkGroupNetworkSecurity},
		{"Azure DDoS Protection", NetworkGroupNetworkSecurity},
		{"VPC Flow Logs", NetworkGroupNetworkMonitoring},
		{"Azure Network Watcher", NetworkGroupNetworkMonitoring},
		{"AWS Network Manager", NetworkGroupNetworkManagement},
		{"AWS Cloud Map Service", NetworkGroupServiceDiscovery},
		{"AWS Global Accelerator", NetworkGroupAcceleration},
		{"AWS Transit Gateway Attachment", NetworkGroupHybrid},
		{"Amazon API Gateway HTTP", NetworkGroupAPIGateway},
		{"Azure API Management Developer", NetworkGroupAPIGateway},
		{"Route 53 Traffic Flow", NetworkGroupTrafficRouting},
		{"Amazon VPC Subnet NAT Gateway", NetworkGroupVirtualNetworking},
	}

	for _, tt := range tests {
		got := MapNetworkGroup(tt.raw)
		if got != tt.expected {
			t.Errorf("MapNetworkGroup(%q) = %q, want %q", tt.raw, got, tt.expected)
		}
	}
}
