package main

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
)

// Handler holds dependencies for HTTP handlers
type Handler struct {
	nft       *NFTManager
	fwd       *ForwardingManager
	cfg       *Config
	logger    *log.Logger
	tokenGen  *TokenGenerator
	bypassMgr *BypassManager
}

// NewHandler creates a new Handler
func NewHandler(nft *NFTManager, fwd *ForwardingManager, cfg *Config, logger *log.Logger, tokenGen *TokenGenerator, bypassMgr *BypassManager) *Handler {
	return &Handler{
		nft:       nft,
		fwd:       fwd,
		cfg:       cfg,
		logger:    logger,
		tokenGen:  tokenGen,
		bypassMgr: bypassMgr,
	}
}

// saveRuleset persists the current nftables ruleset to disk
func (h *Handler) saveRuleset() {
	if err := h.nft.SaveRuleset(); err != nil {
		h.logger.Printf("Error saving ruleset: %v", err)
	}
}

// ListQuotas handles GET /api/v1/quotas
func (h *Handler) ListQuotas(c echo.Context) error {
	quotas, err := h.nft.ListQuotas()
	if err != nil {
		h.logger.Printf("Error listing quotas: %v", err)
		return c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
	}

	allowedPorts, err := h.nft.ListAllowedPorts()
	if err != nil {
		h.logger.Printf("Error listing allowed ports: %v", err)
		// Non-fatal: continue without allowed ports
		allowedPorts = []AllowedPort{}
	}

	return c.JSON(http.StatusOK, QuotasResponse{
		Quotas:          quotas,
		AllowedPorts:    allowedPorts,
		ReadOnly:        h.cfg.ReadOnly,
		RefreshInterval: h.cfg.RefreshInterval,
	})
}

// ResetQuota handles POST /api/v1/quotas/:id/reset
func (h *Handler) ResetQuota(c echo.Context) error {
	id := c.Param("id")

	if err := h.nft.ResetQuota(id); err != nil {
		h.logger.Printf("Error resetting quota %s: %v", id, err)
		return c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
	}

	h.logger.Printf("Quota reset: %s", id)
	h.saveRuleset()
	return c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "Quota reset successfully",
	})
}

// BatchResetQuotas handles POST /api/v1/quotas/batch-reset
func (h *Handler) BatchResetQuotas(c echo.Context) error {
	var req BatchResetRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Invalid request body",
		})
	}

	if len(req.IDs) == 0 {
		return c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "No IDs provided",
		})
	}

	if err := h.nft.BatchResetQuotas(req.IDs); err != nil {
		h.logger.Printf("Error batch resetting quotas: %v", err)
		return c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
	}

	h.logger.Printf("Batch reset %d quotas", len(req.IDs))
	h.saveRuleset()
	return c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "Quotas reset successfully",
	})
}

// ModifyQuota handles PUT /api/v1/quotas/:id
func (h *Handler) ModifyQuota(c echo.Context) error {
	id := c.Param("id")

	var req ModifyQuotaRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Invalid request body",
		})
	}

	if req.Bytes <= 0 {
		return c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Quota limit must be positive",
		})
	}

	if err := h.nft.ModifyQuota(id, req.Bytes); err != nil {
		h.logger.Printf("Error modifying quota %s: %v", id, err)
		return c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
	}

	h.logger.Printf("Quota modified: %s to %d bytes", id, req.Bytes)
	h.saveRuleset()
	return c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "Quota modified successfully",
	})
}

// AddQuota handles POST /api/v1/quotas
func (h *Handler) AddQuota(c echo.Context) error {
	var req AddQuotaRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Invalid request body",
		})
	}

	if req.Port < 1 || req.Port > 65535 {
		return c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Port must be between 1 and 65535",
		})
	}

	if req.Bytes <= 0 {
		return c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Quota limit must be positive",
		})
	}

	if err := h.nft.AddQuota(req.Port, req.Bytes, req.Comment); err != nil {
		h.logger.Printf("Error adding quota for port %d: %v", req.Port, err)
		return c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
	}

	h.logger.Printf("Quota added: port %d, limit %d bytes", req.Port, req.Bytes)
	h.saveRuleset()
	return c.JSON(http.StatusCreated, APIResponse{
		Success: true,
		Message: "Quota added successfully",
	})
}

