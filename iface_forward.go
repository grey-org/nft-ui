package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// IfaceForwardingManager handles interface-based forwarding (dual-NIC DNAT)
type IfaceForwardingManager struct {
	mu           sync.RWMutex
	binary       string
	cfg          *Config
	logger       *log.Logger
	disabledPath string
}

// NewIfaceForwardingManager creates a new IfaceForwardingManager
func NewIfaceForwardingManager(cfg *Config, logger *log.Logger) *IfaceForwardingManager {
	path := cfg.DisabledIfaceForwardsPath
	if path == "" {
		path = "/var/lib/nft-ui/disabled-iface-forwards.json"
	}
	return &IfaceForwardingManager{
		binary:       cfg.NFTBinary,
		cfg:          cfg,
		logger:       logger,
		disabledPath: path,
	}
}

func (m *IfaceForwardingManager) execNFT(args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, m.binary, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("nft %s: %w (output: %s)", strings.Join(args, " "), err, string(output))
	}
	return output, nil
}

// EnsureTableSetup creates the inet nft-ui-iface-fwd table and prerouting chain if missing.
func (m *IfaceForwardingManager) EnsureTableSetup() error {
	// create table (idempotent)
	if _, err := m.execNFT("add", "table", "inet", IfaceForwardTableName); err != nil {
		return fmt.Errorf("create iface-fwd table: %w", err)
	}
	// create prerouting chain (idempotent)
	if _, err := m.execNFT("add", "chain", "inet", IfaceForwardTableName, IfaceForwardChainName,
		"{ type nat hook prerouting priority dstnat; policy accept; }"); err != nil {
		return fmt.Errorf("create iface-fwd chain: %w", err)
	}
	return nil
}

// ListRules returns all interface forwarding rules (enabled from nftables + disabled from file).
func (m *IfaceForwardingManager) ListRules() ([]IfaceForwardRule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Try to read live rules; table may not exist yet
	output, err := m.execNFT("-j", "-a", "list", "chain", "inet", IfaceForwardTableName, IfaceForwardChainName)
	var enabledRules []IfaceForwardRule
	if err == nil {
		enabledRules, err = m.parseRules(output)
		if err != nil {
			return nil, err
		}
	}
	// if chain doesn't exist yet, enabledRules stays nil/empty

	disabledRules, err := m.loadDisabledRules()
	if err != nil {
		disabledRules = []IfaceForwardRule{}
	}

	// Build a set of IDs already present as enabled (so we don't double-list)
	enabledIDs := make(map[string]bool)
	for _, r := range enabledRules {
		enabledIDs[r.ID] = true
	}

	result := make([]IfaceForwardRule, 0, len(enabledRules)+len(disabledRules))
	result = append(result, enabledRules...)
	for _, r := range disabledRules {
		if !enabledIDs[r.ID] {
			result = append(result, r)
		}
	}
	return result, nil
}

// AddRule validates and adds a new interface forwarding rule.
func (m *IfaceForwardingManager) AddRule(req AddIfaceForwardRequest) (*IfaceForwardRule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := validateIfaceForwardFields(req.IifName, req.AddrFamily, req.NatAddrFamily, req.DstAddr, req.NatTo, req.Protocol); err != nil {
		return nil, err
	}

	id := generateIfaceForwardID()
	comment := sanitizeIfaceComment(req.Comment)

	rule := IfaceForwardRule{
		ID:            id,
		IifName:       req.IifName,
		AddrFamily:    req.AddrFamily,
		NatAddrFamily: req.NatAddrFamily,
		DstAddr:       req.DstAddr,
		NatTo:         req.NatTo,
		Protocol:      req.Protocol,
		Comment:       comment,
		Enabled:       true,
		Managed:       true,
	}

	if err := m.EnsureTableSetup(); err != nil {
		return nil, err
	}

	stmt := m.buildRuleStatement(rule)
	if _, err := m.execNFT(strings.Fields(stmt)...); err != nil {
		return nil, fmt.Errorf("add iface-fwd rule: %w", err)
	}

	// Fetch back to get the handle
	output, err := m.execNFT("-j", "-a", "list", "chain", "inet", IfaceForwardTableName, IfaceForwardChainName)
	if err == nil {
		rules, _ := m.parseRules(output)
		for _, r := range rules {
			if r.ID == id {
				rule.Handle = r.Handle
				break
			}
		}
	}

	return &rule, nil
}

