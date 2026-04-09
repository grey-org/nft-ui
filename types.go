package main

// QuotaRule represents a parsed nftables quota rule
type QuotaRule struct {
	ID           string  `json:"id"`            // inet_filter_forward_<handle>_<port>
	Handle       int64   `json:"handle"`        // nft handle for forward chain rule
	Port         int     `json:"port"`          // original destination port (ct original proto-dst)
	QuotaBytes   int64   `json:"quota_bytes"`   // quota limit in bytes
	UsedBytes    int64   `json:"used_bytes"`    // current usage in bytes
	UsagePercent float64 `json:"usage_percent"` // calculated: used/quota * 100
	Status       string  `json:"status"`        // "ok" | "warning" | "exceeded"
	Comment      string  `json:"comment"`       // rule comment
}

// AllowedPort represents an allowed inbound port from the input chain
type AllowedPort struct {
	Port     int    `json:"port"`
	Protocol string `json:"protocol"` // "tcp", "udp", or "both"
	Handle   int64  `json:"handle"`
	Managed  bool   `json:"managed"` // true if comment == "nft-ui managed"
	Comment  string `json:"comment,omitempty"`
}

// AddPortRequest is the request body for adding a new allowed port
type AddPortRequest struct {
	Port     int    `json:"port"`
	Protocol string `json:"protocol"` // "tcp", "udp", or "both"; defaults to "tcp"
}

// QuotasResponse is the API response for listing quotas
type QuotasResponse struct {
	Quotas          []QuotaRule   `json:"quotas"`
	AllowedPorts    []AllowedPort `json:"allowed_ports"`
	ReadOnly        bool          `json:"read_only"`
	RefreshInterval int           `json:"refresh_interval"`
}

// AddQuotaRequest is the request body for adding a new quota
type AddQuotaRequest struct {
	Port    int    `json:"port"`
	Bytes   int64  `json:"bytes"`
	Comment string `json:"comment"`
}

// ModifyQuotaRequest is the request body for modifying a quota
type ModifyQuotaRequest struct {
	Bytes int64 `json:"bytes"`
}

// BatchResetRequest is the request body for batch resetting quotas
type BatchResetRequest struct {
	IDs []string `json:"ids"`
}

// APIResponse is a generic API response
type APIResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// QuotaWithToken extends QuotaRule with a query token for admin panel
type QuotaWithToken struct {
	QuotaRule
	Token string `json:"token,omitempty"`
}

// QuotasResponseWithTokens extends QuotasResponse with tokens for admin panel
type QuotasResponseWithTokens struct {
	Quotas          []QuotaWithToken `json:"quotas"`
	AllowedPorts    []AllowedPort    `json:"allowed_ports"`
	ReadOnly        bool             `json:"read_only"`
	RefreshInterval int              `json:"refresh_interval"`
}

// PublicQueryResponse is the API response for public token-based queries
type PublicQueryResponse struct {
	Port         int     `json:"port"`
	UsedBytes    int64   `json:"used_bytes"`
	QuotaBytes   int64   `json:"quota_bytes"`
	UsagePercent float64 `json:"usage_percent"`
	Status       string  `json:"status"`
	Comment      string  `json:"comment,omitempty"`
}

// NFT JSON structures for parsing nft -j output

// NFTRuleset is the top-level structure from nft -j list chain
type NFTRuleset struct {
	NFTables []NFTObject `json:"nftables"`
}

// NFTObject wraps different nftables objects
type NFTObject struct {
	Metainfo *NFTMetainfo `json:"metainfo,omitempty"`
	Chain    *NFTChain    `json:"chain,omitempty"`
	Rule     *NFTRule     `json:"rule,omitempty"`
}

// NFTMetainfo contains nftables version info
type NFTMetainfo struct {
	Version string `json:"version"`
}

// NFTChain represents an nftables chain
type NFTChain struct {
	Family string `json:"family"`
	Table  string `json:"table"`
	Name   string `json:"name"`
	Handle int64  `json:"handle"`
	Type   string `json:"type"`
	Hook   string `json:"hook"`
	Prio   int    `json:"prio"`
	Policy string `json:"policy"`
}

// NFTRule represents an nftables rule
type NFTRule struct {
	Family  string                   `json:"family"`
	Table   string                   `json:"table"`
	Chain   string                   `json:"chain"`
	Handle  int64                    `json:"handle"`
	Expr    []map[string]interface{} `json:"expr"`
	Comment string                   `json:"comment,omitempty"`
}

