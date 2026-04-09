package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ManagedComment is the comment used to identify rules managed by nft-ui
const ManagedComment = "nft-ui managed"

// QuotaForwardComment is the comment prefix for forward-chain quota rules
const QuotaForwardComment = "nft-ui quota"

// ForwardChainName is the chain name used for forward-chain quota rules
const ForwardChainName = "forward"

// NFTManager handles all nftables operations
type NFTManager struct {
	mu          sync.RWMutex
	binary      string
	tableFamily string
	tableName   string
	chainName   string
	rulesetPath string
}

// NewNFTManager creates a new NFTManager
func NewNFTManager(cfg *Config) *NFTManager {
	return &NFTManager{
		binary:      cfg.NFTBinary,
		tableFamily: cfg.TableFamily,
		tableName:   cfg.TableName,
		chainName:   cfg.ChainName,
		rulesetPath: cfg.RulesetPath,
	}
}

// execNFT executes an nft command and returns the output
func (n *NFTManager) execNFT(args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, n.binary, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("nft %s: %w (output: %s)", strings.Join(args, " "), err, string(output))
	}
	return output, nil
}

// ListQuotas returns all quota rules from the forward chain.
// Quotas are tracked exclusively in the forward chain using ct original proto-dst,
// which correctly counts all forwarded (DNAT) traffic in both directions.
func (n *NFTManager) ListQuotas() ([]QuotaRule, error) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	output, err := n.execNFT("-j", "-a", "list", "chain", n.tableFamily, n.tableName, ForwardChainName)
	if err != nil {
		if strings.Contains(err.Error(), "No such file or directory") ||
			strings.Contains(err.Error(), "does not exist") {
			return []QuotaRule{}, nil
		}
		return nil, err
	}

	rules, err := n.parseQuotaRules(output)
	if err != nil {
		return nil, err
	}

	// Filter to only quota rules managed by nft-ui (QuotaForwardComment prefix)
	var managed []QuotaRule
	for _, r := range rules {
		if strings.HasPrefix(r.Comment, QuotaForwardComment+" ") {
			managed = append(managed, r)
		}
	}
	return managed, nil
}

// parseQuotaRules parses the JSON output and extracts quota rules from any chain
func (n *NFTManager) parseQuotaRules(data []byte) ([]QuotaRule, error) {
	var ruleset NFTRuleset
	if err := json.Unmarshal(data, &ruleset); err != nil {
		return nil, fmt.Errorf("failed to parse nft JSON: %w", err)
	}

	var rules []QuotaRule

	for _, obj := range ruleset.NFTables {
		if obj.Rule == nil {
			continue
		}

		rule := obj.Rule

		// Extract quota rules (may return multiple for port sets)
		quotaRules := n.extractQuotaRules(rule)
		rules = append(rules, quotaRules...)
	}

	return rules, nil
}

// extractQuotaRules extracts quota information from a rule (supports multiple ports)
func (n *NFTManager) extractQuotaRules(rule *NFTRule) []QuotaRule {
	var hasQuota bool
	var quotaBytes, usedBytes int64
	var ports []int

	for _, expr := range rule.Expr {
		// Look for quota expression
		if quotaData, ok := expr["quota"]; ok {
			hasQuota = true
			if qm, ok := quotaData.(map[string]interface{}); ok {
				// Get quota value (limit) with unit conversion
				if val, ok := qm["val"].(float64); ok {
					valUnit, _ := qm["val_unit"].(string)
					quotaBytes = convertToBytes(int64(val), valUnit)
				}
				// Get used value with unit conversion
				if used, ok := qm["used"].(float64); ok {
					usedUnit, _ := qm["used_unit"].(string)
					usedBytes = convertToBytes(int64(used), usedUnit)
				}
			}
		}

		// Look for port match (th sport)
		if matchData, ok := expr["match"]; ok {
			if mm, ok := matchData.(map[string]interface{}); ok {
				extractedPorts := n.extractPorts(mm)
				if len(extractedPorts) > 0 {
					ports = extractedPorts
				}
			}
		}
	}

	if !hasQuota {
		return nil
	}

	// If no ports found, still return a single rule with port 0
	if len(ports) == 0 {
		ports = []int{0}
	}

	// Calculate usage percent
	var usagePercent float64
	if quotaBytes > 0 {
		usagePercent = float64(usedBytes) / float64(quotaBytes) * 100
	}

	// Determine status
	var status string
	if usagePercent >= 100 {
		status = "exceeded"
	} else if usagePercent >= 70 {
		status = "warning"
	} else {
		status = "ok"
	}

	// Create a QuotaRule for each port
	var rules []QuotaRule
	for _, port := range ports {
		qr := QuotaRule{
			Handle:       rule.Handle,
			ID:           fmt.Sprintf("%s_%s_%s_%d_%d", rule.Family, rule.Table, rule.Chain, rule.Handle, port),
			Comment:      rule.Comment,
			Port:         port,
			QuotaBytes:   quotaBytes,
			UsedBytes:    usedBytes,
			UsagePercent: usagePercent,
			Status:       status,
		}
		rules = append(rules, qr)
	}

	return rules
}