// DeleteQuota handles DELETE /api/v1/quotas/:id
func (h *Handler) DeleteQuota(c echo.Context) error {
	id := c.Param("id")

	if err := h.nft.DeleteQuota(id); err != nil {
		h.logger.Printf("Error deleting quota %s: %v", id, err)
		return c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
	}

	h.logger.Printf("Quota deleted: %s", id)
	h.saveRuleset()
	return c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "Quota deleted successfully",
	})
}

// AddPort handles POST /api/v1/ports
func (h *Handler) AddPort(c echo.Context) error {
	var req AddPortRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Invalid request body",
		})
	}

	if req.Port < 1 || req.Port > 65535 {
		return c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Port must be between 1 and 65535",
		})
	}

	if err := h.nft.AddAllowedPort(req.Port); err != nil {
		h.logger.Printf("Error adding allowed port %d: %v", req.Port, err)
		return c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
	}

	h.logger.Printf("Allowed port added: %d", req.Port)
	h.saveRuleset()
	return c.JSON(http.StatusCreated, APIResponse{
		Success: true,
		Message: "Port added successfully",
	})
}

// DeletePort handles DELETE /api/v1/ports/:handle
func (h *Handler) DeletePort(c echo.Context) error {
	handleStr := c.Param("handle")
	handle, err := strconv.ParseInt(handleStr, 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Invalid handle",
		})
	}

	if err := h.nft.DeleteAllowedPort(handle); err != nil {
		h.logger.Printf("Error deleting allowed port handle %d: %v", handle, err)
		return c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
	}

	h.logger.Printf("Allowed port deleted: handle %d", handle)
	h.saveRuleset()
	return c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "Port deleted successfully",
	})
}

// ListQuotasWithTokens handles GET /api/v1/quotas when tokens are enabled
// Returns quotas with their query tokens for the admin panel
func (h *Handler) ListQuotasWithTokens(c echo.Context) error {
	quotas, err := h.nft.ListQuotas()
	if err != nil {
		h.logger.Printf("Error listing quotas: %v", err)
		return c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
	}

	allowedPorts, err := h.nft.ListAllowedPorts()
	if err != nil {
		h.logger.Printf("Error listing allowed ports: %v", err)
		allowedPorts = []AllowedPort{}
	}

	// Add tokens to quotas
	quotasWithTokens := make([]QuotaWithToken, len(quotas))
	for i, q := range quotas {
		quotasWithTokens[i] = QuotaWithToken{
			QuotaRule: q,
			Token:     h.tokenGen.Generate(q.Port),
		}
	}

	return c.JSON(http.StatusOK, QuotasResponseWithTokens{
		Quotas:          quotasWithTokens,
		AllowedPorts:    allowedPorts,
		ReadOnly:        h.cfg.ReadOnly,
		RefreshInterval: h.cfg.RefreshInterval,
	})
}

// QueryByToken handles GET /api/v1/public/query/:token (NO AUTH REQUIRED)
// Allows users to query quota usage with a token
func (h *Handler) QueryByToken(c echo.Context) error {
	token := c.Param("token")

	// Validate token format
	if !IsValidTokenFormat(token) {
		return c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Invalid token format",
		})
	}

	// Get all quotas
	quotas, err := h.nft.ListQuotas()
	if err != nil {
		h.logger.Printf("Error listing quotas for token query: %v", err)
		return c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   "Internal server error",
		})
	}

	// Find matching quota
	quota := h.tokenGen.FindQuotaByToken(token, quotas)
	if quota == nil {
		// Return generic error to prevent enumeration
		return c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   "Token not found",
		})
	}

	return c.JSON(http.StatusOK, PublicQueryResponse{
		Port:         quota.Port,
		UsedBytes:    quota.UsedBytes,
		QuotaBytes:   quota.QuotaBytes,
		UsagePercent: quota.UsagePercent,
		Status:       quota.Status,
		Comment:      quota.Comment,
	})
}