// ForwardingRule represents a port forwarding rule (DNAT + source NAT)
type ForwardingRule struct {
	ID            string `json:"id"`              // "fwd_<srcPort>"
	SrcPort       int    `json:"src_port"`        // Local port to forward from
	DstIP         string `json:"dst_ip"`          // Destination IP address
	DstPort       int    `json:"dst_port"`        // Destination port
	Protocol      string `json:"protocol"`        // "tcp" | "udp" | "both"
	AddrFamily    string `json:"addr_family"`     // "ip" (IPv4) | "ip6" (IPv6); empty = "ip"
	Enabled       bool   `json:"enabled"`         // Whether the rule is active in nftables
	Managed       bool   `json:"managed"`         // Whether the rule is managed by nft-ui (has comment)
	Comment       string `json:"comment"`         // User-provided description
	PreHandle     int64  `json:"pre_handle"`      // nft handle for prerouting DNAT rule
	PostHandle    int64  `json:"post_handle"`     // nft handle for postrouting source NAT rule
	LimitMbps     int    `json:"limit_mbps"`      // Bandwidth limit in Mbps (0 = no limit)
	MSSMode       string `json:"mss_mode"`        // "pmtu" | "fixed1452" | "disabled"
	SourceNATMode string `json:"source_nat_mode"` // "masquerade" | "snat"
	SNATAddress   string `json:"snat_address"`    // fixed SNAT address when source_nat_mode == "snat"`
}

const (
	MSSModePMTU      = "pmtu"
	MSSModeFixed1452 = "fixed1452"
	MSSModeDisabled  = "disabled"

	SourceNATModeMasquerade = "masquerade"
	SourceNATModeSNAT       = "snat"
)

func normalizeMSSMode(mode string) string {
	switch mode {
	case "", MSSModePMTU:
		return MSSModePMTU
	case MSSModeFixed1452:
		return MSSModeFixed1452
	case MSSModeDisabled:
		return MSSModeDisabled
	default:
		return ""
	}
}

func normalizeAddrFamily(family string) string {
	switch family {
	case "ip6":
		return "ip6"
	default:
		return "ip"
	}
}

func normalizeSourceNATMode(mode string) string {
	switch mode {
	case "", SourceNATModeMasquerade:
		return SourceNATModeMasquerade
	case SourceNATModeSNAT:
		return SourceNATModeSNAT
	default:
		return ""
	}
}

// AddForwardingRequest is the request body for adding a new forwarding rule
type AddForwardingRequest struct {
	SrcPort       int    `json:"src_port"`
	DstIP         string `json:"dst_ip"`
	DstPort       int    `json:"dst_port"`
	Protocol      string `json:"protocol"`
	AddrFamily    string `json:"addr_family"`
	Comment       string `json:"comment"`
	LimitMbps     int    `json:"limit_mbps"`
	MSSMode       string `json:"mss_mode"`
	SourceNATMode string `json:"source_nat_mode"`
	SNATAddress   string `json:"snat_address"`
}

// EditForwardingRequest is the request body for editing a forwarding rule
type EditForwardingRequest struct {
	DstIP         string `json:"dst_ip"`
	DstPort       int    `json:"dst_port"`
	Protocol      string `json:"protocol"`
	AddrFamily    string `json:"addr_family"`
	Comment       string `json:"comment"`
	LimitMbps     int    `json:"limit_mbps"`
	MSSMode       string `json:"mss_mode"`
	SourceNATMode string `json:"source_nat_mode"`
	SNATAddress   string `json:"snat_address"`
}

// ForwardingResponse is the API response for listing forwarding rules
type ForwardingResponse struct {
	Rules    []ForwardingRule `json:"rules"`
	ReadOnly bool             `json:"read_only"`
}

// ForwardingProbeResult describes the outcome of a connectivity check.
type ForwardingProbeResult struct {
	Protocol   string `json:"protocol"`
	Status     string `json:"status"`
	Reachable  bool   `json:"reachable"`
	Message    string `json:"message"`
	DurationMs int64  `json:"duration_ms"`
}

// ForwardingProbeResponse is the API response for forwarding connectivity tests.
type ForwardingProbeResponse struct {
	Success           bool                    `json:"success"`
	DstIP             string                  `json:"dst_ip"`
	DstPort           int                     `json:"dst_port"`
	RequestedProtocol string                  `json:"requested_protocol"`
	OverallStatus     string                  `json:"overall_status"`
	TestedAt          string                  `json:"tested_at"`
	Results           []ForwardingProbeResult `json:"results"`
}

// DisabledForwardsFile represents the JSON structure for storing disabled forwarding rules
type DisabledForwardsFile struct {
	Rules []ForwardingRule `json:"rules"`
}

// BackupPort represents a single port entry in a backup
type BackupPort struct {
	Port     int    `json:"port"`
	Protocol string `json:"protocol"` // "tcp", "udp", or "both"
}

// BackupData represents the exported configuration backup
type BackupData struct {
	Version    int                `json:"version"`
	CreatedAt  string             `json:"created_at"`
	Quotas     []BackupQuota      `json:"quotas"`
	Forwarding []BackupForwarding `json:"forwarding"`
	Ports      []BackupPort       `json:"ports"`
}

// BackupQuota represents a quota rule in the backup
type BackupQuota struct {
	Port       int    `json:"port"`
	QuotaBytes int64  `json:"quota_bytes"`
	Comment    string `json:"comment"`
}