// extractPorts extracts port numbers from a match expression (supports single port, port set, and ct original dport)
func (n *NFTManager) extractPorts(match map[string]interface{}) []int {
	left, ok := match["left"].(map[string]interface{})
	if !ok {
		return nil
	}

	// Check for ct match (ct original proto-dst)
	if ct, ok := left["ct"].(map[string]interface{}); ok {
		key, _ := ct["key"].(string)
		dir, _ := ct["dir"].(string)
		if key == "proto-dst" && dir == "original" {
			right := match["right"]
			if right == nil {
				return nil
			}
			if port, ok := right.(float64); ok {
				return []int{int(port)}
			}
			return nil
		}
	}

	// Check for payload match (th sport)
	payload, ok := left["payload"].(map[string]interface{})
	if !ok {
		return nil
	}

	field, _ := payload["field"].(string)
	if field != "sport" && field != "dport" {
		return nil
	}

	right := match["right"]
	if right == nil {
		return nil
	}

	var ports []int

	// Single port
	if port, ok := right.(float64); ok {
		return []int{int(port)}
	}

	// Port set: {"set": [8889, 14001]}
	if rightMap, ok := right.(map[string]interface{}); ok {
		if set, ok := rightMap["set"].([]interface{}); ok {
			for _, p := range set {
				if port, ok := p.(float64); ok {
					ports = append(ports, int(port))
				}
			}
		}
	}

	// Direct array: [8889, 14001]
	if set, ok := right.([]interface{}); ok {
		for _, p := range set {
			if port, ok := p.(float64); ok {
				ports = append(ports, int(port))
			}
		}
	}

	return ports
}

// ResetQuota resets a quota's used bytes to 0 by deleting and re-adding the forward chain rule
func (n *NFTManager) ResetQuota(id string) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	rule, err := n.findRuleByID(id)
	if err != nil {
		return err
	}

	// Strip the "nft-ui quota <port>" prefix to recover the user comment
	userComment := strings.TrimPrefix(rule.Comment, fmt.Sprintf("%s %d", QuotaForwardComment, rule.Port))
	userComment = strings.TrimSpace(userComment)

	n.deleteForwardQuotaByPort(rule.Port)

	if err := n.addQuotaRule(rule.Port, rule.QuotaBytes, userComment); err != nil {
		return fmt.Errorf("failed to recreate rule: %w", err)
	}

	return nil
}

// BatchResetQuotas resets multiple quotas
func (n *NFTManager) BatchResetQuotas(ids []string) error {
	for _, id := range ids {
		if err := n.ResetQuota(id); err != nil {
			return fmt.Errorf("failed to reset %s: %w", id, err)
		}
	}
	return nil
}

// ModifyQuota changes the quota limit
func (n *NFTManager) ModifyQuota(id string, newBytes int64) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	rule, err := n.findRuleByID(id)
	if err != nil {
		return err
	}

	// Strip the "nft-ui quota <port>" prefix to recover the user comment
	userComment := strings.TrimPrefix(rule.Comment, fmt.Sprintf("%s %d", QuotaForwardComment, rule.Port))
	userComment = strings.TrimSpace(userComment)

	n.deleteForwardQuotaByPort(rule.Port)

	if err := n.addQuotaRule(rule.Port, newBytes, userComment); err != nil {
		return fmt.Errorf("failed to recreate rule: %w", err)
	}

	return nil
}