// EditRule replaces an existing rule (delete + re-add, preserving ID).
func (m *IfaceForwardingManager) EditRule(id string, req EditIfaceForwardRequest) (*IfaceForwardRule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := validateIfaceForwardFields(req.IifName, req.AddrFamily, req.NatAddrFamily, req.DstAddr, req.NatTo, req.Protocol); err != nil {
		return nil, err
	}
	comment := sanitizeIfaceComment(req.Comment)

	// Check enabled rules first
	output, err := m.execNFT("-j", "-a", "list", "chain", "inet", IfaceForwardTableName, IfaceForwardChainName)
	if err != nil {
		// Check if it's disabled
		return m.editDisabledRule(id, req, comment)
	}

	rules, err := m.parseRules(output)
	if err != nil {
		return nil, err
	}

	for _, r := range rules {
		if r.ID == id {
			// Delete old rule by handle
			if _, err := m.execNFT("delete", "rule", "inet", IfaceForwardTableName, IfaceForwardChainName,
				"handle", fmt.Sprintf("%d", r.Handle)); err != nil {
				return nil, fmt.Errorf("delete old iface-fwd rule: %w", err)
			}
			// Add new rule with same ID
			newRule := IfaceForwardRule{
				ID:            id,
				IifName:       req.IifName,
				AddrFamily:    req.AddrFamily,
				NatAddrFamily: req.NatAddrFamily,
				DstAddr:       req.DstAddr,
				NatTo:         req.NatTo,
				Protocol:      req.Protocol,
				Comment:       comment,
				Enabled:       true,
				Managed:       true,
			}
			stmt := m.buildRuleStatement(newRule)
			if _, err := m.execNFT(strings.Fields(stmt)...); err != nil {
				return nil, fmt.Errorf("re-add iface-fwd rule: %w", err)
			}
			// Fetch handle
			if out2, err2 := m.execNFT("-j", "-a", "list", "chain", "inet", IfaceForwardTableName, IfaceForwardChainName); err2 == nil {
				if rules2, err3 := m.parseRules(out2); err3 == nil {
					for _, r2 := range rules2 {
						if r2.ID == id {
							newRule.Handle = r2.Handle
							break
						}
					}
				}
			}
			return &newRule, nil
		}
	}

	// Try disabled rules
	return m.editDisabledRule(id, req, comment)
}

func (m *IfaceForwardingManager) editDisabledRule(id string, req EditIfaceForwardRequest, comment string) (*IfaceForwardRule, error) {
	disabledRules, err := m.loadDisabledRules()
	if err != nil {
		return nil, fmt.Errorf("rule %s not found", id)
	}
	for i, r := range disabledRules {
		if r.ID == id {
			disabledRules[i].IifName = req.IifName
			disabledRules[i].AddrFamily = req.AddrFamily
			disabledRules[i].NatAddrFamily = req.NatAddrFamily
			disabledRules[i].DstAddr = req.DstAddr
			disabledRules[i].NatTo = req.NatTo
			disabledRules[i].Protocol = req.Protocol
			disabledRules[i].Comment = comment
			if err := m.saveDisabledRules(disabledRules); err != nil {
				return nil, err
			}
			return &disabledRules[i], nil
		}
	}
	return nil, fmt.Errorf("rule %s not found", id)
}

