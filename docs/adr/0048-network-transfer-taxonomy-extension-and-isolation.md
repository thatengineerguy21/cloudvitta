# ADR 0048: Network Transfer Taxonomy Extension and Dedicated Link Scoring Isolation

## Status
Accepted

## Context
CloudVitta models network transfer costs across hyperscaler cloud providers (AWS, Azure, and GCP). Previously, the transfer taxonomy supported three canonical transfer types:
- `internet_egress`
- `inter_region`
- `intra_region`

However, providers also supply dedicated private connection links and encrypted IPsec VPN links:
- AWS: Direct Connect Data Transfer Out and AWS VPN Gateway egress.
- GCP: Cloud Interconnect Egress and Cloud VPN Egress.
- Azure: ExpressRoute Outbound Data Transfer and Azure VPN Gateway egress.

Previously, `internal/matching/transfertypemap` labeled these dedicated private and VPN SKUs as `transfer_type = "internet_egress"`, even though `groups.go` grouped them into `dedicated_private` and `vpn`. This caused two critical problems:
1. Pricing queries for public internet egress could accidentally match against dedicated private or VPN lines, returning misleading rates with different performance and fee structures.
2. Users querying dedicated link pricing had no canonical identifier to specify `direct_connect_egress` or `vpn_egress`.

In addition, Rule 5 of `08-CONSISTENCY-RULES.md` states:
> Cache key schema version must bump when the cached attribute vector changes. Old keys expire naturally via TTL and are not migrated in place.

## Decision
1. **Extend Canonical Taxonomy:**
   We extended the canonical `transfer_type` vocabulary with two new constants:
   - `TransferTypeDirectConnectEgress = "direct_connect_egress"` (AWS Direct Connect, GCP Cloud Interconnect, Azure ExpressRoute)
   - `TransferTypeVPNEgress = "vpn_egress"` (AWS VPN Gateway, GCP Cloud VPN, Azure VPN Gateway)
   We added validation helpers `SupportedTransferTypes()` and `IsValidTransferType(t)` to `internal/matching/transfertypemap`.

2. **Map Provider SKUs Accurately:**
   - Updated `internal/matching/transfertypemap/gcp.go` to map `"cloud interconnect"` and `"interconnect egress"` to `direct_connect_egress`, and `"vpn"` and `"cloud vpn"` to `vpn_egress`.
   - Updated `internal/matching/transfertypemap/aws.go` to map `"direct connect"` and `"directconnect"` to `direct_connect_egress`, and `"vpn"` to `vpn_egress`.
   - Updated `internal/matching/transfertypemap/azure.go` to map `"expressroute"` to `direct_connect_egress`, and `"vpn"` and `"vpn gateway"` to `vpn_egress`.

3. **Cache Schema Version Bump:**
   We introduced `const SchemaVersionNetwork = "v2"` and the helper `CategorySchemaVersion(category string) string` in `internal/cache/keys.go`. All caching for the network category now uses version `"v2"`. Other categories continue to use `"v1"`.

4. **Service Scoring Isolation Guard:**
   In `internal/service/match.go` (`NetworkScorer`), we added a strict scoring guard. If either the user request or the candidate observation specifies `direct_connect_egress` or `vpn_egress`, they must match exactly:
   ```go
   candDedicated := candNet.TransferType == "direct_connect_egress" || candNet.TransferType == "vpn_egress"
   targetDedicated := targetNet.TransferType == "direct_connect_egress" || targetNet.TransferType == "vpn_egress"
   if (candDedicated || targetDedicated) && candNet.TransferType != targetNet.TransferType {
       return 0, nil, false
   }
   ```
   This prevents any approximate or fuzzy cross-matching between public internet egress and dedicated/VPN connections.

5. **REST API Acceptance:**
   The `/api/v1/prices/network` HTTP endpoint now accepts `transfer_type=direct_connect_egress` and `transfer_type=vpn_egress`.

## Consequences
### Positive
- Accurate SKU taxonomy: Dedicated lines and VPNs are no longer mislabeled as general internet egress.
- Strict isolation: Public internet egress queries cannot match against private dedicated interconnect or VPN rates.
- Transparent cache versioning: Bumping the network schema to `v2` isolates the new attribute space from stale cache entries without downtime.

### Negative
- Clients desiring dedicated link pricing must specify `transfer_type=direct_connect_egress` or `transfer_type=vpn_egress` explicitly.
