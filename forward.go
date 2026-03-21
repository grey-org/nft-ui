package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ForwardingComment is the prefix used to identify forwarding rules managed by nft-ui
const ForwardingComment = "nft-ui fwd"

const forwardBypassRestoreComment = "nft-ui fwd bypass restore"

// ForwardingManager handles port forwarding (DNAT + MASQUERADE) operations
type ForwardingManager struct {
	mu                    sync.Mutex
	binary                string
	ipBinary              string
	forwardBypassMark     uint32
	forwardBypassPriority int
	disabledForwardsPath  string
	filterFamily          string
	filterTable           string
}

// NewForwardingManager creates a new ForwardingManager
func NewForwardingManager(cfg *Config) *ForwardingManager {
	path := cfg.DisabledForwardsPath
	if path == "" {
		path = "/var/lib/nft-ui/disabled-forwards.json"
	}
	return &ForwardingManager{
		binary:                cfg.NFTBinary,
		ipBinary:              cfg.IPBinary,
		forwardBypassMark:     cfg.ForwardBypassMark,
		forwardBypassPriority: cfg.ForwardBypassPriority,
		disabledForwardsPath:  path,
		filterFamily:          cfg.TableFamily,
		filterTable:           cfg.TableName,
	}
}

func (m *ForwardingManager) managedForwardChainRef() (string, string, string) {
	return m.filterFamily, m.filterTable, ForwardChainName
}

func (m *ForwardingManager) legacyForwardChainRef() (string, string, string) {
	return "ip", "filter", ForwardChainName
}

// execNFT executes an nft command and returns the output
func (m *ForwardingManager) execNFT(args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, m.binary, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("nft %s: %w (output: %s)", strings.Join(args, " "), err, string(output))
	}
	return output, nil
}

// execIP executes an ip command and returns the output
func (m *ForwardingManager) execIP(args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, m.ipBinary, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("ip %s: %w (output: %s)", strings.Join(args, " "), err, string(output))
	}
	return output, nil
}

func (m *ForwardingManager) forwardBypassEnabled() bool {
	return m.forwardBypassMark != 0
}

func (m *ForwardingManager) forwardBypassMarkValue() string {
	return fmt.Sprintf("0x%x", m.forwardBypassMark)
}

// EnsureFilterForwardSetup ensures the configured filter table and forward chain exist.
func (m *ForwardingManager) EnsureFilterForwardSetup() error {
	family, table, chain := m.managedForwardChainRef()

	// Check if filter table exists
	_, err := m.execNFT("list", "table", family, table)
	if err != nil {
		if _, err := m.execNFT("add", "table", family, table); err != nil {
			return fmt.Errorf("failed to create filter table %s %s: %w", family, table, err)
		}
	}

	// Check if forward chain exists
	_, err = m.execNFT("list", "chain", family, table, chain)
	if err != nil {
		if _, err := m.execNFT("add", "chain", family, table, chain,
			"{ type filter hook forward priority filter ; policy accept ; }"); err != nil {
			return fmt.Errorf("failed to create %s chain in %s %s: %w", chain, family, table, err)
		}
	}

	return nil
}

// EnsureLegacyMSSForwardSetup keeps the legacy ip filter forward chain available
// for MSS rules that are evaluated separately from the main managed filter chain.
func (m *ForwardingManager) EnsureLegacyMSSForwardSetup() error {
	// Check if filter table exists
	_, err := m.execNFT("list", "table", "ip", "filter")
	if err != nil {
		// Create filter table
		if _, err := m.execNFT("add", "table", "ip", "filter"); err != nil {
			return fmt.Errorf("failed to create filter table: %w", err)
		}
	}

	// Check if forward chain exists
	_, err = m.execNFT("list", "chain", "ip", "filter", "forward")
	if err != nil {
		// Create forward chain
		if _, err := m.execNFT("add", "chain", "ip", "filter", "forward",
			"{ type filter hook forward priority filter ; policy accept ; }"); err != nil {
			return fmt.Errorf("failed to create forward chain: %w", err)
		}
	}

	// Older releases inserted a global conntrack fast-path rule here, but that
	// bypasses later MSS and limit rules. Clean it up if present.
	if err := m.deleteConntrackFastPath(); err != nil {
		return err
	}

	return nil
}

// deleteConntrackFastPath removes the legacy global conntrack fast-path rule so
// later MSS and limit rules in the same chain still see reply packets.
func (m *ForwardingManager) deleteConntrackFastPath() error {
	const ctComment = "nft-ui ct-fastpath"

	output, err := m.execNFT("-j", "-a", "list", "chain", "ip", "filter", "forward")
	if err != nil {
		return nil // best effort
	}

	var ruleset NFTRuleset
	if err := json.Unmarshal(output, &ruleset); err != nil {
		return nil
	}

	for _, obj := range ruleset.NFTables {
		if obj.Rule == nil || obj.Rule.Chain != "forward" {
			continue
		}
		if obj.Rule.Comment == ctComment {
			if _, err := m.execNFT("delete", "rule", "ip", "filter", "forward",
				"handle", strconv.FormatInt(obj.Rule.Handle, 10)); err != nil {
				return fmt.Errorf("failed to delete legacy conntrack fast-path rule: %w", err)
			}
		}
	}

	return nil
}

// EnsureNatSetup ensures the nat table and required chains exist
func (m *ForwardingManager) EnsureNatSetup() error {
	// Check if nat table exists by trying to list it
	_, err := m.execNFT("list", "table", "ip", "nat")
	if err != nil {
		// Table doesn't exist, create it
		if _, err := m.execNFT("add", "table", "ip", "nat"); err != nil {
			return fmt.Errorf("failed to create nat table: %w", err)
		}
	}

	// Check if prerouting chain exists
	_, err = m.execNFT("list", "chain", "ip", "nat", "prerouting")
	if err != nil {
		// Create prerouting chain
		if _, err := m.execNFT("add", "chain", "ip", "nat", "prerouting",
			"{ type nat hook prerouting priority -100 ; policy accept ; }"); err != nil {
			return fmt.Errorf("failed to create prerouting chain: %w", err)
		}
	}

	// Check if postrouting chain exists
	_, err = m.execNFT("list", "chain", "ip", "nat", "postrouting")
	if err != nil {
		// Create postrouting chain
		if _, err := m.execNFT("add", "chain", "ip", "nat", "postrouting",
			"{ type nat hook postrouting priority 100 ; policy accept ; }"); err != nil {
			return fmt.Errorf("failed to create postrouting chain: %w", err)
		}
	}

	// Check if output chain exists
	_, err = m.execNFT("list", "chain", "ip", "nat", "output")
	if err != nil {
		// Create output chain
		if _, err := m.execNFT("add", "chain", "ip", "nat", "output",
			"{ type nat hook output priority -100 ; policy accept ; }"); err != nil {
			return fmt.Errorf("failed to create output chain: %w", err)
		}
	}

	return nil
}

// EnsureForwardBypassSetup creates the fwmark infrastructure used to keep
// forwarded flows on the main routing table when policy-routing TUNs are active.
func (m *ForwardingManager) EnsureForwardBypassSetup() error {
	if !m.forwardBypassEnabled() {
		return nil
	}

	// Ensure mangle table exists
	if _, err := m.execNFT("list", "table", "ip", "mangle"); err != nil {
		if _, err := m.execNFT("add", "table", "ip", "mangle"); err != nil {
			return fmt.Errorf("failed to create mangle table: %w", err)
		}
	}

	// Mark forwarded packets before route lookup.
	if _, err := m.execNFT("list", "chain", "ip", "mangle", "prerouting"); err != nil {
		if _, err := m.execNFT("add", "chain", "ip", "mangle", "prerouting",
			"{ type filter hook prerouting priority mangle ; policy accept ; }"); err != nil {
			return fmt.Errorf("failed to create mangle prerouting chain: %w", err)
		}
	}

	// Mark locally-originated connections that hit the output DNAT path.
	if _, err := m.execNFT("list", "chain", "ip", "mangle", "output"); err != nil {
		if _, err := m.execNFT("add", "chain", "ip", "mangle", "output",
			"{ type route hook output priority mangle ; policy accept ; }"); err != nil {
			return fmt.Errorf("failed to create mangle output chain: %w", err)
		}
	}

	if err := m.ensureForwardBypassRestoreRule(); err != nil {
		return err
	}

	if err := m.ensureForwardBypassIPRule(); err != nil {
		return err
	}

	return nil
}