// DeleteRule removes a rule from nftables or from the disabled JSON file.
func (m *IfaceForwardingManager) DeleteRule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Try enabled rules first
	output, err := m.execNFT("-j", "-a", "list", "chain", "inet", IfaceForwardTableName, IfaceForwardChainName)
	if err == nil {
		rules, _ := m.parseRules(output)
		for _, r := range rules {
			if r.ID == id {
				_, err := m.execNFT("delete", "rule", "inet", IfaceForwardTableName, IfaceForwardChainName,
					"handle", fmt.Sprintf("%d", r.Handle))
				return err
			}
		}
	}

	// Try disabled rules
	disabledRules, err := m.loadDisabledRules()
	if err != nil {
		return fmt.Errorf("rule %s not found", id)
	}
	for i, r := range disabledRules {
		if r.ID == id {
			disabledRules = append(disabledRules[:i], disabledRules[i+1:]...)
			return m.saveDisabledRules(disabledRules)
		}
	}
	return fmt.Errorf("rule %s not found", id)
}

// EnableRule moves a disabled rule from JSON into nftables.
func (m *IfaceForwardingManager) EnableRule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	disabledRules, err := m.loadDisabledRules()
	if err != nil {
		return fmt.Errorf("failed to load disabled rules: %w", err)
	}

	idx := -1
	for i, r := range disabledRules {
		if r.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return fmt.Errorf("rule %s not found in disabled list", id)
	}

	rule := disabledRules[idx]
	rule.Enabled = true

	if err := m.EnsureTableSetup(); err != nil {
		return err
	}

	stmt := m.buildRuleStatement(rule)
	if _, err := m.execNFT(strings.Fields(stmt)...); err != nil {
		return fmt.Errorf("re-enable iface-fwd rule: %w", err)
	}

	// Remove from disabled list
	disabledRules = append(disabledRules[:idx], disabledRules[idx+1:]...)
	return m.saveDisabledRules(disabledRules)
}

// DisableRule removes a rule from nftables and saves it to the disabled JSON file.
func (m *IfaceForwardingManager) DisableRule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	output, err := m.execNFT("-j", "-a", "list", "chain", "inet", IfaceForwardTableName, IfaceForwardChainName)
	if err != nil {
		return fmt.Errorf("failed to list iface-fwd rules: %w", err)
	}

	rules, err := m.parseRules(output)
	if err != nil {
		return err
	}

	var found *IfaceForwardRule
	for i := range rules {
		if rules[i].ID == id {
			found = &rules[i]
			break
		}
	}
	if found == nil {
		return fmt.Errorf("rule %s not found in nftables", id)
	}

	// Delete from nftables
	if _, err := m.execNFT("delete", "rule", "inet", IfaceForwardTableName, IfaceForwardChainName,
		"handle", fmt.Sprintf("%d", found.Handle)); err != nil {
		return fmt.Errorf("delete iface-fwd rule: %w", err)
	}

	// Save to disabled file
	disabledRule := *found
	disabledRule.Enabled = false
	disabledRules, _ := m.loadDisabledRules()
	disabledRules = append(disabledRules, disabledRule)
	return m.saveDisabledRules(disabledRules)
}

// buildRuleStatement produces an "add rule ..." nft command for the given rule.
func (m *IfaceForwardingManager) buildRuleStatement(rule IfaceForwardRule) string {
	nftComment := IfaceForwardComment + " " + rule.ID
	if rule.Comment != "" {
		nftComment += " " + rule.Comment
	}

	natFamily := rule.NatAddrFamily
	if natFamily == "" {
		natFamily = rule.AddrFamily
	}

	var parts []string
	parts = append(parts, "add", "rule", "inet", IfaceForwardTableName, IfaceForwardChainName)
	parts = append(parts, "iifname", fmt.Sprintf(`"%s"`, rule.IifName))
	parts = append(parts, rule.AddrFamily, "daddr", rule.DstAddr)
	if rule.Protocol != "all" {
		parts = append(parts, "meta", "l4proto", rule.Protocol)
	}
	parts = append(parts, "dnat", natFamily, "to", rule.NatTo)
	parts = append(parts, "comment", fmt.Sprintf(`"%s"`, nftComment))
	return strings.Join(parts, " ")
}