// AddQuota adds a new quota rule
func (n *NFTManager) AddQuota(port int, bytes int64, comment string) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	// Validate inputs
	if port < 1 || port > 65535 {
		return fmt.Errorf("invalid port: %d", port)
	}
	if bytes <= 0 {
		return errors.New("quota limit must be positive")
	}

	// Sanitize comment
	comment = sanitizeComment(comment)

	return n.addQuotaRule(port, bytes, comment)
}

// DeleteQuota deletes a quota rule from the forward chain
func (n *NFTManager) DeleteQuota(id string) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	rule, err := n.findRuleByID(id)
	if err != nil {
		return err
	}

	return n.deleteForwardQuotaByPort(rule.Port)
}

// findRuleByID finds a quota rule by its ID in the forward chain (requires lock to be held)
func (n *NFTManager) findRuleByID(id string) (*QuotaRule, error) {
	output, err := n.execNFT("-j", "-a", "list", "chain", n.tableFamily, n.tableName, ForwardChainName)
	if err != nil {
		return nil, err
	}

	rules, err := n.parseQuotaRules(output)
	if err != nil {
		return nil, err
	}

	for _, rule := range rules {
		if rule.ID == id {
			return &rule, nil
		}
	}

	return nil, fmt.Errorf("rule not found: %s", id)
}

// addQuotaRule adds a quota rule in the forward chain using ct original proto-dst.
// This correctly counts all forwarded (DNAT) traffic in both directions.
func (n *NFTManager) addQuotaRule(port int, bytes int64, comment string) error {
	if err := n.EnsureFilterForwardSetup(); err != nil {
		return err
	}

	// Convert bytes to mbytes for the nft quota keyword
	mbytes := bytes / (1000 * 1000)
	if mbytes < 1 {
		mbytes = 1
	}

	fwdComment := fmt.Sprintf("%s %d", QuotaForwardComment, port)
	if comment != "" {
		fwdComment = fmt.Sprintf("%s %s", fwdComment, comment)
	}
	args := []string{
		"insert", "rule", n.tableFamily, n.tableName, ForwardChainName,
		"ct", "original", "proto-dst", strconv.Itoa(port),
		"quota", "over", strconv.FormatInt(mbytes, 10), "mbytes",
		"drop",
		"comment", fmt.Sprintf(`"%s"`, fwdComment),
	}
	if _, err := n.execNFT(args...); err != nil {
		return err
	}

	return nil
}

// EnsureFilterForwardSetup ensures the filter table and forward chain exist
func (n *NFTManager) EnsureFilterForwardSetup() error {
	if _, err := n.execNFT("list", "table", n.tableFamily, n.tableName); err != nil {
		if _, err := n.execNFT("add", "table", n.tableFamily, n.tableName); err != nil {
			return fmt.Errorf("failed to create %s %s table: %w", n.tableFamily, n.tableName, err)
		}
	}
	_, err := n.execNFT("list", "chain", n.tableFamily, n.tableName, ForwardChainName)
	if err != nil {
		if _, err := n.execNFT("add", "chain", n.tableFamily, n.tableName, ForwardChainName,
			"{ type filter hook forward priority filter ; policy accept ; }"); err != nil {
			return fmt.Errorf("failed to create %s chain: %w", ForwardChainName, err)
		}
	}
	return nil
}

// deleteForwardQuotaByPort deletes forward chain quota rule for a given port
func (n *NFTManager) deleteForwardQuotaByPort(port int) error {
	output, err := n.execNFT("-j", "-a", "list", "chain", n.tableFamily, n.tableName, ForwardChainName)
	if err != nil {
		return nil // chain doesn't exist, nothing to delete
	}

	var ruleset NFTRuleset
	if err := json.Unmarshal(output, &ruleset); err != nil {
		return err
	}

	commentPrefix := fmt.Sprintf("%s %d", QuotaForwardComment, port)
	for _, obj := range ruleset.NFTables {
		if obj.Rule == nil || obj.Rule.Chain != ForwardChainName {
			continue
		}
		if strings.HasPrefix(obj.Rule.Comment, commentPrefix) {
			n.execNFT("delete", "rule", n.tableFamily, n.tableName, ForwardChainName,
				"handle", strconv.FormatInt(obj.Rule.Handle, 10))
			return nil
		}
	}
	return nil
}