func (m *ForwardingManager) ensureForwardBypassRestoreRule() error {
	output, err := m.execNFT("-j", "-a", "list", "chain", "ip", "mangle", "prerouting")
	if err == nil {
		var ruleset NFTRuleset
		if err := json.Unmarshal(output, &ruleset); err == nil {
			for _, obj := range ruleset.NFTables {
				if obj.Rule == nil || obj.Rule.Chain != "prerouting" {
					continue
				}
				if obj.Rule.Comment == forwardBypassRestoreComment {
					return nil
				}
			}
		}
	}

	if _, err := m.execNFT("insert", "rule", "ip", "mangle", "prerouting",
		"ct", "mark", m.forwardBypassMarkValue(),
		"meta", "mark", "set", m.forwardBypassMarkValue(),
		"comment", fmt.Sprintf(`"%s"`, forwardBypassRestoreComment)); err != nil {
		return fmt.Errorf("failed to add bypass restore rule: %w", err)
	}

	return nil
}

func (m *ForwardingManager) ensureForwardBypassIPRule() error {
	output, err := m.execIP("-4", "rule", "show")
	if err == nil {
		expectedRule := fmt.Sprintf("%d: from all fwmark %s lookup main",
			m.forwardBypassPriority, strings.ToLower(m.forwardBypassMarkValue()))
		if strings.Contains(strings.ToLower(string(output)), expectedRule) {
			return nil
		}
	}

	if _, err := m.execIP("-4", "rule", "add",
		"priority", strconv.Itoa(m.forwardBypassPriority),
		"fwmark", m.forwardBypassMarkValue(),
		"lookup", "main"); err != nil {
		if strings.Contains(err.Error(), "File exists") {
			return nil
		}
		return fmt.Errorf("failed to add bypass ip rule: %w", err)
	}

	return nil
}

// SyncForwardBypassRules backfills bypass-mark rules for already-existing managed forwards.
func (m *ForwardingManager) SyncForwardBypassRules() error {
	if !m.forwardBypassEnabled() {
		return nil
	}

	preOutput, err := m.execNFT("-j", "-a", "list", "chain", "ip", "nat", "prerouting")
	if err != nil {
		return nil
	}

	postOutput, err := m.execNFT("-j", "-a", "list", "chain", "ip", "nat", "postrouting")
	if err != nil {
		return nil
	}

	rules, err := m.parseForwardingRules(preOutput, postOutput)
	if err != nil {
		return err
	}

	for _, rule := range rules {
		if !rule.Managed || !rule.Enabled {
			continue
		}

		fullComment := fmt.Sprintf("%s %d", ForwardingComment, rule.SrcPort)
		if rule.Comment != "" {
			fullComment = fmt.Sprintf("%s %s", fullComment, rule.Comment)
		}

		if err := m.ensureRouteMarkRule("prerouting", rule.SrcPort, rule.Protocol, fullComment); err != nil {
			return err
		}
		if err := m.ensureRouteMarkRule("output", rule.SrcPort, rule.Protocol, fullComment); err != nil {
			return err
		}
	}

	return nil
}

// ListForwardingRules returns all forwarding rules (enabled from nftables + disabled from file)
func (m *ForwardingManager) ListForwardingRules() ([]ForwardingRule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Ensure nat table exists
	if err := m.EnsureNatSetup(); err != nil {
		return nil, err
	}

	// Get prerouting rules (DNAT)
	preOutput, err := m.execNFT("-j", "-a", "list", "chain", "ip", "nat", "prerouting")
	if err != nil {
		return nil, fmt.Errorf("failed to list prerouting chain: %w", err)
	}

	// Get postrouting rules (MASQUERADE)
	postOutput, err := m.execNFT("-j", "-a", "list", "chain", "ip", "nat", "postrouting")
	if err != nil {
		return nil, fmt.Errorf("failed to list postrouting chain: %w", err)
	}

	// Parse enabled rules from nftables
	enabledRules, err := m.parseForwardingRules(preOutput, postOutput)
	if err != nil {
		return nil, err
	}

	// Get limit, MSS, and source NAT information
	limitMap := m.extractLimitsFromForwardChain()
	mssModeMap := m.extractMSSModesFromForwardChain()
	sourceNATMap := m.extractSourceNATFromPostrouting(postOutput)

	// Apply limits, MSS modes, and source NAT settings to enabled rules
	for i := range enabledRules {
		if limit, ok := limitMap[enabledRules[i].SrcPort]; ok {
			enabledRules[i].LimitMbps = limit
		}
		if mssMode, ok := mssModeMap[enabledRules[i].SrcPort]; ok {
			enabledRules[i].MSSMode = mssMode
		} else if enabledRules[i].Managed {
			enabledRules[i].MSSMode = MSSModeFixed1452
		}
		if natInfo, ok := sourceNATMap[enabledRules[i].SrcPort]; ok {
			enabledRules[i].SourceNATMode = natInfo.Mode
			enabledRules[i].SNATAddress = natInfo.Address
		} else if enabledRules[i].Managed {
			enabledRules[i].SourceNATMode = SourceNATModeMasquerade
		}
	}

	// Load disabled rules from file
	disabledRules, err := m.loadDisabledRules()
	if err != nil {
		// If file doesn't exist, that's fine
		disabledRules = []ForwardingRule{}
	}

	// Merge enabled and disabled rules
	allRules := append(enabledRules, disabledRules...)
	return allRules, nil
}

func (m *ForwardingManager) mergeLimitMapFromChain(limitMap map[int]int, family, table, chain string) {
	output, err := m.execNFT("-j", "-a", "list", "chain", family, table, chain)
	if err != nil {
		return
	}

	var ruleset NFTRuleset
	if err := json.Unmarshal(output, &ruleset); err != nil {
		return
	}

	for _, obj := range ruleset.NFTables {
		if obj.Rule == nil || obj.Rule.Chain != chain {
			continue
		}
		if !strings.HasPrefix(obj.Rule.Comment, ForwardingComment) {
			continue
		}

		srcPort := m.extractSrcPortFromComment(obj.Rule.Comment)
		if srcPort == 0 {
			continue
		}

		for _, expr := range obj.Rule.Expr {
			if limitData, ok := expr["limit"]; ok {
				if lm, ok := limitData.(map[string]interface{}); ok {
					if rate, ok := lm["rate"].(float64); ok {
						limitMap[srcPort] = int((rate * 8) / 1000)
						break
					}
				}
			}
		}
	}
}

// extractLimitsFromForwardChain extracts bandwidth limits from both the legacy
// ip filter forward chain and the active managed filter forward chain.
func (m *ForwardingManager) extractLimitsFromForwardChain() map[int]int {
	limitMap := make(map[int]int)

	legacyFamily, legacyTable, legacyChain := m.legacyForwardChainRef()
	m.mergeLimitMapFromChain(limitMap, legacyFamily, legacyTable, legacyChain)

	family, table, chain := m.managedForwardChainRef()
	if family != legacyFamily || table != legacyTable {
		m.mergeLimitMapFromChain(limitMap, family, table, chain)
	}

	return limitMap
}

type sourceNATInfo struct {
	Mode    string
	Address string
}

func (m *ForwardingManager) extractSourceNATFromPostrouting(postData []byte) map[int]sourceNATInfo {
	sourceNATMap := make(map[int]sourceNATInfo)

	var ruleset NFTRuleset
	if err := json.Unmarshal(postData, &ruleset); err != nil {
		return sourceNATMap
	}

	for _, obj := range ruleset.NFTables {
		if obj.Rule == nil || obj.Rule.Chain != "postrouting" {
			continue
		}
		if !strings.HasPrefix(obj.Rule.Comment, ForwardingComment) {
			continue
		}

		srcPort := m.extractSrcPortFromComment(obj.Rule.Comment)
		if srcPort == 0 {
			continue
		}

		info := sourceNATInfo{Mode: SourceNATModeMasquerade}
		for _, expr := range obj.Rule.Expr {
			if snatData, ok := expr["snat"]; ok {
				if sm, ok := snatData.(map[string]interface{}); ok {
					info.Mode = SourceNATModeSNAT
					if addr, ok := sm["addr"].(string); ok {
						info.Address = addr
					}
				}
				break
			}
			if _, ok := expr["masquerade"]; ok {
				info.Mode = SourceNATModeMasquerade
				info.Address = ""
				break
			}
		}
		sourceNATMap[srcPort] = info
	}

	return sourceNATMap
}

func (m *ForwardingManager) mergeMSSModeMapFromChain(mssModeMap map[int]string, family, table, chain string) {
	output, err := m.execNFT("-j", "-a", "list", "chain", family, table, chain)
	if err != nil {
		return
	}

	var ruleset NFTRuleset
	if err := json.Unmarshal(output, &ruleset); err != nil {
		return
	}

	for _, obj := range ruleset.NFTables {
		if obj.Rule == nil || obj.Rule.Chain != chain {
			continue
		}
		if !strings.HasPrefix(obj.Rule.Comment, ForwardingComment) {
			continue
		}

		srcPort := m.extractSrcPortFromComment(obj.Rule.Comment)
		if srcPort == 0 {
			continue
		}

		exprJSON, err := json.Marshal(obj.Rule.Expr)
		if err != nil {
			continue
		}
		exprText := string(exprJSON)
		if strings.Contains(exprText, `"rt"`) && strings.Contains(exprText, `"mtu"`) {
			mssModeMap[srcPort] = MSSModePMTU
			continue
		}
		if strings.Contains(exprText, `1452`) {
			mssModeMap[srcPort] = MSSModeFixed1452
		}
	}
}

