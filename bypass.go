package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const defaultBypassStatePath = "/var/lib/nft-ui/bypass.json"

// BypassManager handles the forward-bypass state file and kernel operations.
// It delegates low-level nft/ip calls to ForwardingManager.
type BypassManager struct {
	statePath string
	fwd       *ForwardingManager
}

// NewBypassManager creates a new BypassManager.
func NewBypassManager(statePath string, fwd *ForwardingManager) *BypassManager {
	if statePath == "" {
		statePath = defaultBypassStatePath
	}
	return &BypassManager{
		statePath: statePath,
		fwd:       fwd,
	}
}

// Load reads the persisted bypass config from disk.
// Returns a default (disabled) config if the file does not exist.
func (b *BypassManager) Load() (BypassConfig, error) {
	data, err := os.ReadFile(b.statePath)
	if err != nil {
		if os.IsNotExist(err) {
			return BypassConfig{Enabled: false, Mark: 0x1, Priority: 8990}, nil
		}
		return BypassConfig{}, fmt.Errorf("read bypass state: %w", err)
	}
	var cfg BypassConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return BypassConfig{}, fmt.Errorf("parse bypass state: %w", err)
	}
	return cfg, nil
}

// Save writes the bypass config to disk.
func (b *BypassManager) Save(cfg BypassConfig) error {
	if err := os.MkdirAll(filepath.Dir(b.statePath), 0755); err != nil {
		return fmt.Errorf("mkdir bypass state dir: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(b.statePath, data, 0644)
}

// Apply updates the ForwardingManager's mark/priority fields and calls
// EnsureForwardBypassSetup + SyncForwardBypassRules.
func (b *BypassManager) Apply(cfg BypassConfig) error {
	b.fwd.mu.Lock()
	b.fwd.forwardBypassMark = cfg.Mark
	b.fwd.forwardBypassPriority = cfg.Priority
	b.fwd.mu.Unlock()

	if err := b.fwd.EnsureForwardBypassSetup(); err != nil {
		return fmt.Errorf("setup bypass: %w", err)
	}
	if err := b.fwd.SyncForwardBypassRules(); err != nil {
		return fmt.Errorf("sync bypass rules: %w", err)
	}
	return nil
}

// Teardown removes all bypass infrastructure from the kernel and resets the
// ForwardingManager mark to 0 (disabled).
func (b *BypassManager) Teardown(cfg BypassConfig) error {
	// Temporarily set the mark so ForwardingManager knows what to clean up.
	b.fwd.mu.Lock()
	b.fwd.forwardBypassMark = cfg.Mark
	b.fwd.forwardBypassPriority = cfg.Priority
	b.fwd.mu.Unlock()

	var errs []string

	// Remove the ip rule.
	if _, err := b.fwd.execIP("-4", "rule", "del",
		"priority", fmt.Sprintf("%d", cfg.Priority),
		"fwmark", fmt.Sprintf("0x%x", cfg.Mark),
		"lookup", "main"); err != nil {
		if !strings.Contains(err.Error(), "No such file") &&
			!strings.Contains(err.Error(), "No rule") &&
			!strings.Contains(err.Error(), "ENOENT") {
			errs = append(errs, fmt.Sprintf("remove ip rule: %v", err))
		}
	}

	// Remove the ct mark restore rule from ip mangle prerouting.
	if err := b.fwd.deleteBypassRestoreRule(); err != nil {
		errs = append(errs, fmt.Sprintf("remove restore rule: %v", err))
	}

	// Remove per-forward mark rules for all managed forwards.
	if err := b.fwd.teardownAllRouteMarkRules(); err != nil {
		errs = append(errs, fmt.Sprintf("remove route mark rules: %v", err))
	}

	// Reset mark to 0 (disabled).
	b.fwd.mu.Lock()
	b.fwd.forwardBypassMark = 0
	b.fwd.mu.Unlock()

	if len(errs) > 0 {
		return fmt.Errorf("teardown errors: %s", strings.Join(errs, "; "))
	}
	return nil
}

// IPRulePresent returns true if the expected ip rule exists in the kernel.
func (b *BypassManager) IPRulePresent(cfg BypassConfig) bool {
	if cfg.Mark == 0 {
		return false
	}
	output, err := b.fwd.execIP("-4", "rule", "show")
	if err != nil {
		return false
	}
	expected := fmt.Sprintf("%d:", cfg.Priority)
	markStr := fmt.Sprintf("0x%x", cfg.Mark)
	for _, line := range strings.Split(string(output), "\n") {
		if strings.Contains(line, expected) && strings.Contains(line, markStr) && strings.Contains(line, "lookup main") {
			return true
		}
	}
	return false
}

// IPRuleString returns the expected ip rule as a human-readable string.
func IPRuleString(cfg BypassConfig) string {
	return fmt.Sprintf("%d: from all fwmark 0x%x lookup main", cfg.Priority, cfg.Mark)
}