// convertToBytes converts a value with unit to bytes
func convertToBytes(val int64, unit string) int64 {
	switch unit {
	case "kbytes":
		return val * 1000
	case "mbytes":
		return val * 1000 * 1000
	case "gbytes":
		return val * 1000 * 1000 * 1000
	case "tbytes":
		return val * 1000 * 1000 * 1000 * 1000
	default:
		// "bytes" or empty string means already in bytes
		return val
	}
}

// sanitizeComment removes characters that could break nft parsing
func sanitizeComment(s string) string {
	// Remove quotes and special characters
	re := regexp.MustCompile(`[^a-zA-Z0-9\s\-_.]`)
	s = re.ReplaceAllString(s, "")
	// Limit length
	if len(s) > 100 {
		s = s[:100]
	}
	return s
}

// ListAllowedPorts returns allowed inbound ports from the input chain
func (n *NFTManager) ListAllowedPorts() ([]AllowedPort, error) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	// Get JSON output from input chain
	output, err := n.execNFT("-j", "-a", "list", "chain", n.tableFamily, n.tableName, "input")
	if err != nil {
		// If chain doesn't exist, return empty list instead of error
		if strings.Contains(err.Error(), "No such file or directory") ||
			strings.Contains(err.Error(), "does not exist") {
			return []AllowedPort{}, nil
		}
		return nil, err
	}

	return n.parseAllowedPorts(output)
}

// parseAllowedPorts extracts allowed ports from nft JSON output
func (n *NFTManager) parseAllowedPorts(data []byte) ([]AllowedPort, error) {
	var ruleset NFTRuleset
	if err := json.Unmarshal(data, &ruleset); err != nil {
		return nil, fmt.Errorf("failed to parse nft JSON: %w", err)
	}

	var ports []AllowedPort
	seen := make(map[string]bool)

	for _, obj := range ruleset.NFTables {
		if obj.Rule == nil {
			continue
		}

		rule := obj.Rule
		if rule.Chain != "input" {
			continue
		}

		// Check if this rule has an accept verdict
		hasAccept := false
		for _, expr := range rule.Expr {
			if _, ok := expr["accept"]; ok {
				hasAccept = true
				break
			}
		}
		if !hasAccept {
			continue
		}

		// Look for tcp/udp dport matches
		for _, expr := range rule.Expr {
			matchData, ok := expr["match"]
			if !ok {
				continue
			}

			mm, ok := matchData.(map[string]interface{})
			if !ok {
				continue
			}

			extractedPorts := n.extractDPorts(mm)
			proto := n.extractDPortProtocol(mm, rule.Expr)
			for _, port := range extractedPorts {
				key := fmt.Sprintf("%d:%s", port, proto)
				if !seen[key] {
					seen[key] = true
					ports = append(ports, AllowedPort{
						Port:     port,
						Protocol: proto,
						Handle:   rule.Handle,
						Managed:  rule.Comment == ManagedComment,
						Comment:  rule.Comment,
					})
				}
			}
		}
	}

	return ports, nil
}

// extractDPorts extracts destination ports from a match expression
func (n *NFTManager) extractDPorts(match map[string]interface{}) []int {
	left, ok := match["left"].(map[string]interface{})
	if !ok {
		return nil
	}

	// Check for payload match (tcp/udp dport)
	payload, ok := left["payload"].(map[string]interface{})
	if !ok {
		return nil
	}

	field, _ := payload["field"].(string)
	if field != "dport" {
		return nil
	}

	// Get the right side (port number or set of ports)
	right := match["right"]
	if right == nil {
		return nil
	}

	var ports []int

	// Single port
	if port, ok := right.(float64); ok {
		ports = append(ports, int(port))
		return ports
	}

	// Set of ports: {"set": [22, 80, 443]}
	if rightMap, ok := right.(map[string]interface{}); ok {
		if set, ok := rightMap["set"].([]interface{}); ok {
			for _, p := range set {
				if port, ok := p.(float64); ok {
					ports = append(ports, int(port))
				}
			}
		}
	}

	// Direct array of ports
	if set, ok := right.([]interface{}); ok {
		for _, p := range set {
			if port, ok := p.(float64); ok {
				ports = append(ports, int(port))
			}
		}
	}

	return ports
}