// ListForwarding handles GET /api/v1/forwarding
func (h *Handler) ListForwarding(c echo.Context) error {
	rules, err := h.fwd.ListForwardingRules()
	if err != nil {
		h.logger.Printf("Error listing forwarding rules: %v", err)
		return c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
	}

	return c.JSON(http.StatusOK, ForwardingResponse{
		Rules:    rules,
		ReadOnly: h.cfg.ReadOnly,
	})
}

// TestForwardingConnectivity handles GET /api/v1/forwarding/test
func (h *Handler) TestForwardingConnectivity(c echo.Context) error {
	dstIP := c.QueryParam("dst_ip")
	if !isValidIPv4(dstIP) {
		return c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Destination IP must be a valid IPv4 address",
		})
	}

	dstPort, err := strconv.Atoi(c.QueryParam("dst_port"))
	if err != nil || dstPort < 1 || dstPort > 65535 {
		return c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Destination port must be between 1 and 65535",
		})
	}

	protocol := c.QueryParam("protocol")
	if protocol != "tcp" && protocol != "udp" && protocol != "both" {
		return c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Protocol must be 'tcp', 'udp', or 'both'",
		})
	}

	timeout := defaultForwardingProbeTimeout
	if timeoutMs := c.QueryParam("timeout_ms"); timeoutMs != "" {
		parsedTimeout, err := strconv.Atoi(timeoutMs)
		if err != nil || parsedTimeout < 100 || parsedTimeout > int(maxForwardingProbeTimeout/time.Millisecond) {
			return c.JSON(http.StatusBadRequest, APIResponse{
				Success: false,
				Error:   "Timeout must be between 100 and 5000 milliseconds",
			})
		}
		timeout = time.Duration(parsedTimeout) * time.Millisecond
	}

	results, overallStatus, err := testForwardingTarget(dstIP, dstPort, protocol, timeout)
	if err != nil {
		h.logger.Printf("Error testing forwarding target %s:%d (%s): %v", dstIP, dstPort, protocol, err)
		return c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
	}

	h.logger.Printf("Forwarding connectivity test: %s:%d (%s) -> %s", dstIP, dstPort, protocol, overallStatus)
	return c.JSON(http.StatusOK, ForwardingProbeResponse{
		Success:           true,
		DstIP:             dstIP,
		DstPort:           dstPort,
		RequestedProtocol: protocol,
		OverallStatus:     overallStatus,
		TestedAt:          time.Now().Format(time.RFC3339),
		Results:           results,
	})
}

// AddForwarding handles POST /api/v1/forwarding
func (h *Handler) AddForwarding(c echo.Context) error {
	var req AddForwardingRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Invalid request body",
		})
	}

	if req.SrcPort < 1 || req.SrcPort > 65535 {
		return c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Source port must be between 1 and 65535",
		})
	}

	if req.DstPort < 1 || req.DstPort > 65535 {
		return c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Destination port must be between 1 and 65535",
		})
	}

	if req.Protocol != "tcp" && req.Protocol != "udp" && req.Protocol != "both" {
		return c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Protocol must be 'tcp', 'udp', or 'both'",
		})
	}

	if req.LimitMbps < 0 {
		return c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Limit must be >= 0 (0 = no limit)",
		})
	}

	if normalizeMSSMode(req.MSSMode) == "" {
		return c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "MSS mode must be 'pmtu', 'fixed1452', or 'disabled'",
		})
	}

	if normalizeSourceNATMode(req.SourceNATMode) == "" {
		return c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Source NAT mode must be 'masquerade' or 'snat'",
		})
	}
	if normalizeSourceNATMode(req.SourceNATMode) == SourceNATModeSNAT && !isValidIPv4(req.SNATAddress) {
		return c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "SNAT address must be a valid IPv4 address",
		})
	}

	if err := h.fwd.AddForwardingRule(req.SrcPort, req.DstIP, req.DstPort, req.Protocol, req.Comment, req.LimitMbps, req.MSSMode, req.SourceNATMode, req.SNATAddress); err != nil {
		h.logger.Printf("Error adding forwarding rule: %v", err)
		return c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
	}

	h.logger.Printf("Forwarding rule added: %d -> %s:%d (%s) limit=%d Mbps mss=%s nat=%s snat=%s", req.SrcPort, req.DstIP, req.DstPort, req.Protocol, req.LimitMbps, normalizeMSSMode(req.MSSMode), normalizeSourceNATMode(req.SourceNATMode), req.SNATAddress)
	h.saveRuleset()
	return c.JSON(http.StatusCreated, APIResponse{
		Success: true,
		Message: "Forwarding rule added successfully",
	})
}