// BackupForwarding represents a forwarding rule in the backup
type BackupForwarding struct {
	SrcPort       int    `json:"src_port"`
	DstIP         string `json:"dst_ip"`
	DstPort       int    `json:"dst_port"`
	Protocol      string `json:"protocol"`
	AddrFamily    string `json:"addr_family,omitempty"`
	Comment       string `json:"comment"`
	LimitMbps     int    `json:"limit_mbps"`
	MSSMode       string `json:"mss_mode,omitempty"`
	SourceNATMode string `json:"source_nat_mode,omitempty"`
	SNATAddress   string `json:"snat_address,omitempty"`
	Enabled       bool   `json:"enabled"`
}

// ImportSummary represents the result of an import operation
type ImportSummary struct {
	QuotasAdded       int `json:"quotas_added"`
	QuotasSkipped     int `json:"quotas_skipped"`
	ForwardingAdded   int `json:"forwarding_added"`
	ForwardingSkipped int `json:"forwarding_skipped"`
	PortsAdded        int `json:"ports_added"`
	PortsSkipped      int `json:"ports_skipped"`
}

// ConntrackStatus holds current conntrack table utilisation
type ConntrackStatus struct {
	Current      int64   `json:"current"`       // nf_conntrack_count
	Max          int64   `json:"max"`           // nf_conntrack_max
	UsagePercent float64 `json:"usage_percent"` // current/max * 100
	Warning      bool    `json:"warning"`       // true when usage_percent >= 80
}

// ConntrackResponse is the API response for GET /api/v1/system/conntrack
type ConntrackResponse struct {
	Success bool            `json:"success"`
	Data    ConntrackStatus `json:"data"`
}

// IfaceForwardRule represents an interface-based forwarding rule (dual-NIC DNAT)
type IfaceForwardRule struct {
	ID         string `json:"id"`          // "ifwd_<8hex>"
	IifName    string `json:"iif_name"`    // e.g. "eth0"
	AddrFamily    string `json:"addr_family"`     // "ip" or "ip6" — inbound match
	NatAddrFamily string `json:"nat_addr_family"` // "ip" or "ip6" — DNAT target family, empty = same as AddrFamily
	DstAddr       string `json:"dst_addr"`        // destination address to match
	NatTo         string `json:"nat_to"`          // DNAT target address
	Protocol   string `json:"protocol"`   // "udp", "tcp", or "all"
	Comment    string `json:"comment"`    // user comment
	Enabled    bool   `json:"enabled"`
	Managed    bool   `json:"managed"`
	Handle     int64  `json:"handle"`
}

// AddIfaceForwardRequest is the request body for adding an interface forwarding rule
type AddIfaceForwardRequest struct {
	IifName       string `json:"iif_name"`
	AddrFamily    string `json:"addr_family"`
	NatAddrFamily string `json:"nat_addr_family"`
	DstAddr       string `json:"dst_addr"`
	NatTo         string `json:"nat_to"`
	Protocol      string `json:"protocol"`
	Comment       string `json:"comment"`
}

// EditIfaceForwardRequest is the request body for editing an interface forwarding rule
type EditIfaceForwardRequest struct {
	IifName       string `json:"iif_name"`
	AddrFamily    string `json:"addr_family"`
	NatAddrFamily string `json:"nat_addr_family"`
	DstAddr       string `json:"dst_addr"`
	NatTo         string `json:"nat_to"`
	Protocol      string `json:"protocol"`
	Comment       string `json:"comment"`
}

// IfaceForwardingResponse is the API response for listing interface forwarding rules
type IfaceForwardingResponse struct {
	Rules    []IfaceForwardRule `json:"rules"`
	ReadOnly bool               `json:"read_only"`
}

// DisabledIfaceForwardsFile represents the JSON structure for storing disabled iface forwarding rules
type DisabledIfaceForwardsFile struct {
	Rules []IfaceForwardRule `json:"rules"`
}

const (
	IfaceForwardComment   = "nft-ui iface-fwd"
	IfaceForwardTableName = "nft-ui-iface-fwd"
	IfaceForwardChainName = "prerouting"
)

// BypassConfig holds the persisted forward-bypass configuration
type BypassConfig struct {
	Enabled  bool   `json:"enabled"`
	Mark     uint32 `json:"mark"`
	Priority int    `json:"priority"`
}

// BypassStatusResponse is the API response for GET /api/v1/bypass
type BypassStatusResponse struct {
	Success  bool         `json:"success"`
	Config   BypassConfig `json:"config"`
	Applied  bool         `json:"applied"`  // ip rule is present in kernel
	IPRule   string       `json:"ip_rule"`  // the ip rule string, for display
}

// SetBypassRequest is the request body for POST /api/v1/bypass
type SetBypassRequest struct {
	Enabled  bool   `json:"enabled"`
	Mark     uint32 `json:"mark"`
	Priority int    `json:"priority"`
}