// extractDPortProtocol determines the protocol ("tcp", "udp", or "both") for a dport match expression.
// It inspects payload.protocol; for "th" (transport header), it checks the full expr list for a
// meta l4proto match to determine if both tcp and udp are covered.
func (n *NFTManager) extractDPortProtocol(match map[string]interface{}, allExprs []map[string]interface{}) string {
	left, ok := match["left"].(map[string]interface{})
	if !ok {
		return "tcp"
	}
	payload, ok := left["payload"].(map[string]interface{})
	if !ok {
		return "tcp"
	}
	proto, _ := payload["protocol"].(string)
	switch proto {
	case "tcp":
		return "tcp"
	case "udp":
		return "udp"
	case "th":
		// Check for meta l4proto { tcp, udp } match in siblings
		hasTCP, hasUDP := false, false
		for _, expr := range allExprs {
			mm, ok := expr["match"].(map[string]interface{})
			if !ok {
				continue
			}
			left2, ok := mm["left"].(map[string]interface{})
			if !ok {
				continue
			}
			meta, ok := left2["meta"].(map[string]interface{})
			if !ok {
				continue
			}
			if meta["key"] != "l4proto" {
				continue
			}
			// right can be a set of protocol numbers (6=tcp, 17=udp) or strings
			right := mm["right"]
			if rightMap, ok := right.(map[string]interface{}); ok {
				if set, ok := rightMap["set"].([]interface{}); ok {
					for _, v := range set {
						switch val := v.(type) {
						case float64:
							if val == 6 {
								hasTCP = true
							} else if val == 17 {
								hasUDP = true
							}
						case string:
							if val == "tcp" {
								hasTCP = true
							} else if val == "udp" {
								hasUDP = true
							}
						}
					}
				}
			}
		}
		if hasTCP && hasUDP {
			return "both"
		} else if hasUDP {
			return "udp"
		}
		return "tcp"
	}
	return "tcp"
}

// EnsureFilterInputSetup ensures the filter table and input chain exist
func (n *NFTManager) EnsureFilterInputSetup() error {
	// Check if table exists
	_, err := n.execNFT("list", "table", n.tableFamily, n.tableName)
	if err != nil {
		// Create table
		if _, err := n.execNFT("add", "table", n.tableFamily, n.tableName); err != nil {
			return fmt.Errorf("failed to create %s %s table: %w", n.tableFamily, n.tableName, err)
		}
	}

	// Check if input chain exists
	_, err = n.execNFT("list", "chain", n.tableFamily, n.tableName, "input")
	if err != nil {
		// Create input chain
		if _, err := n.execNFT("add", "chain", n.tableFamily, n.tableName, "input",
			"{ type filter hook input priority filter ; policy accept ; }"); err != nil {
			return fmt.Errorf("failed to create input chain: %w", err)
		}
	}

	return nil
}

// AddAllowedPort adds a new allowed inbound port rule.
// protocol must be "tcp", "udp", or "both".
func (n *NFTManager) AddAllowedPort(port int, protocol string) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if port < 1 || port > 65535 {
		return fmt.Errorf("invalid port: %d", port)
	}
	if protocol == "" {
		protocol = "tcp"
	}
	if protocol != "tcp" && protocol != "udp" && protocol != "both" {
		return fmt.Errorf("invalid protocol %q: must be tcp, udp, or both", protocol)
	}

	if err := n.EnsureFilterInputSetup(); err != nil {
		return err
	}

	var args []string
	switch protocol {
	case "both":
		// meta l4proto { tcp, udp } th dport <port> accept
		args = []string{
			"insert", "rule", n.tableFamily, n.tableName, "input",
			"meta", "l4proto", "{", "tcp,", "udp", "}",
			"th", "dport", strconv.Itoa(port),
			"accept",
			"comment", fmt.Sprintf(`"%s"`, ManagedComment),
		}
	default:
		// tcp dport <port> accept  OR  udp dport <port> accept
		args = []string{
			"insert", "rule", n.tableFamily, n.tableName, "input",
			protocol, "dport", strconv.Itoa(port),
			"accept",
			"comment", fmt.Sprintf(`"%s"`, ManagedComment),
		}
	}

	_, err := n.execNFT(args...)
	return err
}