// EditForwarding handles PUT /api/v1/forwarding/:id
func (h *Handler) EditForwarding(c echo.Context) error {
	id := c.Param("id")

	var req EditForwardingRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Invalid request body",
		})
	}

	if req.DstPort < 1 || req.DstPort > 65535 {
		return c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Destination port must be between 1 and 65535",
		})
	}

	if req.Protocol != "tcp" && req.Protocol != "udp" && req.Protocol != "both" {
		return c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Protocol must be 'tcp', 'udp', or 'both'",
		})
	}

	if req.LimitMbps < 0 {
		return c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Limit must be >= 0 (0 = no limit)",
		})
	}

	if normalizeMSSMode(req.MSSMode) == "" {
		return c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "MSS mode must be 'pmtu', 'fixed1452', or 'disabled'",
		})
	}

	if normalizeSourceNATMode(req.SourceNATMode) == "" {
		return c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Source NAT mode must be 'masquerade' or 'snat'",
		})
	}
	if normalizeSourceNATMode(req.SourceNATMode) == SourceNATModeSNAT && !isValidIPv4(req.SNATAddress) {
		return c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "SNAT address must be a valid IPv4 address",
		})
	}

	if err := h.fwd.EditForwardingRule(id, req.DstIP, req.DstPort, req.Protocol, req.Comment, req.LimitMbps, req.MSSMode, req.SourceNATMode, req.SNATAddress); err != nil {
		h.logger.Printf("Error editing forwarding rule %s: %v", id, err)
		return c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
	}

	h.logger.Printf("Forwarding rule edited: %s -> %s:%d (%s) limit=%d Mbps mss=%s nat=%s snat=%s", id, req.DstIP, req.DstPort, req.Protocol, req.LimitMbps, normalizeMSSMode(req.MSSMode), normalizeSourceNATMode(req.SourceNATMode), req.SNATAddress)
	h.saveRuleset()
	return c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "Forwarding rule updated successfully",
	})
}

// DeleteForwarding handles DELETE /api/v1/forwarding/:id
func (h *Handler) DeleteForwarding(c echo.Context) error {
	id := c.Param("id")

	if err := h.fwd.DeleteForwardingRule(id); err != nil {
		h.logger.Printf("Error deleting forwarding rule %s: %v", id, err)
		return c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
	}

	h.logger.Printf("Forwarding rule deleted: %s", id)
	h.saveRuleset()
	return c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "Forwarding rule deleted successfully",
	})
}

// EnableForwarding handles POST /api/v1/forwarding/:id/enable
func (h *Handler) EnableForwarding(c echo.Context) error {
	id := c.Param("id")

	if err := h.fwd.EnableForwardingRule(id); err != nil {
		h.logger.Printf("Error enabling forwarding rule %s: %v", id, err)
		return c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
	}

	h.logger.Printf("Forwarding rule enabled: %s", id)
	h.saveRuleset()
	return c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "Forwarding rule enabled successfully",
	})
}

// DisableForwarding handles POST /api/v1/forwarding/:id/disable
func (h *Handler) DisableForwarding(c echo.Context) error {
	id := c.Param("id")

	if err := h.fwd.DisableForwardingRule(id); err != nil {
		h.logger.Printf("Error disabling forwarding rule %s: %v", id, err)
		return c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
	}

	h.logger.Printf("Forwarding rule disabled: %s", id)
	h.saveRuleset()
	return c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "Forwarding rule disabled successfully",
	})
}