func (m *ForwardingManager) extractMSSModesFromForwardChain() map[int]string {
	mssModeMap := make(map[int]string)

	legacyFamily, legacyTable, legacyChain := m.legacyForwardChainRef()
	m.mergeMSSModeMapFromChain(mssModeMap, legacyFamily, legacyTable, legacyChain)

	family, table, chain := m.managedForwardChainRef()
	if family != legacyFamily || table != legacyTable {
		m.mergeMSSModeMapFromChain(mssModeMap, family, table, chain)
	}

	return mssModeMap
}

func (m *ForwardingManager) chainHasCommentPrefix(family, table, chain, commentPrefix string) bool {
	output, err := m.execNFT("-j", "-a", "list", "chain", family, table, chain)
	if err != nil {
		return false
	}

	var ruleset NFTRuleset
	if err := json.Unmarshal(output, &ruleset); err != nil {
		return false
	}

	for _, obj := range ruleset.NFTables {
		if obj.Rule == nil || obj.Rule.Chain != chain {
			continue
		}
		if strings.HasPrefix(obj.Rule.Comment, commentPrefix) {
			return true
		}
	}

	return false
}

// ReconcileManagedForwardingRules repairs managed forwarding rules created by
// older releases by ensuring the active forward chain has explicit accept rules.
func (m *ForwardingManager) ReconcileManagedForwardingRules() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.deleteConntrackFastPath(); err != nil {
		return err
	}

	preOutput, err := m.execNFT("-j", "-a", "list", "chain", "ip", "nat", "prerouting")
	if err != nil {
		return nil
	}
	postOutput, err := m.execNFT("-j", "-a", "list", "chain", "ip", "nat", "postrouting")
	if err != nil {
		return nil
	}

	rules, err := m.parseForwardingRules(preOutput, postOutput)
	if err != nil {
		return err
	}

	family, table, chain := m.managedForwardChainRef()
	for _, rule := range rules {
		if !rule.Managed {
			continue
		}

		commentPrefix := fmt.Sprintf("%s %d", ForwardingComment, rule.SrcPort)
		if m.chainHasCommentPrefix(family, table, chain, commentPrefix) {
			continue
		}

		fullComment := commentPrefix
		if rule.Comment != "" {
			fullComment = fmt.Sprintf("%s %s", fullComment, rule.Comment)
		}

		if err := m.addForwardAcceptRules(rule.DstIP, rule.DstPort, rule.Protocol, fullComment); err != nil {
			return fmt.Errorf("failed to reconcile forward rule %d: %w", rule.SrcPort, err)
		}
	}

	return nil
}

// parseForwardingRules parses JSON output from prerouting and postrouting chains
func (m *ForwardingManager) parseForwardingRules(preData, postData []byte) ([]ForwardingRule, error) {
	var preRuleset, postRuleset NFTRuleset
	if err := json.Unmarshal(preData, &preRuleset); err != nil {
		return nil, fmt.Errorf("failed to parse prerouting JSON: %w", err)
	}
	if err := json.Unmarshal(postData, &postRuleset); err != nil {
		return nil, fmt.Errorf("failed to parse postrouting JSON: %w", err)
	}

	// Build a map of postrouting handles by srcPort for managed rules
	postHandles := make(map[int]int64)
	for _, obj := range postRuleset.NFTables {
		if obj.Rule == nil || obj.Rule.Chain != "postrouting" {
			continue
		}
		if strings.HasPrefix(obj.Rule.Comment, ForwardingComment) {
			srcPort := m.extractSrcPortFromComment(obj.Rule.Comment)
			if srcPort > 0 {
				postHandles[srcPort] = obj.Rule.Handle
			}
		}
	}

	var rules []ForwardingRule

	// Parse ALL prerouting rules that have DNAT
	for _, obj := range preRuleset.NFTables {
		if obj.Rule == nil || obj.Rule.Chain != "prerouting" {
			continue
		}

		rule := m.extractForwardingRule(obj.Rule)
		if rule != nil {
			rule.Enabled = true
			rule.PreHandle = obj.Rule.Handle

			// Check if this is a managed rule (has our comment)
			if strings.HasPrefix(obj.Rule.Comment, ForwardingComment) {
				rule.Managed = true
				// Try to find matching postrouting handle
				if handle, ok := postHandles[rule.SrcPort]; ok {
					rule.PostHandle = handle
				}
			} else {
				rule.Managed = false
			}

			rules = append(rules, *rule)
		}
	}

	return rules, nil
}

// extractSrcPortFromComment extracts source port from comment like "nft-ui fwd 12103 some comment"
func (m *ForwardingManager) extractSrcPortFromComment(comment string) int {
	parts := strings.Fields(comment)
	if len(parts) >= 3 {
		port, err := strconv.Atoi(parts[2])
		if err == nil {
			return port
		}
	}
	return 0
}

// extractForwardingRule extracts forwarding rule info from a prerouting DNAT rule
func (m *ForwardingManager) extractForwardingRule(rule *NFTRule) *ForwardingRule {
	var srcPort, dstPort int
	var dstIP string
	var protocol string

	for _, expr := range rule.Expr {
		// Look for protocol meta match
		if metaData, ok := expr["match"]; ok {
			if mm, ok := metaData.(map[string]interface{}); ok {
				if left, ok := mm["left"].(map[string]interface{}); ok {
					if meta, ok := left["meta"].(map[string]interface{}); ok {
						if key, ok := meta["key"].(string); ok && key == "l4proto" {
							// Check for protocol set
							if right, ok := mm["right"].(map[string]interface{}); ok {
								if set, ok := right["set"].([]interface{}); ok {
									hasTCP := false
									hasUDP := false
									for _, p := range set {
										if ps, ok := p.(string); ok {
											if ps == "tcp" {
												hasTCP = true
											} else if ps == "udp" {
												hasUDP = true
											}
										}
									}
									if hasTCP && hasUDP {
										protocol = "both"
									} else if hasTCP {
										protocol = "tcp"
									} else if hasUDP {
										protocol = "udp"
									}
								}
							}
						}
					}
					// Check for payload match (dport)
					if payload, ok := left["payload"].(map[string]interface{}); ok {
						if field, ok := payload["field"].(string); ok && field == "dport" {
							if right, ok := mm["right"].(float64); ok {
								srcPort = int(right)
							}
						}
					}
				}
			}
		}

		// Look for DNAT expression
		if dnatData, ok := expr["dnat"]; ok {
			if dm, ok := dnatData.(map[string]interface{}); ok {
				if addr, ok := dm["addr"].(string); ok {
					dstIP = addr
				}
				if port, ok := dm["port"].(float64); ok {
					dstPort = int(port)
				}
			}
		}
	}

	// If we couldn't determine protocol, try from the rule structure
	if protocol == "" {
		for _, expr := range rule.Expr {
			if matchData, ok := expr["match"]; ok {
				if mm, ok := matchData.(map[string]interface{}); ok {
					if left, ok := mm["left"].(map[string]interface{}); ok {
						if payload, ok := left["payload"].(map[string]interface{}); ok {
							if proto, ok := payload["protocol"].(string); ok {
								protocol = proto
							}
						}
					}
				}
			}
		}
	}

	if protocol == "" {
		protocol = "both" // Default to both if not determined
	}

	if srcPort == 0 || dstIP == "" || dstPort == 0 {
		return nil
	}

	// Extract user comment based on whether it's a managed rule
	userComment := ""
	if strings.HasPrefix(rule.Comment, ForwardingComment) {
		// Managed rule: extract part after "nft-ui fwd <port>"
		parts := strings.Fields(rule.Comment)
		if len(parts) > 3 {
			userComment = strings.Join(parts[3:], " ")
		}
	} else {
		// Unmanaged rule: use the raw comment
		userComment = rule.Comment
	}

	return &ForwardingRule{
		ID:            fmt.Sprintf("fwd_%d", srcPort),
		SrcPort:       srcPort,
		DstIP:         dstIP,
		DstPort:       dstPort,
		Protocol:      protocol,
		Comment:       userComment,
		MSSMode:       MSSModeFixed1452,
		SourceNATMode: SourceNATModeMasquerade,
		// LimitMbps and SNATAddress will be filled by chain-specific extraction.
	}
}