// parseRules extracts IfaceForwardRule entries from nft -j -a list chain output.
func (m *IfaceForwardingManager) parseRules(jsonData []byte) ([]IfaceForwardRule, error) {
	var ruleset NFTRuleset
	if err := json.Unmarshal(jsonData, &ruleset); err != nil {
		return nil, fmt.Errorf("failed to parse nft JSON: %w", err)
	}

	var rules []IfaceForwardRule
	for _, obj := range ruleset.NFTables {
		if obj.Rule == nil {
			continue
		}
		r := obj.Rule
		if r.Table != IfaceForwardTableName || r.Chain != IfaceForwardChainName {
			continue
		}
		if r.Comment == "" || !strings.HasPrefix(r.Comment, IfaceForwardComment+" ") {
			continue
		}

		id := m.extractIDFromComment(r.Comment)
		if id == "" {
			continue
		}
		userComment := m.extractUserCommentFromComment(r.Comment, id)

		rule := IfaceForwardRule{
			ID:      id,
			Comment: userComment,
			Handle:  r.Handle,
			Enabled: true,
			Managed: true,
		}
		// Parse rule expressions to recover iifname, addr_family, daddr, nat_to, protocol
		m.fillRuleFromExpr(&rule, r.Expr)
		rules = append(rules, rule)
	}
	return rules, nil
}

// fillRuleFromExpr reconstructs IfaceForwardRule fields from parsed nft JSON expressions.
func (m *IfaceForwardingManager) fillRuleFromExpr(rule *IfaceForwardRule, expr []map[string]interface{}) {
	rule.Protocol = "all"
	for _, e := range expr {
		// iifname match
		if match, ok := e["match"].(map[string]interface{}); ok {
			if left, ok := match["left"].(map[string]interface{}); ok {
				if meta, ok := left["meta"].(map[string]interface{}); ok {
					if key, _ := meta["key"].(string); key == "iifname" {
						if right, ok := match["right"].(string); ok {
							rule.IifName = right
						}
					}
				}
				// addr family + daddr
				if payload, ok := left["payload"].(map[string]interface{}); ok {
					field, _ := payload["field"].(string)
					protocol, _ := payload["protocol"].(string)
					if field == "daddr" {
						if protocol == "ip" || protocol == "ip6" {
							rule.AddrFamily = protocol
						}
						if right, ok := match["right"].(string); ok {
							rule.DstAddr = right
						}
					}
				}
			}
			// meta l4proto -> protocol
			if left, ok := match["left"].(map[string]interface{}); ok {
				if meta, ok := left["meta"].(map[string]interface{}); ok {
					if key, _ := meta["key"].(string); key == "l4proto" {
						if right, ok := match["right"].(string); ok {
							rule.Protocol = right
						}
					}
				}
			}
		}
		// dnat target
		if dnat, ok := e["dnat"].(map[string]interface{}); ok {
			if addr, ok := dnat["addr"].(string); ok {
				rule.NatTo = addr
			}
			if family, ok := dnat["family"].(string); ok {
				rule.NatAddrFamily = family
				if rule.AddrFamily == "" {
					rule.AddrFamily = family
				}
			}
		}
	}
}

var ifaceForwardIDRe = regexp.MustCompile(`^nft-ui iface-fwd (ifwd_[0-9a-f]{8})`)

func (m *IfaceForwardingManager) extractIDFromComment(comment string) string {
	matches := ifaceForwardIDRe.FindStringSubmatch(comment)
	if len(matches) < 2 {
		return ""
	}
	return matches[1]
}

func (m *IfaceForwardingManager) extractUserCommentFromComment(comment, id string) string {
	// comment format: "nft-ui iface-fwd ifwd_<8hex> <user comment>"
	prefix := IfaceForwardComment + " " + id + " "
	if strings.HasPrefix(comment, prefix) {
		return strings.TrimSpace(comment[len(prefix):])
	}
	return ""
}