// GetRawRuleset handles GET /api/v1/raw-ruleset
func (h *Handler) GetRawRuleset(c echo.Context) error {
	rawData, err := h.nft.GetRawRuleset()
	if err != nil {
		h.logger.Printf("Error getting raw ruleset: %v", err)
		return c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    rawData,
	})
}

// ExportBackup handles GET /api/v1/backup
func (h *Handler) ExportBackup(c echo.Context) error {
	// Get all quotas
	quotas, err := h.nft.ListQuotas()
	if err != nil {
		h.logger.Printf("Error listing quotas for backup: %v", err)
		return c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
	}

	// Get all forwarding rules
	forwardingRules, err := h.fwd.ListForwardingRules()
	if err != nil {
		h.logger.Printf("Error listing forwarding rules for backup: %v", err)
		return c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
	}

	// Get all allowed ports (managed ones only)
	allowedPorts, err := h.nft.ListAllowedPorts()
	if err != nil {
		h.logger.Printf("Error listing allowed ports for backup: %v", err)
		allowedPorts = []AllowedPort{}
	}

	// Build backup data
	backup := BackupData{
		Version:    1,
		CreatedAt:  time.Now().Format(time.RFC3339),
		Quotas:     make([]BackupQuota, 0, len(quotas)),
		Forwarding: make([]BackupForwarding, 0, len(forwardingRules)),
		Ports:      make([]int, 0),
	}

	// Convert quotas
	for _, q := range quotas {
		backup.Quotas = append(backup.Quotas, BackupQuota{
			Port:       q.Port,
			QuotaBytes: q.QuotaBytes,
			Comment:    q.Comment,
		})
	}

	// Convert forwarding rules
	for _, rule := range forwardingRules {
		backup.Forwarding = append(backup.Forwarding, BackupForwarding{
			SrcPort:       rule.SrcPort,
			DstIP:         rule.DstIP,
			DstPort:       rule.DstPort,
			Protocol:      rule.Protocol,
			Comment:       rule.Comment,
			LimitMbps:     rule.LimitMbps,
			MSSMode:       rule.MSSMode,
			SourceNATMode: rule.SourceNATMode,
			SNATAddress:   rule.SNATAddress,
			Enabled:       rule.Enabled,
		})
	}

	// Convert allowed ports (only managed ones)
	for _, p := range allowedPorts {
		if p.Managed {
			backup.Ports = append(backup.Ports, p.Port)
		}
	}

	// Set download headers
	filename := "nft-ui-backup-" + time.Now().Format("2006-01-02") + ".json"
	c.Response().Header().Set("Content-Disposition", "attachment; filename="+filename)
	c.Response().Header().Set("Content-Type", "application/json")

	h.logger.Printf("Exported backup: %d quotas, %d forwarding rules, %d ports",
		len(backup.Quotas), len(backup.Forwarding), len(backup.Ports))

	return c.JSON(http.StatusOK, backup)
}