// AddForwardingRule adds a new port forwarding rule
func (m *ForwardingManager) AddForwardingRule(srcPort int, dstIP string, dstPort int, protocol string, comment string, limitMbps int, mssMode string, sourceNATMode string, snatAddress string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Validate inputs
	if srcPort < 1 || srcPort > 65535 {
		return fmt.Errorf("invalid source port: %d", srcPort)
	}
	if dstPort < 1 || dstPort > 65535 {
		return fmt.Errorf("invalid destination port: %d", dstPort)
	}
	if !isValidIPv4(dstIP) {
		return fmt.Errorf("invalid destination IP: %s", dstIP)
	}
	if protocol != "tcp" && protocol != "udp" && protocol != "both" {
		return fmt.Errorf("invalid protocol: %s", protocol)
	}

	// Sanitize comment
	comment = sanitizeComment(comment)

	// Ensure nat table exists
	if err := m.EnsureNatSetup(); err != nil {
		return err
	}

	// Check for duplicate source port
	preOutput, _ := m.execNFT("-j", "-a", "list", "chain", "ip", "nat", "prerouting")
	postOutput, _ := m.execNFT("-j", "-a", "list", "chain", "ip", "nat", "postrouting")
	existingRules, _ := m.parseForwardingRules(preOutput, postOutput)
	for _, r := range existingRules {
		if r.SrcPort == srcPort {
			return fmt.Errorf("source port %d is already in use", srcPort)
		}
	}

	// Also check disabled rules
	disabledRules, _ := m.loadDisabledRules()
	for _, r := range disabledRules {
		if r.SrcPort == srcPort {
			return fmt.Errorf("source port %d is already in use (disabled rule)", srcPort)
		}
	}

	// Validate limit
	if limitMbps < 0 {
		return fmt.Errorf("invalid limit: %d (must be >= 0)", limitMbps)
	}
	mssMode = normalizeMSSMode(mssMode)
	if mssMode == "" {
		return fmt.Errorf("invalid MSS mode: %s", mssMode)
	}
	sourceNATMode = normalizeSourceNATMode(sourceNATMode)
	if sourceNATMode == "" {
		return fmt.Errorf("invalid source NAT mode: %s", sourceNATMode)
	}
	if sourceNATMode == SourceNATModeSNAT {
		if !isValidIPv4(snatAddress) {
			return fmt.Errorf("invalid SNAT address: %s", snatAddress)
		}
	} else {
		snatAddress = ""
	}

	// Build comment string
	fullComment := fmt.Sprintf("%s %d", ForwardingComment, srcPort)
	if comment != "" {
		fullComment = fmt.Sprintf("%s %s", fullComment, comment)
	}

	// Add prerouting DNAT rule
	if err := m.addDNATRule(srcPort, dstIP, dstPort, protocol, fullComment, limitMbps); err != nil {
		return fmt.Errorf("failed to add DNAT rule: %w", err)
	}

	// Add postrouting MASQUERADE rule
	if err := m.addSourceNATRule(dstIP, dstPort, protocol, fullComment, sourceNATMode, snatAddress); err != nil {
		// Rollback: delete the DNAT rule
		m.deleteDNATRuleBySrcPort(srcPort)
		return fmt.Errorf("failed to add source NAT rule: %w", err)
	}

	// Add output DNAT rule for local traffic
	if err := m.addOutputDNATRule(srcPort, dstIP, dstPort, protocol, fullComment, limitMbps); err != nil {
		// Rollback: delete previous rules
		m.deleteDNATRuleBySrcPort(srcPort)
		m.deleteMasqueradeRuleBySrcPort(srcPort)
		return fmt.Errorf("failed to add output DNAT rule: %w", err)
	}

	if err := m.addRouteMarkRules(srcPort, protocol, fullComment); err != nil {
		m.deleteDNATRuleBySrcPort(srcPort)
		m.deleteMasqueradeRuleBySrcPort(srcPort)
		m.deleteOutputDNATRuleBySrcPort(srcPort)
		return fmt.Errorf("failed to add bypass mark rules: %w", err)
	}

	// Add bandwidth limit rules in filter forward chain (if limit > 0)
	if err := m.addForwardLimitRules(dstIP, dstPort, protocol, fullComment, limitMbps); err != nil {
		// Rollback: delete previous rules
		m.deleteDNATRuleBySrcPort(srcPort)
		m.deleteMasqueradeRuleBySrcPort(srcPort)
		m.deleteOutputDNATRuleBySrcPort(srcPort)
		m.deleteRouteMarkRules(srcPort)
		return fmt.Errorf("failed to add forward limit rules: %w", err)
	}

	if err := m.addForwardAcceptRules(dstIP, dstPort, protocol, fullComment); err != nil {
		m.deleteDNATRuleBySrcPort(srcPort)
		m.deleteMasqueradeRuleBySrcPort(srcPort)
		m.deleteOutputDNATRuleBySrcPort(srcPort)
		m.deleteForwardFilterRules(srcPort)
		return fmt.Errorf("failed to add forward accept rules: %w", err)
	}

	// Add TCP MSS handling rule to prevent MTU-related stalls
	if err := m.addMSSClampRule(dstIP, dstPort, protocol, fullComment, mssMode); err != nil {
		// Rollback: delete previous rules
		m.deleteDNATRuleBySrcPort(srcPort)
		m.deleteMasqueradeRuleBySrcPort(srcPort)
		m.deleteOutputDNATRuleBySrcPort(srcPort)
		m.deleteRouteMarkRules(srcPort)
		m.deleteForwardFilterRules(srcPort)
		return fmt.Errorf("failed to add MSS clamp rule: %w", err)
	}

	return nil
}

// addDNATRule adds a prerouting DNAT rule (without limit - limit goes in filter forward)
func (m *ForwardingManager) addDNATRule(srcPort int, dstIP string, dstPort int, protocol string, comment string, limitMbps int) error {
	var args []string

	switch protocol {
	case "tcp":
		args = []string{
			"add", "rule", "ip", "nat", "prerouting",
			"tcp", "dport", strconv.Itoa(srcPort),
			"dnat", "to", fmt.Sprintf("%s:%d", dstIP, dstPort),
			"comment", fmt.Sprintf(`"%s"`, comment),
		}
	case "udp":
		args = []string{
			"add", "rule", "ip", "nat", "prerouting",
			"udp", "dport", strconv.Itoa(srcPort),
			"dnat", "to", fmt.Sprintf("%s:%d", dstIP, dstPort),
			"comment", fmt.Sprintf(`"%s"`, comment),
		}
	default: // "both"
		args = []string{
			"add", "rule", "ip", "nat", "prerouting",
			"meta", "l4proto", "{", "tcp,", "udp", "}",
			"th", "dport", strconv.Itoa(srcPort),
			"dnat", "to", fmt.Sprintf("%s:%d", dstIP, dstPort),
			"comment", fmt.Sprintf(`"%s"`, comment),
		}
	}

	_, err := m.execNFT(args...)
	return err
}

// addSourceNATRule adds a postrouting source NAT rule using masquerade or fixed SNAT.
func (m *ForwardingManager) addSourceNATRule(dstIP string, dstPort int, protocol string, comment string, sourceNATMode string, snatAddress string) error {
	sourceNATMode = normalizeSourceNATMode(sourceNATMode)
	if sourceNATMode == "" {
		return fmt.Errorf("invalid source NAT mode")
	}
	if sourceNATMode == SourceNATModeSNAT && !isValidIPv4(snatAddress) {
		return fmt.Errorf("invalid SNAT address: %s", snatAddress)
	}

	buildArgs := func(protoArgs []string) []string {
		args := []string{"add", "rule", "ip", "nat", "postrouting", "ip", "daddr", dstIP}
		args = append(args, protoArgs...)
		if sourceNATMode == SourceNATModeSNAT {
			args = append(args, "snat", "to", snatAddress)
		} else {
			args = append(args, "masquerade")
		}
		args = append(args, "comment", fmt.Sprintf(`"%s"`, comment))
		return args
	}

	var args []string
	switch protocol {
	case "tcp":
		args = buildArgs([]string{"tcp", "dport", strconv.Itoa(dstPort)})
	case "udp":
		args = buildArgs([]string{"udp", "dport", strconv.Itoa(dstPort)})
	default: // "both"
		args = buildArgs([]string{"meta", "l4proto", "{", "tcp,", "udp", "}", "th", "dport", strconv.Itoa(dstPort)})
	}

	_, err := m.execNFT(args...)
	return err
}