// DeleteAllowedPort deletes an allowed inbound port rule by handle
func (n *NFTManager) DeleteAllowedPort(handle int64) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	// First verify the rule exists and has the managed comment
	output, err := n.execNFT("-j", "-a", "list", "chain", n.tableFamily, n.tableName, "input")
	if err != nil {
		return err
	}

	ports, err := n.parseAllowedPorts(output)
	if err != nil {
		return err
	}

	// Find the port with this handle and verify it's managed
	var found bool
	for _, p := range ports {
		if p.Handle == handle {
			if !p.Managed {
				return errors.New("cannot delete: rule is not managed by nft-ui")
			}
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("rule not found: handle %d", handle)
	}

	// Delete the rule
	_, err = n.execNFT("delete", "rule", n.tableFamily, n.tableName, "input", "handle", strconv.FormatInt(handle, 10))
	return err
}

// GetRawRuleset returns the raw output of 'nft list ruleset'
func (n *NFTManager) GetRawRuleset() (string, error) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	output, err := n.execNFT("list", "ruleset")
	if err != nil {
		return "", err
	}

	return string(output), nil
}

// GetConntrackStats reads the current conntrack table utilisation from /proc.
func (n *NFTManager) GetConntrackStats() (ConntrackStatus, error) {
	readInt := func(path string) (int64, error) {
		data, err := os.ReadFile(path)
		if err != nil {
			return 0, err
		}
		v, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parse %s: %w", path, err)
		}
		return v, nil
	}

	current, err := readInt("/proc/sys/net/netfilter/nf_conntrack_count")
	if err != nil {
		return ConntrackStatus{}, err
	}
	max, err := readInt("/proc/sys/net/netfilter/nf_conntrack_max")
	if err != nil {
		return ConntrackStatus{}, err
	}

	var usagePct float64
	if max > 0 {
		usagePct = float64(current) / float64(max) * 100
	}

	return ConntrackStatus{
		Current:      current,
		Max:          max,
		UsagePercent: usagePct,
		Warning:      usagePct >= 80,
	}, nil
}

