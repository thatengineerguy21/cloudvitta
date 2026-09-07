package transfertypemap

import "strings"

// Canonical network service groups across cloud providers.
const (
	NetworkGroupVirtualNetworking = "virtual_networking"
	NetworkGroupDNS               = "dns"
	NetworkGroupLoadBalancing     = "load_balancing"
	NetworkGroupCDNEdge           = "cdn_edge"
	NetworkGroupVPN               = "vpn"
	NetworkGroupDedicatedPrivate  = "dedicated_private"
	NetworkGroupNetworkSecurity   = "network_security"
	NetworkGroupNetworkMonitoring = "network_monitoring"
	NetworkGroupNetworkManagement = "network_management"
	NetworkGroupServiceDiscovery  = "service_discovery"
	NetworkGroupAcceleration      = "acceleration"
	NetworkGroupHybrid            = "hybrid"
	NetworkGroupAPIGateway        = "api_gateway"
	NetworkGroupTrafficRouting    = "traffic_routing"
)

// KnownNetworkGroups returns all 14 canonical network service groups.
func KnownNetworkGroups() []string {
	return []string{
		NetworkGroupVirtualNetworking,
		NetworkGroupDNS,
		NetworkGroupLoadBalancing,
		NetworkGroupCDNEdge,
		NetworkGroupVPN,
		NetworkGroupDedicatedPrivate,
		NetworkGroupNetworkSecurity,
		NetworkGroupNetworkMonitoring,
		NetworkGroupNetworkManagement,
		NetworkGroupServiceDiscovery,
		NetworkGroupAcceleration,
		NetworkGroupHybrid,
		NetworkGroupAPIGateway,
		NetworkGroupTrafficRouting,
	}
}

// MapNetworkGroup resolves a raw product or service description to a canonical network group.
func MapNetworkGroup(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	switch {
	case strings.Contains(s, "traffic flow") || strings.Contains(s, "traffic manager") || strings.Contains(s, "service mesh"):
		return NetworkGroupTrafficRouting
	case strings.Contains(s, "dns") || strings.Contains(s, "route 53") || strings.Contains(s, "route53"):
		return NetworkGroupDNS
	case strings.Contains(s, "load balancer") || strings.Contains(s, "load balancing") || strings.Contains(s, "alb") || strings.Contains(s, "nlb") || strings.Contains(s, "application gateway") || strings.Contains(s, "app gateway"):
		return NetworkGroupLoadBalancing
	case strings.Contains(s, "cdn") || strings.Contains(s, "cloudfront") || strings.Contains(s, "front door") || strings.Contains(s, "edge"):
		return NetworkGroupCDNEdge
	case strings.Contains(s, "vpn"):
		return NetworkGroupVPN
	case strings.Contains(s, "direct connect") || strings.Contains(s, "expressroute") || strings.Contains(s, "interconnect") || strings.Contains(s, "privatelink") || strings.Contains(s, "private service connect"):
		return NetworkGroupDedicatedPrivate
	case strings.Contains(s, "firewall") || strings.Contains(s, "waf") || strings.Contains(s, "shield") || strings.Contains(s, "armor") || strings.Contains(s, "security") || strings.Contains(s, "ddos"):
		return NetworkGroupNetworkSecurity
	case strings.Contains(s, "flow logs") || strings.Contains(s, "traffic mirror") || strings.Contains(s, "network watcher") || strings.Contains(s, "intelligence center") || strings.Contains(s, "monitor"):
		return NetworkGroupNetworkMonitoring
	case strings.Contains(s, "network manager") || strings.Contains(s, "cloud wan") || strings.Contains(s, "virtual wan") || strings.Contains(s, "connectivity center"):
		return NetworkGroupNetworkManagement
	case strings.Contains(s, "cloud map") || strings.Contains(s, "service directory") || strings.Contains(s, "discovery"):
		return NetworkGroupServiceDiscovery
	case strings.Contains(s, "global accelerator") || strings.Contains(s, "acceleration") || strings.Contains(s, "service tiers"):
		return NetworkGroupAcceleration
	case strings.Contains(s, "transit gateway") || strings.Contains(s, "router"):
		return NetworkGroupHybrid
	case strings.Contains(s, "api gateway") || strings.Contains(s, "apigee") || strings.Contains(s, "api management"):
		return NetworkGroupAPIGateway
	default:
		return NetworkGroupVirtualNetworking
	}
}