// addOutputDNATRule adds an output chain DNAT rule for local traffic (without limit)
func (m *ForwardingManager) addOutputDNATRule(srcPort int, dstIP string, dstPort int, protocol string, comment string, limitMbps int) error {
	var args []string

	switch protocol {
	case "tcp":
		args = []string{
			"add", "rule", "ip", "nat", "output",
			"tcp", "dport", strconv.Itoa(srcPort),
			"dnat", "to", fmt.Sprintf("%s:%d", dstIP, dstPort),
			"comment", fmt.Sprintf(`"%s"`, comment),
		}
	case "udp":
		args = []string{
			"add", "rule", "ip", "nat", "output",
			"udp", "dport", strconv.Itoa(srcPort),
			"dnat", "to", fmt.Sprintf("%s:%d", dstIP, dstPort),
			"comment", fmt.Sprintf(`"%s"`, comment),
		}
	default: // "both"
		args = []string{
			"add", "rule", "ip", "nat", "output",
			"meta", "l4proto", "{", "tcp,", "udp", "}",
			"th", "dport", strconv.Itoa(srcPort),
			"dnat", "to", fmt.Sprintf("%s:%d", dstIP, dstPort),
			"comment", fmt.Sprintf(`"%s"`, comment),
		}
	}

	_, err := m.execNFT(args...)
	return err
}

// addRouteMarkRules tags forwarding traffic so policy routing can keep it on the main table.
func (m *ForwardingManager) addRouteMarkRules(srcPort int, protocol string, comment string) error {
	if !m.forwardBypassEnabled() {
		return nil
	}

	if err := m.EnsureForwardBypassSetup(); err != nil {
		return err
	}

	if err := m.addRouteMarkRule("prerouting", srcPort, protocol, comment); err != nil {
		return err
	}

	if err := m.addRouteMarkRule("output", srcPort, protocol, comment); err != nil {
		m.deleteRouteMarkRules(srcPort)
		return err
	}

	return nil
}

func (m *ForwardingManager) ensureRouteMarkRule(chain string, srcPort int, protocol string, comment string) error {
	exists, err := m.routeMarkRuleExists(chain, srcPort)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	return m.addRouteMarkRule(chain, srcPort, protocol, comment)
}

func (m *ForwardingManager) routeMarkRuleExists(chain string, srcPort int) (bool, error) {
	output, err := m.execNFT("-j", "-a", "list", "chain", "ip", "mangle", chain)
	if err != nil {
		return false, nil
	}

	var ruleset NFTRuleset
	if err := json.Unmarshal(output, &ruleset); err != nil {
		return false, err
	}

	commentPrefix := fmt.Sprintf("%s %d", ForwardingComment, srcPort)
	for _, obj := range ruleset.NFTables {
		if obj.Rule == nil || obj.Rule.Chain != chain {
			continue
		}
		if strings.HasPrefix(obj.Rule.Comment, commentPrefix) {
			return true, nil
		}
	}

	return false, nil
}

func (m *ForwardingManager) addRouteMarkRule(chain string, srcPort int, protocol string, comment string) error {
	var args []string

	switch protocol {
	case "tcp":
		args = []string{
			"add", "rule", "ip", "mangle", chain,
			"tcp", "dport", strconv.Itoa(srcPort),
			"meta", "mark", "set", m.forwardBypassMarkValue(),
			"ct", "mark", "set", m.forwardBypassMarkValue(),
			"comment", fmt.Sprintf(`"%s"`, comment),
		}
	case "udp":
		args = []string{
			"add", "rule", "ip", "mangle", chain,
			"udp", "dport", strconv.Itoa(srcPort),
			"meta", "mark", "set", m.forwardBypassMarkValue(),
			"ct", "mark", "set", m.forwardBypassMarkValue(),
			"comment", fmt.Sprintf(`"%s"`, comment),
		}
	default: // "both"
		args = []string{
			"add", "rule", "ip", "mangle", chain,
			"meta", "l4proto", "{", "tcp,", "udp", "}",
			"th", "dport", strconv.Itoa(srcPort),
			"meta", "mark", "set", m.forwardBypassMarkValue(),
			"ct", "mark", "set", m.forwardBypassMarkValue(),
			"comment", fmt.Sprintf(`"%s"`, comment),
		}
	}

	if _, err := m.execNFT(args...); err != nil {
		return fmt.Errorf("failed to add %s bypass mark rule: %w", chain, err)
	}

	return nil
}

// addMSSClampRule adds bidirectional TCP MSS rules in the legacy ip filter
// forward chain. MSS handling only applies to TCP traffic.
// Supported modes: pmtu (recommended), fixed1452 (legacy), disabled.
func (m *ForwardingManager) addMSSClampRule(dstIP string, dstPort int, protocol string, comment string, mode string) error {
	mode = normalizeMSSMode(mode)
	if mode == "" {
		return fmt.Errorf("invalid MSS mode")
	}
	if mode == MSSModeDisabled || protocol == "udp" {
		return nil
	}

	// Ensure legacy filter table and forward chain exist
	if err := m.EnsureLegacyMSSForwardSetup(); err != nil {
		return err
	}

	setArgs := []string{"rt", "mtu"}
	if mode == MSSModeFixed1452 {
		setArgs = []string{"1452"}
	}

	// Outbound: to destination
	outArgs := []string{"add", "rule", "ip", "filter", "forward",
		"ip", "daddr", dstIP,
		"tcp", "dport", strconv.Itoa(dstPort),
		"tcp", "flags", "syn",
		"tcp", "option", "maxseg", "size", "set"}
	outArgs = append(outArgs, setArgs...)
	outArgs = append(outArgs, "comment", fmt.Sprintf(`"%s"`, comment))
	if _, err := m.execNFT(outArgs...); err != nil {
		return fmt.Errorf("failed to add outbound MSS rule: %w", err)
	}

	// Inbound: SYN-ACK from destination
	inArgs := []string{"add", "rule", "ip", "filter", "forward",
		"ip", "saddr", dstIP,
		"tcp", "sport", strconv.Itoa(dstPort),
		"tcp", "flags", "syn",
		"tcp", "option", "maxseg", "size", "set"}
	inArgs = append(inArgs, setArgs...)
	inArgs = append(inArgs, "comment", fmt.Sprintf(`"%s"`, comment))
	if _, err := m.execNFT(inArgs...); err != nil {
		return fmt.Errorf("failed to add inbound MSS rule: %w", err)
	}

	return nil
}

// deleteMSSClampRules deletes TCP MSS clamping rules from filter forward chain
func (m *ForwardingManager) deleteMSSClampRules(srcPort int) error {
	output, err := m.execNFT("-j", "-a", "list", "chain", "ip", "filter", "forward")
	if err != nil {
		return nil
	}

	var ruleset NFTRuleset
	if err := json.Unmarshal(output, &ruleset); err != nil {
		return err
	}

	commentPrefix := fmt.Sprintf("%s %d", ForwardingComment, srcPort)
	for _, obj := range ruleset.NFTables {
		if obj.Rule == nil || obj.Rule.Chain != "forward" {
			continue
		}
		if !strings.HasPrefix(obj.Rule.Comment, commentPrefix) {
			continue
		}
		// Check if this rule has a mangle expression (MSS clamping)
		for _, expr := range obj.Rule.Expr {
			if _, ok := expr["mangle"]; ok {
				m.execNFT("delete", "rule", "ip", "filter", "forward", "handle", strconv.FormatInt(obj.Rule.Handle, 10))
				break
			}
		}
	}

	return nil
}

// deleteRouteMarkRules removes bypass fwmark rules for a forwarded source port.
func (m *ForwardingManager) deleteRouteMarkRules(srcPort int) error {
	commentPrefix := fmt.Sprintf("%s %d", ForwardingComment, srcPort)
	chains := []string{"prerouting", "output"}

	for _, chain := range chains {
		output, err := m.execNFT("-j", "-a", "list", "chain", "ip", "mangle", chain)
		if err != nil {
			continue
		}

		var ruleset NFTRuleset
		if err := json.Unmarshal(output, &ruleset); err != nil {
			return err
		}

		for _, obj := range ruleset.NFTables {
			if obj.Rule == nil || obj.Rule.Chain != chain {
				continue
			}
			if strings.HasPrefix(obj.Rule.Comment, commentPrefix) {
				m.execNFT("delete", "rule", "ip", "mangle", chain, "handle", strconv.FormatInt(obj.Rule.Handle, 10))
			}
		}
	}

	return nil
}

