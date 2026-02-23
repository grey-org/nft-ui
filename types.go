package main

// QuotaRule represents a parsed nftables quota rule
type QuotaRule struct {
	ID           string  `json:"id"`            // inet_filter_output_<handle>
	Handle       int64   `json:"handle"`        // nft handle for deletion
	Port         int     `json:"port"`          // source port
	QuotaBytes   int64   `json:"quota_bytes"`   // quota limit in bytes
	UsedBytes    int64   `json:"used_bytes"`    // current usage in bytes
	UsagePercent float64 `json:"usage_percent"` // calculated: used/quota * 100
	Status       string  `json:"status"`        // "ok" | "warning" | "exceeded"
	Comment      string  `json:"comment"`       // rule comment
}

// AllowedPort represents an allowed inbound port from the input chain
type AllowedPort struct {
	Port    int    `json:"port"`
	Handle  int64  `json:"handle"`
	Managed bool   `json:"managed"` // true if comment == "nft-ui managed"
	Comment string `json:"comment,omitempty"`
}

// AddPortRequest is the request body for adding a new allowed port
type AddPortRequest struct {
	Port int `json:"port"`
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

// ForwardingRule represents a port forwarding rule (DNAT + MASQUERADE)
type ForwardingRule struct {
	ID         string `json:"id"`          // "fwd_<srcPort>"
	SrcPort    int    `json:"src_port"`    // Local port to forward from
	DstIP      string `json:"dst_ip"`      // Destination IP address
	DstPort    int    `json:"dst_port"`    // Destination port
	Protocol   string `json:"protocol"`    // "tcp" | "udp" | "both"
	Enabled    bool   `json:"enabled"`     // Whether the rule is active in nftables
	Managed    bool   `json:"managed"`     // Whether the rule is managed by nft-ui (has comment)
	Comment    string `json:"comment"`     // User-provided description
	PreHandle  int64  `json:"pre_handle"`  // nft handle for prerouting DNAT rule
	PostHandle int64  `json:"post_handle"` // nft handle for postrouting MASQUERADE rule
	LimitMbps  int    `json:"limit_mbps"`  // Bandwidth limit in Mbps (0 = no limit)
}

// AddForwardingRequest is the request body for adding a new forwarding rule
type AddForwardingRequest struct {
	SrcPort   int    `json:"src_port"`
	DstIP     string `json:"dst_ip"`
	DstPort   int    `json:"dst_port"`
	Protocol  string `json:"protocol"`
	Comment   string `json:"comment"`
	LimitMbps int    `json:"limit_mbps"`
}

// EditForwardingRequest is the request body for editing a forwarding rule
type EditForwardingRequest struct {
	DstIP     string `json:"dst_ip"`
	DstPort   int    `json:"dst_port"`
	Protocol  string `json:"protocol"`
	Comment   string `json:"comment"`
	LimitMbps int    `json:"limit_mbps"`
}

// ForwardingResponse is the API response for listing forwarding rules
type ForwardingResponse struct {
	Rules    []ForwardingRule `json:"rules"`
	ReadOnly bool             `json:"read_only"`
}

// DisabledForwardsFile represents the JSON structure for storing disabled forwarding rules
type DisabledForwardsFile struct {
	Rules []ForwardingRule `json:"rules"`
}

// BackupData represents the exported configuration backup
type BackupData struct {
	Version    int                `json:"version"`
	CreatedAt  string             `json:"created_at"`
	Quotas     []BackupQuota      `json:"quotas"`
	Forwarding []BackupForwarding `json:"forwarding"`
	Ports      []int              `json:"ports"`
}

// BackupQuota represents a quota rule in the backup
type BackupQuota struct {
	Port       int    `json:"port"`
	QuotaBytes int64  `json:"quota_bytes"`
	Comment    string `json:"comment"`
}

// BackupForwarding represents a forwarding rule in the backup
type BackupForwarding struct {
	SrcPort   int    `json:"src_port"`
	DstIP     string `json:"dst_ip"`
	DstPort   int    `json:"dst_port"`
	Protocol  string `json:"protocol"`
	Comment   string `json:"comment"`
	LimitMbps int    `json:"limit_mbps"`
	Enabled   bool   `json:"enabled"`
}

// ImportSummary represents the result of an import operation
type ImportSummary struct {
	QuotasAdded      int `json:"quotas_added"`
	QuotasSkipped    int `json:"quotas_skipped"`
	ForwardingAdded  int `json:"forwarding_added"`
	ForwardingSkipped int `json:"forwarding_skipped"`
	PortsAdded       int `json:"ports_added"`
	PortsSkipped     int `json:"ports_skipped"`
}
