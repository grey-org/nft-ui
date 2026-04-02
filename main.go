package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

//go:embed frontend/dist/*
var frontendFS embed.FS

func main() {
	// Load configuration
	cfg, err := LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize logger
	logger := log.New(os.Stdout, "[nft-ui] ", log.LstdFlags)

	// Initialize NFT manager
	nftMgr := NewNFTManager(cfg)

	// Restore saved ruleset (if any)
	if err := nftMgr.RestoreRuleset(); err != nil {
		logger.Printf("Warning: failed to restore ruleset: %v", err)
	} else {
		logger.Printf("Ruleset restored from %s", cfg.RulesetPath)
	}

	// Initialize forwarding manager
	fwdMgr := NewForwardingManager(cfg)

	// Initialize interface forwarding manager
	ifaceFwdMgr := NewIfaceForwardingManager(cfg, logger)

	// Initialize bypass manager and apply persisted state on startup
	bypassMgr := NewBypassManager(cfg.BypassStatePath, fwdMgr)
	if bypassCfg, err := bypassMgr.Load(); err != nil {
		logger.Printf("Warning: failed to load bypass state: %v", err)
	} else if bypassCfg.Enabled && bypassCfg.Mark != 0 {
		logger.Printf("Applying persisted forward bypass (mark=0x%x priority=%d)", bypassCfg.Mark, bypassCfg.Priority)
		if err := bypassMgr.Apply(bypassCfg); err != nil {
			logger.Printf("Warning: failed to apply forward bypass: %v", err)
		}
	}

	if err := fwdMgr.ReconcileManagedForwardingRules(); err != nil {
		logger.Printf("Warning: failed to reconcile forwarding rules: %v", err)
	}
	if err := fwdMgr.ReconcileMSSClampRules(); err != nil {
		logger.Printf("Warning: failed to reconcile MSS clamp rules: %v", err)
	}
	if err := nftMgr.ReconcileForwardQuotaRules(); err != nil {
		logger.Printf("Warning: failed to reconcile forward quota rules: %v", err)
	}

	// Initialize token generator (may be nil if not configured)
	var tokenGen *TokenGenerator
	if cfg.TokenSalt != "" {
		tokenGen = NewTokenGenerator(cfg.TokenSalt)
	}

	// Initialize handler
	handler := NewHandler(nftMgr, fwdMgr, ifaceFwdMgr, cfg, logger, tokenGen, bypassMgr)

	// Create Echo instance
	e := echo.New()
	e.HideBanner = true

	// Global middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete},
	}))

	// Public API routes (NO AUTH) - must be registered BEFORE auth middleware
	if cfg.TokenEnabled() {
		public := e.Group("/api/v1/public")
		public.Use(RateLimitMiddleware(100, time.Minute)) // 100 requests per minute per IP
		public.GET("/query/:token", handler.QueryByToken)
		logger.Printf("Public query endpoint enabled at /api/v1/public/query/:token")
	}

	// Protected API routes (with auth)
	api := e.Group("/api/v1")
	api.Use(BasicAuthMiddleware(cfg))
	api.Use(ReadOnlyMiddleware(cfg))
	api.Use(AuditLogMiddleware(logger))

	// Register API endpoints - use token-enhanced version if tokens are configured
	if cfg.TokenSalt != "" {
		api.GET("/quotas", handler.ListQuotasWithTokens)
	} else {
		api.GET("/quotas", handler.ListQuotas)
	}
	api.POST("/quotas/:id/reset", handler.ResetQuota)
	api.POST("/quotas/batch-reset", handler.BatchResetQuotas)
	api.PUT("/quotas/:id", handler.ModifyQuota)
	api.POST("/quotas", handler.AddQuota)
	api.DELETE("/quotas/:id", handler.DeleteQuota)

	// Port management endpoints
	api.POST("/ports", handler.AddPort)
	api.DELETE("/ports/:handle", handler.DeletePort)

	// Forwarding management endpoints
	api.GET("/forwarding", handler.ListForwarding)
	api.GET("/forwarding/test", handler.TestForwardingConnectivity)
	api.POST("/forwarding", handler.AddForwarding)
	api.PUT("/forwarding/:id", handler.EditForwarding)
	api.DELETE("/forwarding/:id", handler.DeleteForwarding)
	api.POST("/forwarding/:id/enable", handler.EnableForwarding)
	api.POST("/forwarding/:id/disable", handler.DisableForwarding)

	// Interface forwarding management endpoints
	api.GET("/iface-forwarding", handler.ListIfaceForwarding)
	api.POST("/iface-forwarding", handler.AddIfaceForwarding)
	api.PUT("/iface-forwarding/:id", handler.EditIfaceForwarding)
	api.DELETE("/iface-forwarding/:id", handler.DeleteIfaceForwarding)
	api.POST("/iface-forwarding/:id/enable", handler.EnableIfaceForwarding)
	api.POST("/iface-forwarding/:id/disable", handler.DisableIfaceForwarding)

	// Bypass management endpoints
	api.GET("/bypass", handler.GetBypass)
	api.POST("/bypass", handler.SetBypass)

	// Raw ruleset endpoint
	api.GET("/raw-ruleset", handler.GetRawRuleset)

	// System status endpoints
	api.GET("/system/conntrack", handler.GetConntrackStatus)

	// Backup/restore endpoints
	api.GET("/backup", handler.ExportBackup)
	api.POST("/backup", handler.ImportBackup)

	// Serve frontend
	setupFrontend(e)

	// Start server
	logger.Printf("Starting server on %s (read_only=%v, auth=%v, public_query=%v)",
		cfg.ListenAddr, cfg.ReadOnly, cfg.AuthEnabled(), cfg.TokenEnabled())
	e.Logger.Fatal(e.Start(cfg.ListenAddr))
}

// setupFrontend configures the embedded frontend serving
func setupFrontend(e *echo.Echo) {
	// Get the frontend/dist subdirectory
	distFS, err := fs.Sub(frontendFS, "frontend/dist")
	if err != nil {
		// Frontend not built yet, serve a placeholder
		e.GET("/*", func(c echo.Context) error {
			return c.HTML(http.StatusOK, `<!DOCTYPE html>
<html>
<head><title>nft-ui</title></head>
<body>
<h1>nft-ui</h1>
<p>Frontend not built. Run <code>cd frontend && npm install && npm run build</code></p>
<p>API available at <a href="/api/v1/quotas">/api/v1/quotas</a></p>
</body>
</html>`)
		})
		return
	}

	// Create file server for static files
	fileServer := http.FileServer(http.FS(distFS))

	// Read index.html content for SPA fallback
	indexHTML, _ := fs.ReadFile(distFS, "index.html")

	// Serve static files and handle SPA routing
	e.GET("/*", echo.WrapHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/" {
			path = "/index.html"
		}

		// Try to open the file
		f, err := distFS.Open(path[1:]) // Remove leading /
		if err != nil {
			// File not found, serve index.html for SPA routing
			// Don't use fileServer here as it redirects /index.html to /
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write(indexHTML)
			return
		}
		f.Close()

		fileServer.ServeHTTP(w, r)
	})))
}