// addForwardLimitRules adds bandwidth limit rules in filter forward chain (bidirectional)
func (m *ForwardingManager) addForwardLimitRules(dstIP string, dstPort int, protocol string, comment string, limitMbps int) error {
	if limitMbps <= 0 {
		return nil // No limit needed
	}

	// Convert Mbps to kbytes/second for nftables
	// 1 Mbps = 1000 kbits/s = 125 KByte/s (using 1000-based conversion for network speeds)
	limitKbytes := (limitMbps * 1000) / 8

	// Ensure filter table and forward chain exist
	if err := m.EnsureFilterForwardSetup(); err != nil {
		return err
	}

	family, table, chain := m.managedForwardChainRef()

	// Outbound limit (to destination)
	switch protocol {
	case "tcp":
		// TCP outbound
		if _, err := m.execNFT("add", "rule", family, table, chain,
			"ip", "daddr", dstIP, "tcp", "dport", strconv.Itoa(dstPort),
			"limit", "rate", "over", strconv.Itoa(limitKbytes), "kbytes/second",
			"drop", "comment", fmt.Sprintf(`"%s"`, comment)); err != nil {
			return fmt.Errorf("failed to add TCP outbound limit: %w", err)
		}
		// TCP inbound
		if _, err := m.execNFT("add", "rule", family, table, chain,
			"ip", "saddr", dstIP, "tcp", "sport", strconv.Itoa(dstPort),
			"limit", "rate", "over", strconv.Itoa(limitKbytes), "kbytes/second",
			"drop", "comment", fmt.Sprintf(`"%s"`, comment)); err != nil {
			return fmt.Errorf("failed to add TCP inbound limit: %w", err)
		}
	case "udp":
		// UDP outbound
		if _, err := m.execNFT("add", "rule", family, table, chain,
			"ip", "daddr", dstIP, "udp", "dport", strconv.Itoa(dstPort),
			"limit", "rate", "over", strconv.Itoa(limitKbytes), "kbytes/second",
			"drop", "comment", fmt.Sprintf(`"%s"`, comment)); err != nil {
			return fmt.Errorf("failed to add UDP outbound limit: %w", err)
		}
		// UDP inbound
		if _, err := m.execNFT("add", "rule", family, table, chain,
			"ip", "saddr", dstIP, "udp", "sport", strconv.Itoa(dstPort),
			"limit", "rate", "over", strconv.Itoa(limitKbytes), "kbytes/second",
			"drop", "comment", fmt.Sprintf(`"%s"`, comment)); err != nil {
			return fmt.Errorf("failed to add UDP inbound limit: %w", err)
		}
	default: // "both"
		// Both TCP/UDP outbound
		if _, err := m.execNFT("add", "rule", family, table, chain,
			"ip", "daddr", dstIP, "meta", "l4proto", "{", "tcp,", "udp", "}",
			"th", "dport", strconv.Itoa(dstPort),
			"limit", "rate", "over", strconv.Itoa(limitKbytes), "kbytes/second",
			"drop", "comment", fmt.Sprintf(`"%s"`, comment)); err != nil {
			return fmt.Errorf("failed to add outbound limit: %w", err)
		}
		// Both TCP/UDP inbound
		if _, err := m.execNFT("add", "rule", family, table, chain,
			"ip", "saddr", dstIP, "meta", "l4proto", "{", "tcp,", "udp", "}",
			"th", "sport", strconv.Itoa(dstPort),
			"limit", "rate", "over", strconv.Itoa(limitKbytes), "kbytes/second",
			"drop", "comment", fmt.Sprintf(`"%s"`, comment)); err != nil {
			return fmt.Errorf("failed to add inbound limit: %w", err)
		}
	}

	return nil
}

// addForwardAcceptRules adds explicit allow rules in the active managed forward
// chain so DNAT traffic still works when the user's main forward policy is drop.
func (m *ForwardingManager) addForwardAcceptRules(dstIP string, dstPort int, protocol string, comment string) error {
	if err := m.EnsureFilterForwardSetup(); err != nil {
		return err
	}

	family, table, chain := m.managedForwardChainRef()

	switch protocol {
	case "tcp":
		if _, err := m.execNFT("add", "rule", family, table, chain,
			"ip", "daddr", dstIP, "tcp", "dport", strconv.Itoa(dstPort),
			"accept", "comment", fmt.Sprintf(`"%s"`, comment)); err != nil {
			return fmt.Errorf("failed to add TCP outbound accept: %w", err)
		}
		if _, err := m.execNFT("add", "rule", family, table, chain,
			"ip", "saddr", dstIP, "tcp", "sport", strconv.Itoa(dstPort),
			"ct", "state", "established,related",
			"accept", "comment", fmt.Sprintf(`"%s"`, comment)); err != nil {
			return fmt.Errorf("failed to add TCP reply accept: %w", err)
		}
	case "udp":
		if _, err := m.execNFT("add", "rule", family, table, chain,
			"ip", "daddr", dstIP, "udp", "dport", strconv.Itoa(dstPort),
			"accept", "comment", fmt.Sprintf(`"%s"`, comment)); err != nil {
			return fmt.Errorf("failed to add UDP outbound accept: %w", err)
		}
		if _, err := m.execNFT("add", "rule", family, table, chain,
			"ip", "saddr", dstIP, "udp", "sport", strconv.Itoa(dstPort),
			"ct", "state", "established,related",
			"accept", "comment", fmt.Sprintf(`"%s"`, comment)); err != nil {
			return fmt.Errorf("failed to add UDP reply accept: %w", err)
		}
	default: // "both"
		if _, err := m.execNFT("add", "rule", family, table, chain,
			"ip", "daddr", dstIP, "meta", "l4proto", "{", "tcp,", "udp", "}",
			"th", "dport", strconv.Itoa(dstPort),
			"accept", "comment", fmt.Sprintf(`"%s"`, comment)); err != nil {
			return fmt.Errorf("failed to add outbound accept: %w", err)
		}
		if _, err := m.execNFT("add", "rule", family, table, chain,
			"ip", "saddr", dstIP, "meta", "l4proto", "{", "tcp,", "udp", "}",
			"th", "sport", strconv.Itoa(dstPort),
			"ct", "state", "established,related",
			"accept", "comment", fmt.Sprintf(`"%s"`, comment)); err != nil {
			return fmt.Errorf("failed to add reply accept: %w", err)
		}
	}

	return nil
}

func ruleHasExpr(rule *NFTRule, exprName string) bool {
	for _, expr := range rule.Expr {
		if _, ok := expr[exprName]; ok {
			return true
		}
	}
	return false
}

func (m *ForwardingManager) deleteRulesByCommentPrefix(family, table, chain, commentPrefix string, shouldDelete func(*NFTRule) bool) error {
	output, err := m.execNFT("-j", "-a", "list", "chain", family, table, chain)
	if err != nil {
		return nil
	}

	var ruleset NFTRuleset
	if err := json.Unmarshal(output, &ruleset); err != nil {
		return err
	}

	for _, obj := range ruleset.NFTables {
		if obj.Rule == nil || obj.Rule.Chain != chain {
			continue
		}
		if !strings.HasPrefix(obj.Rule.Comment, commentPrefix) {
			continue
		}
		if !shouldDelete(obj.Rule) {
			continue
		}
		m.execNFT("delete", "rule", family, table, chain, "handle", strconv.FormatInt(obj.Rule.Handle, 10))
	}

	return nil
}

// deleteForwardFilterRules deletes managed forward-chain limit and access rules.
// It also cleans up legacy ip filter limit rules left by older releases.
func (m *ForwardingManager) deleteForwardFilterRules(srcPort int) error {
	commentPrefix := fmt.Sprintf("%s %d", ForwardingComment, srcPort)

	family, table, chain := m.managedForwardChainRef()
	if err := m.deleteRulesByCommentPrefix(family, table, chain, commentPrefix, func(rule *NFTRule) bool {
		return true
	}); err != nil {
		return err
	}

	legacyFamily, legacyTable, legacyChain := m.legacyForwardChainRef()
	if family != legacyFamily || table != legacyTable {
		if err := m.deleteRulesByCommentPrefix(legacyFamily, legacyTable, legacyChain, commentPrefix, func(rule *NFTRule) bool {
			return ruleHasExpr(rule, "limit")
		}); err != nil {
			return err
		}
	}

	return nil
}

// DeleteForwardingRule deletes a forwarding rule by ID
func (m *ForwardingManager) DeleteForwardingRule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Parse source port from ID
	srcPort, err := m.parseSrcPortFromID(id)
	if err != nil {
		return err
	}

	// Check if it's a disabled rule
	disabledRules, _ := m.loadDisabledRules()
	for i, r := range disabledRules {
		if r.SrcPort == srcPort {
			// Remove from disabled rules
			disabledRules = append(disabledRules[:i], disabledRules[i+1:]...)
			return m.saveDisabledRules(disabledRules)
		}
	}

	// It's an enabled rule, delete from nftables
	if err := m.deleteDNATRuleBySrcPort(srcPort); err != nil {
		return fmt.Errorf("failed to delete DNAT rule: %w", err)
	}

	if err := m.deleteMasqueradeRuleBySrcPort(srcPort); err != nil {
		// Log warning but don't fail
		fmt.Printf("Warning: failed to delete MASQUERADE rule: %v\n", err)
	}

	if err := m.deleteOutputDNATRuleBySrcPort(srcPort); err != nil {
		// Log warning but don't fail
		fmt.Printf("Warning: failed to delete output DNAT rule: %v\n", err)
	}

	if err := m.deleteRouteMarkRules(srcPort); err != nil {
		fmt.Printf("Warning: failed to delete bypass mark rules: %v\n", err)
	}

	if err := m.deleteForwardFilterRules(srcPort); err != nil {
		// Log warning but don't fail
		fmt.Printf("Warning: failed to delete forward limit rules: %v\n", err)
	}

	if err := m.deleteMSSClampRules(srcPort); err != nil {
		fmt.Printf("Warning: failed to delete MSS clamp rules: %v\n", err)
	}

	return nil
}