func (m *IfaceForwardingManager) loadDisabledRules() ([]IfaceForwardRule, error) {
	data, err := os.ReadFile(m.disabledPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []IfaceForwardRule{}, nil
		}
		return nil, err
	}
	var file DisabledIfaceForwardsFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, err
	}
	for i := range file.Rules {
		file.Rules[i].Enabled = false
		file.Rules[i].Managed = true
	}
	return file.Rules, nil
}

func (m *IfaceForwardingManager) saveDisabledRules(rules []IfaceForwardRule) error {
	dir := filepath.Dir(m.disabledPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	file := DisabledIfaceForwardsFile{Rules: rules}
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.disabledPath, data, 0644)
}

// validateIfaceForwardFields validates the fields of an iface forward rule.
func validateIfaceForwardFields(iifName, addrFamily, natAddrFamily, dstAddr, natTo, protocol string) error {
	if iifName == "" {
		return fmt.Errorf("iif_name is required")
	}
	if len(iifName) > 15 {
		return fmt.Errorf("iif_name must be at most 15 characters")
	}
	ifaceRe := regexp.MustCompile(`^[a-zA-Z0-9._:-]+$`)
	if !ifaceRe.MatchString(iifName) {
		return fmt.Errorf("iif_name contains invalid characters")
	}
	if addrFamily != "ip" && addrFamily != "ip6" {
		return fmt.Errorf("addr_family must be 'ip' or 'ip6'")
	}
	effectiveNatFamily := natAddrFamily
	if effectiveNatFamily == "" {
		effectiveNatFamily = addrFamily
	} else if effectiveNatFamily != "ip" && effectiveNatFamily != "ip6" {
		return fmt.Errorf("nat_addr_family must be 'ip' or 'ip6'")
	}
	if err := validateAddrForFamily(dstAddr, addrFamily); err != nil {
		return fmt.Errorf("dst_addr: %w", err)
	}
	if err := validateAddrForFamily(natTo, effectiveNatFamily); err != nil {
		return fmt.Errorf("nat_to: %w", err)
	}
	if protocol != "tcp" && protocol != "udp" && protocol != "all" {
		return fmt.Errorf("protocol must be 'tcp', 'udp', or 'all'")
	}
	return nil
}

func validateAddrForFamily(addr, family string) error {
	if addr == "" {
		return fmt.Errorf("address is required")
	}
	parsed := net.ParseIP(addr)
	if parsed == nil {
		return fmt.Errorf("invalid IP address: %s", addr)
	}
	if family == "ip" && parsed.To4() == nil {
		return fmt.Errorf("%s is not a valid IPv4 address", addr)
	}
	if family == "ip6" && parsed.To4() != nil {
		return fmt.Errorf("%s is not a valid IPv6 address", addr)
	}
	return nil
}

// generateIfaceForwardID generates a unique "ifwd_<8hex>" ID.
func generateIfaceForwardID() string {
	b := make([]byte, 4)
	// Use time-based uniqueness (good enough for rule IDs)
	ts := time.Now().UnixNano()
	b[0] = byte(ts >> 24)
	b[1] = byte(ts >> 16)
	b[2] = byte(ts >> 8)
	b[3] = byte(ts)
	return fmt.Sprintf("ifwd_%02x%02x%02x%02x", b[0], b[1], b[2], b[3])
}

// sanitizeIfaceComment sanitizes a user-provided comment for use in nft rules.
func sanitizeIfaceComment(s string) string {
	re := regexp.MustCompile(`[^a-zA-Z0-9\s\-_.` + "\u4e00-\u9fff" + `]`)
	s = re.ReplaceAllString(s, "")
	if len(s) > 100 {
		s = s[:100]
	}
	return strings.TrimSpace(s)
}