// ImportBackup handles POST /api/v1/backup
func (h *Handler) ImportBackup(c echo.Context) error {
	var backup BackupData
	if err := c.Bind(&backup); err != nil {
		return c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Invalid backup file format",
		})
	}

	// Validate version
	if backup.Version != 1 {
		return c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Unsupported backup version",
		})
	}

	summary := ImportSummary{}

	// Import quotas
	for _, q := range backup.Quotas {
		err := h.nft.AddQuota(q.Port, q.QuotaBytes, q.Comment)
		if err != nil {
			h.logger.Printf("Skipping quota port %d: %v", q.Port, err)
			summary.QuotasSkipped++
		} else {
			summary.QuotasAdded++
		}
	}

	// Import forwarding rules
	for _, f := range backup.Forwarding {
		if f.Enabled {
			// Add as active rule
			err := h.fwd.AddForwardingRule(f.SrcPort, f.DstIP, f.DstPort, f.Protocol, f.Comment, f.LimitMbps, f.MSSMode, f.SourceNATMode, f.SNATAddress)
			if err != nil {
				h.logger.Printf("Skipping forwarding rule %d: %v", f.SrcPort, err)
				summary.ForwardingSkipped++
			} else {
				summary.ForwardingAdded++
			}
		} else {
			// Add as disabled rule - we'll use the forwarding manager's internal method
			// Since there's no public API for adding disabled rules, we'll add it first then disable it
			err := h.fwd.AddForwardingRule(f.SrcPort, f.DstIP, f.DstPort, f.Protocol, f.Comment, f.LimitMbps, f.MSSMode, f.SourceNATMode, f.SNATAddress)
			if err != nil {
				h.logger.Printf("Skipping disabled forwarding rule %d: %v", f.SrcPort, err)
				summary.ForwardingSkipped++
			} else {
				// Disable it immediately
				id := "fwd_" + strconv.Itoa(f.SrcPort)
				if err := h.fwd.DisableForwardingRule(id); err != nil {
					h.logger.Printf("Warning: added rule %d but failed to disable: %v", f.SrcPort, err)
				}
				summary.ForwardingAdded++
			}
		}
	}

	// Import allowed ports
	for _, port := range backup.Ports {
		err := h.nft.AddAllowedPort(port)
		if err != nil {
			h.logger.Printf("Skipping port %d: %v", port, err)
			summary.PortsSkipped++
		} else {
			summary.PortsAdded++
		}
	}

	// Save ruleset
	h.saveRuleset()

	h.logger.Printf("Import complete: quotas=%d/%d, forwarding=%d/%d, ports=%d/%d",
		summary.QuotasAdded, summary.QuotasAdded+summary.QuotasSkipped,
		summary.ForwardingAdded, summary.ForwardingAdded+summary.ForwardingSkipped,
		summary.PortsAdded, summary.PortsAdded+summary.PortsSkipped)

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Backup imported successfully",
		"summary": summary,
	})
}

// GetBypass handles GET /api/v1/bypass
func (h *Handler) GetBypass(c echo.Context) error {
	cfg, err := h.bypassMgr.Load()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
	}

	applied := h.bypassMgr.IPRulePresent(cfg)
	ipRule := ""
	if cfg.Mark != 0 {
		ipRule = IPRuleString(cfg)
	}

	return c.JSON(http.StatusOK, BypassStatusResponse{
		Success: true,
		Config:  cfg,
		Applied: applied,
		IPRule:  ipRule,
	})
}

// SetBypass handles POST /api/v1/bypass
func (h *Handler) SetBypass(c echo.Context) error {
	var req SetBypassRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Invalid request body",
		})
	}

	if req.Enabled && req.Mark == 0 {
		req.Mark = 0x1
	}
	if req.Priority <= 0 {
		req.Priority = 8990
	}

	cfg := BypassConfig{
		Enabled:  req.Enabled,
		Mark:     req.Mark,
		Priority: req.Priority,
	}

	if req.Enabled {
		if err := h.bypassMgr.Apply(cfg); err != nil {
			h.logger.Printf("Error applying forward bypass: %v", err)
			return c.JSON(http.StatusInternalServerError, APIResponse{
				Success: false,
				Error:   err.Error(),
			})
		}
		h.logger.Printf("Forward bypass enabled (mark=0x%x priority=%d)", cfg.Mark, cfg.Priority)
	} else {
		// Load current config to know the old mark for teardown
		oldCfg, err := h.bypassMgr.Load()
		if err == nil && oldCfg.Enabled && oldCfg.Mark != 0 {
			if err := h.bypassMgr.Teardown(oldCfg); err != nil {
				h.logger.Printf("Warning: forward bypass teardown errors: %v", err)
			}
		}
		// Keep mark/priority in saved state even when disabled (for re-enable UX)
		cfg.Mark = req.Mark
		if cfg.Mark == 0 {
			cfg.Mark = 0x1
		}
		h.logger.Printf("Forward bypass disabled")
	}

	if err := h.bypassMgr.Save(cfg); err != nil {
		h.logger.Printf("Warning: failed to save bypass state: %v", err)
	}

	applied := h.bypassMgr.IPRulePresent(cfg)
	ipRule := ""
	if cfg.Mark != 0 {
		ipRule = IPRuleString(cfg)
	}

	return c.JSON(http.StatusOK, BypassStatusResponse{
		Success: true,
		Config:  cfg,
		Applied: applied,
		IPRule:  ipRule,
	})
}