// SaveRuleset dumps the current nftables ruleset to the configured file path
func (n *NFTManager) SaveRuleset() error {
	output, err := n.execNFT("list", "ruleset")
	if err != nil {
		return fmt.Errorf("failed to list ruleset: %w", err)
	}

	// Create parent directories if needed
	dir := filepath.Dir(n.rulesetPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Write atomically: write to .tmp then rename
	tmpPath := n.rulesetPath + ".tmp"
	if err := os.WriteFile(tmpPath, output, 0644); err != nil {
		return fmt.Errorf("failed to write ruleset file: %w", err)
	}

	if err := os.Rename(tmpPath, n.rulesetPath); err != nil {
		return fmt.Errorf("failed to rename ruleset file: %w", err)
	}

	return nil
}

// ReconcileForwardQuotaRules ensures all forward-chain quota rules appear before
// the forwarding accept rules. After DNAT in prerouting the accept rules terminate
// chain traversal, so any quota rule appended after them is never evaluated.
// This repairs rulesets saved before the insert-at-front fix was applied.
func (n *NFTManager) ReconcileForwardQuotaRules() error {
	n.mu.Lock()
	defer n.mu.Unlock()

	output, err := n.execNFT("-j", "-a", "list", "chain", n.tableFamily, n.tableName, ForwardChainName)
	if err != nil {
		return nil // chain doesn't exist yet, nothing to do
	}

	var ruleset NFTRuleset
	if err := json.Unmarshal(output, &ruleset); err != nil {
		return fmt.Errorf("failed to parse forward chain: %w", err)
	}

	// Walk rules in order; once we've seen the first accept rule, any subsequent
	// quota rule is mispositioned and needs to be moved to the front.
	type quotaToMove struct {
		handle  int64
		port    int
		mbytes  int64
		comment string
	}

	seenAccept := false
	var toMove []quotaToMove

	for _, obj := range ruleset.NFTables {
		r := obj.Rule
		if r == nil || r.Chain != ForwardChainName {
			continue
		}

		isAccept := false
		isQuota := false
		var port int
		var mbytes int64

		for _, expr := range r.Expr {
			if _, ok := expr["accept"]; ok {
				isAccept = true
			}
			if q, ok := expr["quota"].(map[string]interface{}); ok {
				isQuota = true
				if val, ok := q["val"].(float64); ok {
					unit, _ := q["val_unit"].(string)
					mbytes = convertToBytes(int64(val), unit) / (1000 * 1000)
					if mbytes < 1 {
						mbytes = 1
					}
				}
			}
			if mm, ok := expr["match"].(map[string]interface{}); ok {
				if ct, ok := mm["left"].(map[string]interface{}); ok {
					if ctMap, ok := ct["ct"].(map[string]interface{}); ok {
						if ctMap["key"] == "proto-dst" && ctMap["dir"] == "original" {
							if p, ok := mm["right"].(float64); ok {
								port = int(p)
							}
						}
					}
				}
			}
		}

		if isAccept {
			seenAccept = true
		}
		if isQuota && seenAccept && port > 0 {
			toMove = append(toMove, quotaToMove{
				handle:  r.Handle,
				port:    port,
				mbytes:  mbytes,
				comment: r.Comment,
			})
		}
	}

	for _, q := range toMove {
		// Delete the mispositioned rule
		if _, err := n.execNFT("delete", "rule", n.tableFamily, n.tableName, ForwardChainName,
			"handle", strconv.FormatInt(q.handle, 10)); err != nil {
			return fmt.Errorf("failed to delete mispositioned quota rule (handle %d): %w", q.handle, err)
		}
		// Reinsert at the front
		args := []string{
			"insert", "rule", n.tableFamily, n.tableName, ForwardChainName,
			"ct", "original", "proto-dst", strconv.Itoa(q.port),
			"quota", "over", strconv.FormatInt(q.mbytes, 10), "mbytes",
			"drop",
			"comment", fmt.Sprintf(`"%s"`, q.comment),
		}
		if _, err := n.execNFT(args...); err != nil {
			return fmt.Errorf("failed to reinsert quota rule for port %d: %w", q.port, err)
		}
		fmt.Printf("[nft-ui] reconciled forward quota rule for port %d (moved before accept rules)\n", q.port)
	}

	return nil
}

// ReconcileOutputChainQuotaRules removes stale quota rules from the output chain.
// Older releases added a quota rule to the output chain (th sport <port>) alongside
// the forward chain rule. The output chain rule never matched forwarded traffic and
// has been removed from new additions; this function cleans up the leftover rules.
func (n *NFTManager) ReconcileOutputChainQuotaRules() error {
	n.mu.Lock()
	defer n.mu.Unlock()

	output, err := n.execNFT("-j", "-a", "list", "chain", n.tableFamily, n.tableName, n.chainName)
	if err != nil {
		return nil // chain doesn't exist, nothing to do
	}

	var ruleset NFTRuleset
	if err := json.Unmarshal(output, &ruleset); err != nil {
		return fmt.Errorf("failed to parse output chain: %w", err)
	}

	for _, obj := range ruleset.NFTables {
		r := obj.Rule
		if r == nil || r.Chain != n.chainName {
			continue
		}
		hasQuota := false
		hasSport := false
		for _, expr := range r.Expr {
			if _, ok := expr["quota"]; ok {
				hasQuota = true
			}
			if mm, ok := expr["match"].(map[string]interface{}); ok {
				if left, ok := mm["left"].(map[string]interface{}); ok {
					if payload, ok := left["payload"].(map[string]interface{}); ok {
						if payload["field"] == "sport" {
							hasSport = true
						}
					}
				}
			}
		}
		if hasQuota && hasSport {
			if _, err := n.execNFT("delete", "rule", n.tableFamily, n.tableName, n.chainName,
				"handle", strconv.FormatInt(r.Handle, 10)); err != nil {
				return fmt.Errorf("failed to delete stale output quota rule (handle %d): %w", r.Handle, err)
			}
			fmt.Printf("[nft-ui] removed stale output-chain quota rule handle %d\n", r.Handle)
		}
	}

	return nil
}

// RestoreRuleset restores the nftables ruleset from the configured file path
func (n *NFTManager) RestoreRuleset() error {
	if _, err := os.Stat(n.rulesetPath); os.IsNotExist(err) {
		return nil // File doesn't exist yet, skip silently
	}

	// Flush existing ruleset before restoring
	if _, err := n.execNFT("flush", "ruleset"); err != nil {
		return fmt.Errorf("failed to flush ruleset: %w", err)
	}

	if _, err := n.execNFT("-f", n.rulesetPath); err != nil {
		return fmt.Errorf("failed to restore ruleset from %s: %w", n.rulesetPath, err)
	}

	return nil
}