// EditForwardingRule modifies an existing forwarding rule
func (m *ForwardingManager) EditForwardingRule(id string, dstIP string, dstPort int, protocol string, comment string, limitMbps int, mssMode string, sourceNATMode string, snatAddress string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Parse source port from ID
	srcPort, err := m.parseSrcPortFromID(id)
	if err != nil {
		return err
	}

	// Validate inputs
	if dstPort < 1 || dstPort > 65535 {
		return fmt.Errorf("invalid destination port: %d", dstPort)
	}
	if !isValidIPv4(dstIP) {
		return fmt.Errorf("invalid destination IP: %s", dstIP)
	}
	if protocol != "tcp" && protocol != "udp" && protocol != "both" {
		return fmt.Errorf("invalid protocol: %s", protocol)
	}
	if limitMbps < 0 {
		return fmt.Errorf("invalid limit: %d (must be >= 0)", limitMbps)
	}
	mssMode = normalizeMSSMode(mssMode)
	if mssMode == "" {
		return fmt.Errorf("invalid MSS mode: %s", mssMode)
	}
	sourceNATMode = normalizeSourceNATMode(sourceNATMode)
	if sourceNATMode == "" {
		return fmt.Errorf("invalid source NAT mode: %s", sourceNATMode)
	}
	if sourceNATMode == SourceNATModeSNAT {
		if !isValidIPv4(snatAddress) {
			return fmt.Errorf("invalid SNAT address: %s", snatAddress)
		}
	} else {
		snatAddress = ""
	}

	comment = sanitizeComment(comment)

	// Check if it's a disabled rule
	disabledRules, _ := m.loadDisabledRules()
	for i, r := range disabledRules {
		if r.SrcPort == srcPort {
			// Update the disabled rule
			disabledRules[i].DstIP = dstIP
			disabledRules[i].DstPort = dstPort
			disabledRules[i].Protocol = protocol
			disabledRules[i].Comment = comment
			disabledRules[i].LimitMbps = limitMbps
			disabledRules[i].MSSMode = mssMode
			disabledRules[i].SourceNATMode = sourceNATMode
			disabledRules[i].SNATAddress = snatAddress
			return m.saveDisabledRules(disabledRules)
		}
	}

	// It's an enabled rule - delete and recreate
	// Delete existing rules
	m.deleteDNATRuleBySrcPort(srcPort)
	m.deleteMasqueradeRuleBySrcPort(srcPort)
	m.deleteOutputDNATRuleBySrcPort(srcPort)
	m.deleteRouteMarkRules(srcPort)
	m.deleteForwardFilterRules(srcPort)
	m.deleteMSSClampRules(srcPort)

	// Build comment string
	fullComment := fmt.Sprintf("%s %d", ForwardingComment, srcPort)
	if comment != "" {
		fullComment = fmt.Sprintf("%s %s", fullComment, comment)
	}

	// Add new rules
	if err := m.addDNATRule(srcPort, dstIP, dstPort, protocol, fullComment, limitMbps); err != nil {
		return fmt.Errorf("failed to add DNAT rule: %w", err)
	}

	if err := m.addSourceNATRule(dstIP, dstPort, protocol, fullComment, sourceNATMode, snatAddress); err != nil {
		m.deleteDNATRuleBySrcPort(srcPort)
		return fmt.Errorf("failed to add source NAT rule: %w", err)
	}

	if err := m.addOutputDNATRule(srcPort, dstIP, dstPort, protocol, fullComment, limitMbps); err != nil {
		m.deleteDNATRuleBySrcPort(srcPort)
		m.deleteMasqueradeRuleBySrcPort(srcPort)
		return fmt.Errorf("failed to add output DNAT rule: %w", err)
	}

	if err := m.addRouteMarkRules(srcPort, protocol, fullComment); err != nil {
		m.deleteDNATRuleBySrcPort(srcPort)
		m.deleteMasqueradeRuleBySrcPort(srcPort)
		m.deleteOutputDNATRuleBySrcPort(srcPort)
		return fmt.Errorf("failed to add bypass mark rules: %w", err)
	}

	if err := m.addForwardLimitRules(dstIP, dstPort, protocol, fullComment, limitMbps); err != nil {
		m.deleteDNATRuleBySrcPort(srcPort)
		m.deleteMasqueradeRuleBySrcPort(srcPort)
		m.deleteOutputDNATRuleBySrcPort(srcPort)
		m.deleteRouteMarkRules(srcPort)
		return fmt.Errorf("failed to add forward limit rules: %w", err)
	}

	if err := m.addForwardAcceptRules(dstIP, dstPort, protocol, fullComment); err != nil {
		m.deleteDNATRuleBySrcPort(srcPort)
		m.deleteMasqueradeRuleBySrcPort(srcPort)
		m.deleteOutputDNATRuleBySrcPort(srcPort)
		m.deleteRouteMarkRules(srcPort)
		m.deleteForwardFilterRules(srcPort)
		return fmt.Errorf("failed to add forward accept rules: %w", err)
	}

	if err := m.addMSSClampRule(dstIP, dstPort, protocol, fullComment, mssMode); err != nil {
		m.deleteDNATRuleBySrcPort(srcPort)
		m.deleteMasqueradeRuleBySrcPort(srcPort)
		m.deleteOutputDNATRuleBySrcPort(srcPort)
		m.deleteRouteMarkRules(srcPort)
		m.deleteForwardFilterRules(srcPort)
		return fmt.Errorf("failed to add MSS clamp rule: %w", err)
	}

	return nil
}

// EnableForwardingRule enables a disabled forwarding rule
func (m *ForwardingManager) EnableForwardingRule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	srcPort, err := m.parseSrcPortFromID(id)
	if err != nil {
		return err
	}

	// Find the disabled rule
	disabledRules, err := m.loadDisabledRules()
	if err != nil {
		return fmt.Errorf("failed to load disabled rules: %w", err)
	}

	var rule *ForwardingRule
	var idx int
	for i, r := range disabledRules {
		if r.SrcPort == srcPort {
			rule = &disabledRules[i]
			idx = i
			break
		}
	}

	if rule == nil {
		return fmt.Errorf("disabled rule not found: %s", id)
	}

	// Ensure nat table exists
	if err := m.EnsureNatSetup(); err != nil {
		return err
	}

	// Build comment string
	fullComment := fmt.Sprintf("%s %d", ForwardingComment, rule.SrcPort)
	if rule.Comment != "" {
		fullComment = fmt.Sprintf("%s %s", fullComment, rule.Comment)
	}

	// Create nftables rules
	if err := m.addDNATRule(rule.SrcPort, rule.DstIP, rule.DstPort, rule.Protocol, fullComment, rule.LimitMbps); err != nil {
		return fmt.Errorf("failed to add DNAT rule: %w", err)
	}

	if err := m.addSourceNATRule(rule.DstIP, rule.DstPort, rule.Protocol, fullComment, rule.SourceNATMode, rule.SNATAddress); err != nil {
		m.deleteDNATRuleBySrcPort(rule.SrcPort)
		return fmt.Errorf("failed to add source NAT rule: %w", err)
	}

	if err := m.addOutputDNATRule(rule.SrcPort, rule.DstIP, rule.DstPort, rule.Protocol, fullComment, rule.LimitMbps); err != nil {
		m.deleteDNATRuleBySrcPort(rule.SrcPort)
		m.deleteMasqueradeRuleBySrcPort(rule.SrcPort)
		return fmt.Errorf("failed to add output DNAT rule: %w", err)
	}

	if err := m.addRouteMarkRules(rule.SrcPort, rule.Protocol, fullComment); err != nil {
		m.deleteDNATRuleBySrcPort(rule.SrcPort)
		m.deleteMasqueradeRuleBySrcPort(rule.SrcPort)
		m.deleteOutputDNATRuleBySrcPort(rule.SrcPort)
		return fmt.Errorf("failed to add bypass mark rules: %w", err)
	}

	if err := m.addForwardLimitRules(rule.DstIP, rule.DstPort, rule.Protocol, fullComment, rule.LimitMbps); err != nil {
		m.deleteDNATRuleBySrcPort(rule.SrcPort)
		m.deleteMasqueradeRuleBySrcPort(rule.SrcPort)
		m.deleteOutputDNATRuleBySrcPort(rule.SrcPort)
		m.deleteRouteMarkRules(rule.SrcPort)
		return fmt.Errorf("failed to add forward limit rules: %w", err)
	}

	if err := m.addForwardAcceptRules(rule.DstIP, rule.DstPort, rule.Protocol, fullComment); err != nil {
		m.deleteDNATRuleBySrcPort(rule.SrcPort)
		m.deleteMasqueradeRuleBySrcPort(rule.SrcPort)
		m.deleteOutputDNATRuleBySrcPort(rule.SrcPort)
		m.deleteRouteMarkRules(rule.SrcPort)
		m.deleteForwardFilterRules(rule.SrcPort)
		return fmt.Errorf("failed to add forward accept rules: %w", err)
	}

	if err := m.addMSSClampRule(rule.DstIP, rule.DstPort, rule.Protocol, fullComment, rule.MSSMode); err != nil {
		m.deleteDNATRuleBySrcPort(rule.SrcPort)
		m.deleteMasqueradeRuleBySrcPort(rule.SrcPort)
		m.deleteOutputDNATRuleBySrcPort(rule.SrcPort)
		m.deleteRouteMarkRules(rule.SrcPort)
		m.deleteForwardFilterRules(rule.SrcPort)
		return fmt.Errorf("failed to add MSS clamp rule: %w", err)
	}

	// Remove from disabled rules
	disabledRules = append(disabledRules[:idx], disabledRules[idx+1:]...)
	return m.saveDisabledRules(disabledRules)
}

// DisableForwardingRule disables an enabled forwarding rule
func (m *ForwardingManager) DisableForwardingRule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	srcPort, err := m.parseSrcPortFromID(id)
	if err != nil {
		return err
	}

	// Get current enabled rules
	preOutput, _ := m.execNFT("-j", "-a", "list", "chain", "ip", "nat", "prerouting")
	postOutput, _ := m.execNFT("-j", "-a", "list", "chain", "ip", "nat", "postrouting")
	enabledRules, err := m.parseForwardingRules(preOutput, postOutput)
	if err != nil {
		return err
	}

	limitMap := m.extractLimitsFromForwardChain()
	mssModeMap := m.extractMSSModesFromForwardChain()
	sourceNATMap := m.extractSourceNATFromPostrouting(postOutput)
	for i := range enabledRules {
		if limit, ok := limitMap[enabledRules[i].SrcPort]; ok {
			enabledRules[i].LimitMbps = limit
		}
		if mssMode, ok := mssModeMap[enabledRules[i].SrcPort]; ok {
			enabledRules[i].MSSMode = mssMode
		} else if enabledRules[i].Managed {
			enabledRules[i].MSSMode = MSSModeFixed1452
		}
		if natInfo, ok := sourceNATMap[enabledRules[i].SrcPort]; ok {
			enabledRules[i].SourceNATMode = natInfo.Mode
			enabledRules[i].SNATAddress = natInfo.Address
		} else if enabledRules[i].Managed {
			enabledRules[i].SourceNATMode = SourceNATModeMasquerade
		}
	}

	// Find the rule to disable
	var rule *ForwardingRule
	for i, r := range enabledRules {
		if r.SrcPort == srcPort {
			rule = &enabledRules[i]
			break
		}
	}

	if rule == nil {
		return fmt.Errorf("enabled rule not found: %s", id)
	}

	// Delete from nftables
	if err := m.deleteDNATRuleBySrcPort(srcPort); err != nil {
		return fmt.Errorf("failed to delete DNAT rule: %w", err)
	}
	m.deleteMasqueradeRuleBySrcPort(srcPort) // Ignore errors
	m.deleteOutputDNATRuleBySrcPort(srcPort) // Ignore errors
	m.deleteRouteMarkRules(srcPort)          // Ignore errors
	m.deleteForwardFilterRules(srcPort)      // Ignore errors
	m.deleteMSSClampRules(srcPort)           // Ignore errors

	// Save to disabled rules
	disabledRules, _ := m.loadDisabledRules()
	disabledRule := ForwardingRule{
		ID:            rule.ID,
		SrcPort:       rule.SrcPort,
		DstIP:         rule.DstIP,
		DstPort:       rule.DstPort,
		Protocol:      rule.Protocol,
		Enabled:       false,
		Comment:       rule.Comment,
		LimitMbps:     rule.LimitMbps,
		MSSMode:       rule.MSSMode,
		SourceNATMode: rule.SourceNATMode,
		SNATAddress:   rule.SNATAddress,
	}
	disabledRules = append(disabledRules, disabledRule)
	return m.saveDisabledRules(disabledRules)
}

// Helper functions

func (m *ForwardingManager) parseSrcPortFromID(id string) (int, error) {
	if !strings.HasPrefix(id, "fwd_") {
		return 0, fmt.Errorf("invalid forwarding rule ID: %s", id)
	}
	portStr := strings.TrimPrefix(id, "fwd_")
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 0, fmt.Errorf("invalid forwarding rule ID: %s", id)
	}
	return port, nil
}

func (m *ForwardingManager) deleteDNATRuleBySrcPort(srcPort int) error {
	output, err := m.execNFT("-j", "-a", "list", "chain", "ip", "nat", "prerouting")
	if err != nil {
		return err
	}

	var ruleset NFTRuleset
	if err := json.Unmarshal(output, &ruleset); err != nil {
		return err
	}

	for _, obj := range ruleset.NFTables {
		if obj.Rule == nil || obj.Rule.Chain != "prerouting" {
			continue
		}
		if strings.HasPrefix(obj.Rule.Comment, fmt.Sprintf("%s %d", ForwardingComment, srcPort)) {
			_, err := m.execNFT("delete", "rule", "ip", "nat", "prerouting", "handle", strconv.FormatInt(obj.Rule.Handle, 10))
			return err
		}
	}

	return errors.New("DNAT rule not found")
}

func (m *ForwardingManager) deleteMasqueradeRuleBySrcPort(srcPort int) error {
	output, err := m.execNFT("-j", "-a", "list", "chain", "ip", "nat", "postrouting")
	if err != nil {
		return err
	}

	var ruleset NFTRuleset
	if err := json.Unmarshal(output, &ruleset); err != nil {
		return err
	}

	for _, obj := range ruleset.NFTables {
		if obj.Rule == nil || obj.Rule.Chain != "postrouting" {
			continue
		}
		if strings.HasPrefix(obj.Rule.Comment, fmt.Sprintf("%s %d", ForwardingComment, srcPort)) {
			_, err := m.execNFT("delete", "rule", "ip", "nat", "postrouting", "handle", strconv.FormatInt(obj.Rule.Handle, 10))
			return err
		}
	}

	return errors.New("MASQUERADE rule not found")
}

func (m *ForwardingManager) deleteOutputDNATRuleBySrcPort(srcPort int) error {
	output, err := m.execNFT("-j", "-a", "list", "chain", "ip", "nat", "output")
	if err != nil {
		return err
	}

	var ruleset NFTRuleset
	if err := json.Unmarshal(output, &ruleset); err != nil {
		return err
	}

	for _, obj := range ruleset.NFTables {
		if obj.Rule == nil || obj.Rule.Chain != "output" {
			continue
		}
		if strings.HasPrefix(obj.Rule.Comment, fmt.Sprintf("%s %d", ForwardingComment, srcPort)) {
			_, err := m.execNFT("delete", "rule", "ip", "nat", "output", "handle", strconv.FormatInt(obj.Rule.Handle, 10))
			return err
		}
	}

	return errors.New("output DNAT rule not found")
}

func (m *ForwardingManager) loadDisabledRules() ([]ForwardingRule, error) {
	data, err := os.ReadFile(m.disabledForwardsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []ForwardingRule{}, nil
		}
		return nil, err
	}

	var file DisabledForwardsFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, err
	}

	// Mark all as disabled
	for i := range file.Rules {
		file.Rules[i].Enabled = false
		file.Rules[i].ID = fmt.Sprintf("fwd_%d", file.Rules[i].SrcPort)
		if normalizeMSSMode(file.Rules[i].MSSMode) == "" {
			file.Rules[i].MSSMode = MSSModeFixed1452
		} else {
			file.Rules[i].MSSMode = normalizeMSSMode(file.Rules[i].MSSMode)
		}
	}

	return file.Rules, nil
}

func (m *ForwardingManager) saveDisabledRules(rules []ForwardingRule) error {
	// Ensure directory exists
	dir := filepath.Dir(m.disabledForwardsPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	file := DisabledForwardsFile{Rules: rules}
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(m.disabledForwardsPath, data, 0644)
}

// isValidIPv4 validates an IPv4 address
func isValidIPv4(ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	// Ensure it's IPv4 (not IPv6)
	return parsed.To4() != nil
}

// sanitizeForwardingComment removes invalid characters from comment
func sanitizeForwardingComment(s string) string {
	re := regexp.MustCompile(`[^a-zA-Z0-9\s\-_.\u4e00-\u9fff]`)
	s = re.ReplaceAllString(s, "")
	if len(s) > 100 {
		s = s[:100]
	}
	return s
}
