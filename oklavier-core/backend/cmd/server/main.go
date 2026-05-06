package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/gofiber/fiber/v2/utils"
	"golang.org/x/crypto/bcrypt"
	"oklavier-api/internal/agent"
	"oklavier-api/internal/auth"
	"oklavier-api/internal/cache"
	"oklavier-api/internal/db"
	"oklavier-api/internal/handlers"
	"oklavier-api/internal/metrics"
	"oklavier-api/internal/middleware"
	mtlsLib "oklavier-api/internal/mtls"
)

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

// Branding represents the platform branding configuration
type Branding struct {
	AppName      string `db:"app_name" json:"app_name"`
	LogoURL      string `db:"logo_url" json:"logo_url"`
	LogoDarkURL  string `db:"logo_dark_url" json:"logo_dark_url"`
	FaviconURL   string `db:"favicon_url" json:"favicon_url"`
	Creator      string `db:"creator" json:"creator"`
	CreatorURL   string `db:"creator_url" json:"creator_url"`
	PrimaryColor string `db:"primary_color" json:"primary_color"`
	AccentColor  string `db:"accent_color" json:"accent_color"`
	LoginBG      string `db:"login_bg" json:"login_bg"`
}

const Version = "1.0.2"

func main() {
	// Config
	dbHost := getEnv("DB_HOST", "localhost")
	dbPort := getEnv("DB_PORT", "5432")
	dbUser := getEnv("DB_USER", "oklavier")
	dbPass := os.Getenv("DB_PASSWORD")
	if dbPass == "" {
		log.Fatal("FATAL: DB_PASSWORD environment variable must be set")
	}
	dbName := getEnv("DB_NAME", "oklavier")
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("FATAL: JWT_SECRET environment variable must be set")
	}
	// SECURITY: enforce minimum entropy. Anything shorter than 32 bytes is
	// brute-forceable offline given a single signed JWT.
	if len(jwtSecret) < 32 {
		log.Fatal("FATAL: JWT_SECRET must be at least 32 characters (use `openssl rand -hex 32`)")
	}
	// SECURITY: encryption-at-rest key is now separate from JWT signing key.
	// Reusing JWT_SECRET as the AES key collapsed two trust boundaries: any
	// leak of the JWT secret meant decryption of all DB-encrypted credentials.
	// Falls back to a key derived from JWT_SECRET via HKDF (with a distinct label)
	// for backwards-compat with existing encrypted data; new deployments should
	// set OKLAVIER_ENCRYPTION_KEY explicitly.
	encryptionKey := os.Getenv("OKLAVIER_ENCRYPTION_KEY")
	if encryptionKey == "" {
		log.Println("WARN: OKLAVIER_ENCRYPTION_KEY not set; deriving from JWT_SECRET via HKDF for backwards compat. Set it explicitly to decouple encryption-at-rest from JWT signing.")
		encryptionKey = jwtSecret
	} else if len(encryptionKey) < 32 {
		log.Fatal("FATAL: OKLAVIER_ENCRYPTION_KEY must be at least 32 characters")
	}
	listenAddr := getEnv("LISTEN_ADDR", ":8080")
	frontendURL := getEnv("FRONTEND_URL", "http://localhost:3001")
	kubeconfig := getEnv("KUBECONFIG", "")
	k8sNamespace := getEnv("K8S_NAMESPACE", "oklavier")

	// Init
	auth.SetJWTSecret(jwtSecret)

	database, err := db.New(dbHost, dbPort, dbUser, dbPass, dbName)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()
	log.Println("Connected to PostgreSQL")

	// Set encryption key for credential encryption at rest (uses JWT_SECRET)
	database.EncryptionKey = encryptionKey

	// Auto-create all tables
	migrations := []string{
		// Auth tables (replaces BetterAuth)
		`CREATE TABLE IF NOT EXISTS "user" (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			email TEXT NOT NULL UNIQUE,
			"emailVerified" BOOLEAN NOT NULL DEFAULT false,
			image TEXT,
			"createdAt" TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
			"updatedAt" TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
			role TEXT DEFAULT 'user',
			banned BOOLEAN DEFAULT false,
			"banReason" TEXT,
			"banExpires" TIMESTAMP WITH TIME ZONE
		)`,
		`CREATE TABLE IF NOT EXISTS "session" (
			id TEXT PRIMARY KEY,
			"expiresAt" TIMESTAMP WITH TIME ZONE NOT NULL,
			token TEXT NOT NULL UNIQUE,
			"createdAt" TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
			"updatedAt" TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
			"ipAddress" TEXT,
			"userAgent" TEXT,
			"userId" TEXT NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
			"impersonatedBy" TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS "account" (
			id TEXT PRIMARY KEY,
			"accountId" TEXT NOT NULL,
			"providerId" TEXT NOT NULL,
			"userId" TEXT NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
			"accessToken" TEXT,
			"refreshToken" TEXT,
			"idToken" TEXT,
			"accessTokenExpiresAt" TIMESTAMP WITH TIME ZONE,
			"refreshTokenExpiresAt" TIMESTAMP WITH TIME ZONE,
			scope TEXT,
			password TEXT,
			"createdAt" TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
			"updatedAt" TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS "verification" (
			id TEXT PRIMARY KEY,
			identifier TEXT NOT NULL,
			value TEXT NOT NULL,
			"expiresAt" TIMESTAMP WITH TIME ZONE NOT NULL,
			"createdAt" TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			"updatedAt" TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)`,
		// Application tables
		`CREATE TABLE IF NOT EXISTS agent (
			id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
			name TEXT NOT NULL,
			region TEXT NOT NULL DEFAULT 'default',
			namespace TEXT NOT NULL DEFAULT 'oklavier',
			endpoint TEXT DEFAULT '',
			public_url TEXT DEFAULT '',
			token TEXT NOT NULL DEFAULT gen_random_uuid()::text,
			status TEXT NOT NULL DEFAULT 'pending',
			last_heartbeat TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			active_sessions INTEGER DEFAULT 0,
			total_nodes INTEGER DEFAULT 0,
			total_cpu TEXT DEFAULT '0',
			total_memory TEXT DEFAULT '0',
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS workspace (
			id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
			name TEXT NOT NULL,
			friendly_name TEXT NOT NULL,
			description TEXT DEFAULT '',
			image_src TEXT DEFAULT '',
			docker_image TEXT NOT NULL,
			cores REAL NOT NULL DEFAULT 2,
			memory BIGINT NOT NULL DEFAULT 2768000000,
			shm_size TEXT DEFAULT '512m',
			enabled BOOLEAN DEFAULT true,
			category TEXT DEFAULT '',
			x_res INTEGER DEFAULT 1920,
			y_res INTEGER DEFAULT 1080,
			docker_registry TEXT DEFAULT '',
			docker_user TEXT DEFAULT '',
			docker_password TEXT DEFAULT '',
			session_time_limit INTEGER DEFAULT 0,
			gpu_count INTEGER DEFAULT 0,
			uncompressed_size_mb INTEGER DEFAULT 0,
			restrict_to_agent TEXT DEFAULT '',
			restrict_to_region TEXT DEFAULT '',
			run_config TEXT DEFAULT '{}',
			exec_config TEXT DEFAULT '{}',
			volume_mappings TEXT DEFAULT '{}',
			categories TEXT DEFAULT '{}',
			notes TEXT,
			persistent BOOLEAN DEFAULT false,
			persistent_size TEXT DEFAULT '',
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS workspace_session (
			id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
			user_id TEXT NOT NULL DEFAULT '',
			workspace_id TEXT NOT NULL REFERENCES workspace(id),
			pod_name TEXT DEFAULT '',
			service_name TEXT DEFAULT '',
			container_ip TEXT DEFAULT '',
			vnc_password TEXT DEFAULT '',
			status TEXT NOT NULL DEFAULT 'starting',
			agent_id TEXT DEFAULT '',
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			expires_at TIMESTAMP WITH TIME ZONE,
			keepalive_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS oklavier_group (
			id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
			name TEXT NOT NULL UNIQUE,
			description TEXT DEFAULT '',
			color TEXT DEFAULT '#6366f1',
			is_default BOOLEAN DEFAULT false,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS user_group (
			user_id TEXT NOT NULL,
			group_id TEXT NOT NULL REFERENCES oklavier_group(id),
			PRIMARY KEY (user_id, group_id)
		)`,
		`CREATE TABLE IF NOT EXISTS workspace_group (
			workspace_id TEXT NOT NULL REFERENCES workspace(id),
			group_id TEXT NOT NULL REFERENCES oklavier_group(id),
			PRIMARY KEY (workspace_id, group_id)
		)`,
		`CREATE TABLE IF NOT EXISTS oidc_role_mapping (
			id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
			oidc_role TEXT NOT NULL,
			group_id TEXT NOT NULL REFERENCES oklavier_group(id),
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS user_oidc_role (
			user_id TEXT NOT NULL,
			role TEXT NOT NULL,
			PRIMARY KEY (user_id, role)
		)`,
		`CREATE TABLE IF NOT EXISTS auth_method (
			id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
			name TEXT NOT NULL,
			type TEXT NOT NULL DEFAULT 'oidc',
			enabled BOOLEAN DEFAULT false,
			config JSONB DEFAULT '{}',
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS audit_log (
			id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
			user_id TEXT NOT NULL DEFAULT '',
			user_email TEXT NOT NULL DEFAULT '',
			action TEXT NOT NULL,
			resource_type TEXT NOT NULL,
			resource_id TEXT NOT NULL DEFAULT '',
			details TEXT NOT NULL DEFAULT '',
			ip_address TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_log_created_at ON audit_log(created_at DESC)`,
		`CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL DEFAULT '',
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)`,
		// Add auth_provider column to user table if missing (for OIDC tracking)
		`DO $$ BEGIN
			ALTER TABLE "user" ADD COLUMN IF NOT EXISTS auth_provider TEXT DEFAULT 'credential';
			ALTER TABLE "user" ADD COLUMN IF NOT EXISTS oidc_provider_name TEXT DEFAULT '';
			ALTER TABLE "user" ADD COLUMN IF NOT EXISTS last_login TIMESTAMP WITH TIME ZONE;
			ALTER TABLE "user" ADD COLUMN IF NOT EXISTS failed_login_attempts INTEGER DEFAULT 0;
			ALTER TABLE "user" ADD COLUMN IF NOT EXISTS locked_until TIMESTAMP WITH TIME ZONE;
		EXCEPTION WHEN others THEN NULL; END $$`,
		// Add quota columns to groups
		`DO $$ BEGIN
			ALTER TABLE oklavier_group ADD COLUMN IF NOT EXISTS max_sessions INTEGER DEFAULT 0;
			ALTER TABLE oklavier_group ADD COLUMN IF NOT EXISTS max_cpu REAL DEFAULT 0;
			ALTER TABLE oklavier_group ADD COLUMN IF NOT EXISTS max_memory BIGINT DEFAULT 0;
		EXCEPTION WHEN others THEN NULL; END $$`,
		// Create default group if none exists
		`INSERT INTO oklavier_group (name, description, is_default) SELECT 'Default', 'Default group for all users', true WHERE NOT EXISTS (SELECT 1 FROM oklavier_group WHERE is_default = true)`,
		// Branding table
		`CREATE TABLE IF NOT EXISTS branding (
			id TEXT PRIMARY KEY DEFAULT 'default',
			app_name TEXT NOT NULL DEFAULT 'Oklavier',
			logo_url TEXT DEFAULT '',
			logo_dark_url TEXT DEFAULT '',
			favicon_url TEXT DEFAULT '',
			creator TEXT DEFAULT '',
			creator_url TEXT DEFAULT '',
			primary_color TEXT DEFAULT '#4F46E5',
			accent_color TEXT DEFAULT '#F97316',
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)`,
		`INSERT INTO branding (id, app_name, creator) VALUES ('default', 'Oklavier', '') ON CONFLICT DO NOTHING`,
		// Add login_bg column to branding
		`DO $$ BEGIN
			ALTER TABLE branding ADD COLUMN IF NOT EXISTS login_bg TEXT DEFAULT '';
		EXCEPTION WHEN others THEN NULL; END $$`,
		// Server workspace support
		`DO $$ BEGIN
			ALTER TABLE workspace ADD COLUMN IF NOT EXISTS workspace_type TEXT DEFAULT 'container';
			ALTER TABLE workspace ADD COLUMN IF NOT EXISTS server_hostname TEXT DEFAULT '';
			ALTER TABLE workspace ADD COLUMN IF NOT EXISTS server_port INTEGER DEFAULT 0;
			ALTER TABLE workspace ADD COLUMN IF NOT EXISTS server_protocol TEXT DEFAULT '';
			ALTER TABLE workspace ADD COLUMN IF NOT EXISTS server_username TEXT DEFAULT '';
			ALTER TABLE workspace ADD COLUMN IF NOT EXISTS server_password TEXT DEFAULT '';
			ALTER TABLE workspace ADD COLUMN IF NOT EXISTS server_domain TEXT DEFAULT '';
			ALTER TABLE workspace ADD COLUMN IF NOT EXISTS server_ignore_cert BOOLEAN DEFAULT true;
			ALTER TABLE workspace ADD COLUMN IF NOT EXISTS server_auth_mode TEXT DEFAULT 'static';
			ALTER TABLE workspace ADD COLUMN IF NOT EXISTS server_allow_remember BOOLEAN DEFAULT false;
			ALTER TABLE workspace ADD COLUMN IF NOT EXISTS server_default_settings TEXT DEFAULT '{}';
			ALTER TABLE workspace ADD COLUMN IF NOT EXISTS server_security TEXT DEFAULT '';
			ALTER TABLE workspace_session ADD COLUMN IF NOT EXISTS session_type TEXT DEFAULT 'container';
			ALTER TABLE agent ADD COLUMN IF NOT EXISTS version TEXT DEFAULT '';
		EXCEPTION WHEN others THEN NULL; END $$`,
		// JWT auth: refresh token table
		`CREATE TABLE IF NOT EXISTS refresh_token (
			jti TEXT PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
			expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			ip_address TEXT DEFAULT '',
			user_agent TEXT DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS idx_refresh_token_user_id ON refresh_token(user_id)`,
		// Drop legacy cookie-based auth session table (migrated to JWT + refresh_token)
		`DROP TABLE IF EXISTS "session"`,
		// Session recording support
		`DO $$ BEGIN
			ALTER TABLE workspace ADD COLUMN IF NOT EXISTS record_sessions BOOLEAN DEFAULT false;
		EXCEPTION WHEN others THEN NULL; END $$`,
		`CREATE TABLE IF NOT EXISTS session_recording (
			id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
			session_id TEXT NOT NULL,
			user_id TEXT NOT NULL DEFAULT '',
			workspace_name TEXT NOT NULL DEFAULT '',
			s3_key TEXT NOT NULL DEFAULT '',
			file_size BIGINT DEFAULT 0,
			duration_seconds INTEGER DEFAULT 0,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_session_recording_created_at ON session_recording(created_at DESC)`,
		// v1.1.0: support local storage (no S3)
		`DO $$ BEGIN
			ALTER TABLE session_recording ADD COLUMN IF NOT EXISTS storage_type TEXT DEFAULT 's3';
			ALTER TABLE session_recording ADD COLUMN IF NOT EXISTS agent_id TEXT DEFAULT '';
		EXCEPTION WHEN others THEN NULL; END $$`,
		// Guest access links
		`CREATE TABLE IF NOT EXISTS guest_link (
			id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
			workspace_id TEXT NOT NULL REFERENCES workspace(id),
			token TEXT NOT NULL UNIQUE DEFAULT gen_random_uuid()::text,
			created_by TEXT NOT NULL DEFAULT '',
			label TEXT NOT NULL DEFAULT '',
			password_hash TEXT DEFAULT '',
			max_uses INTEGER DEFAULT 1,
			used_count INTEGER DEFAULT 0,
			expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_guest_link_token ON guest_link(token)`,
		// v1.2.0: workspace favorites
		`CREATE TABLE IF NOT EXISTS workspace_favorite (
			user_id TEXT NOT NULL,
			workspace_id TEXT NOT NULL REFERENCES workspace(id) ON DELETE CASCADE,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			PRIMARY KEY (user_id, workspace_id)
		)`,
		// v1.2.0: performance indexes for analytics and cursor pagination
		`CREATE INDEX IF NOT EXISTS idx_ws_created_at ON workspace_session(created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_ws_status ON workspace_session(status)`,
		`CREATE INDEX IF NOT EXISTS idx_ws_user_status ON workspace_session(user_id, status)`,
		`CREATE INDEX IF NOT EXISTS idx_ws_workspace_created ON workspace_session(workspace_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_action ON audit_log(action)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_resource ON audit_log(resource_type)`,
		`CREATE INDEX IF NOT EXISTS idx_session_recording_user ON session_recording(user_id)`,
	}
	for _, m := range migrations {
		if _, err := database.Exec(m); err != nil {
			log.Printf("Migration warning: %v", err)
		}
	}
	log.Println("Database migrations complete")

	// Encrypt any existing plaintext credentials (one-time migration)
	database.MigrateEncryptCredentials()

	// Bootstrap admin user from env vars (only if no users exist)
	adminEmail := getEnv("OKLAVIER_ADMIN_EMAIL", "")
	adminPassword := getEnv("OKLAVIER_ADMIN_PASSWORD", "")
	adminName := getEnv("OKLAVIER_ADMIN_NAME", "Admin")
	if adminEmail != "" && adminPassword != "" {
		var count int
		database.Get(&count, `SELECT COUNT(*) FROM "user"`)
		if count == 0 {
			hash, _ := auth.HashPassword(adminPassword)
			userID := generateID()
			now := time.Now()
			database.Exec(`INSERT INTO "user" (id, name, email, "emailVerified", "createdAt", "updatedAt", role) VALUES ($1, $2, $3, true, $4, $4, 'admin')`,
				userID, adminName, adminEmail, now)
			database.Exec(`INSERT INTO "account" (id, "accountId", "providerId", "userId", password, "createdAt", "updatedAt") VALUES ($1, $2, 'credential', $3, $4, $5, $5)`,
				generateID(), userID, userID, hash, now)
			log.Printf("[Bootstrap] Admin created: %s", adminEmail)
		}
	}

	// K8s Agent
	k8sAgent, err := agent.New(kubeconfig, k8sNamespace)
	if err != nil {
		log.Fatalf("Failed to create K8s agent: %v", err)
	}

	// Handlers
	authHandler := &handlers.AuthHandler{DB: database}

	// New auth handlers (replaces BetterAuth)
	valkeyURL := getEnv("VALKEY_URL", "")
	rateLimiter := auth.NewRateLimiter(valkeyURL)
	blacklist := auth.NewTokenBlacklist(valkeyURL)
	appCache := cache.New(valkeyURL)
	// SecureCookie drives the `Secure` attribute on auth cookies; false in
	// local dev (http://localhost), true once the frontend is HTTPS.
	secureCookie := strings.HasPrefix(frontendURL, "https://")
	authHandlers := &handlers.AuthHandlers{DB: database, RateLimiter: rateLimiter, Blacklist: blacklist, FrontendURL: frontendURL, SecureCookie: secureCookie}

	sessionHandler := &handlers.SessionHandler{DB: database, Agent: k8sAgent, RateLimiter: rateLimiter, Cache: appCache}
	adminHandler := &handlers.AdminHandler{DB: database, Cache: appCache}

	// Fiber app
	app := fiber.New(fiber.Config{
		AppName:   "Oklavier API",
		BodyLimit: 10 * 1024 * 1024, // 10MB max request body
	})

	// Structured JSON logs with request_id (correlated to handler logs by Locals).
	// SECURITY: format excludes ${queryParams} and ${body} so secrets and bearers
	// (in URL or body) never reach the access log.
	app.Use(requestid.New(requestid.Config{
		Header:     "X-Request-ID",
		Generator:  utils.UUIDv4,
		ContextKey: "request_id",
	}))
	app.Use(logger.New(logger.Config{
		Format:     `{"ts":"${time}","level":"info","status":${status},"method":"${method}","path":"${route}","ip":"${ip}","ua":"${ua}","latency_ms":${latency},"req_id":"${locals:request_id}"}` + "\n",
		TimeFormat: "2006-01-02T15:04:05.000Z07:00",
	}))
	app.Use(middleware.SecurityHeaders())
	app.Use(middleware.InternalOnly())
	// CSRF defense: reject cross-site state-changing requests. Cookies use
	// SameSite=Lax which already blocks XHR cross-site, but Lax allows
	// top-level form-POSTs — this Origin/Sec-Fetch-Site check closes that gap.
	app.Use(middleware.CSRFGuard([]string{frontendURL}))
	app.Use(cors.New(cors.Config{
		AllowOrigins:     frontendURL,
		AllowCredentials: false,
		AllowHeaders:     "Content-Type, Authorization",
	}))
	app.Use(middleware.AuditFailedRequests(database))

	// HTTP request counter middleware for Prometheus metrics
	app.Use(func(c *fiber.Ctx) error {
		err := c.Next()
		status := c.Response().StatusCode()
		switch {
		case status >= 500:
			metrics.HTTPRequests5xx.Add(1)
		case status >= 400:
			metrics.HTTPRequests4xx.Add(1)
		default:
			metrics.HTTPRequests2xx.Add(1)
		}
		return err
	})

	// Public routes (no auth)
	api := app.Group("/api")
	api.Get("/health", func(c *fiber.Ctx) error {
		if err := database.DB.Ping(); err != nil {
			return c.Status(503).JSON(fiber.Map{"ok": false, "error": "database unreachable"})
		}
		return c.JSON(fiber.Map{"ok": true})
	})

	// Prometheus metrics endpoint (no auth — scraped by Prometheus)
	api.Get("/metrics", func(c *fiber.Ctx) error {
		var activeSessions int
		if err := database.Get(&activeSessions, "SELECT COUNT(*) FROM workspace_session WHERE status IN ('running','starting')"); err != nil {
			activeSessions = 0
		}
		var totalUsers int
		if err := database.Get(&totalUsers, `SELECT COUNT(*) FROM "user"`); err != nil {
			totalUsers = 0
		}
		var totalWorkspaces int
		if err := database.Get(&totalWorkspaces, "SELECT COUNT(*) FROM workspace"); err != nil {
			totalWorkspaces = 0
		}
		var agentsConnected int
		if err := database.Get(&agentsConnected, "SELECT COUNT(*) FROM agent WHERE status = 'connected' AND last_heartbeat > NOW() - INTERVAL '2 minutes'"); err != nil {
			agentsConnected = 0
		}
		var agentsTotal int
		if err := database.Get(&agentsTotal, "SELECT COUNT(*) FROM agent"); err != nil {
			agentsTotal = 0
		}
		var guestLinksActive int
		if err := database.Get(&guestLinksActive, "SELECT COUNT(*) FROM guest_link WHERE expires_at > NOW()"); err != nil {
			guestLinksActive = 0
		}
		dbHealthy := 0
		if err := database.DB.Ping(); err == nil {
			dbHealthy = 1
		}
		valkeyHealthy := 0
		if rateLimiter != nil && rateLimiter.Ping() {
			valkeyHealthy = 1
		}
		out := fmt.Sprintf(
			"# HELP oklavier_active_sessions Number of active sessions\n"+
				"# TYPE oklavier_active_sessions gauge\n"+
				"oklavier_active_sessions %d\n"+
				"# HELP oklavier_total_users Total registered users\n"+
				"# TYPE oklavier_total_users gauge\n"+
				"oklavier_total_users %d\n"+
				"# HELP oklavier_total_workspaces Total configured workspaces\n"+
				"# TYPE oklavier_total_workspaces gauge\n"+
				"oklavier_total_workspaces %d\n"+
				"# HELP oklavier_agents_connected Number of connected agents\n"+
				"# TYPE oklavier_agents_connected gauge\n"+
				"oklavier_agents_connected %d\n"+
				"# HELP oklavier_agents_total Total registered agents\n"+
				"# TYPE oklavier_agents_total gauge\n"+
				"oklavier_agents_total %d\n"+
				"# HELP oklavier_sessions_created_total Total sessions created (counter)\n"+
				"# TYPE oklavier_sessions_created_total counter\n"+
				"oklavier_sessions_created_total %d\n"+
				"# HELP oklavier_http_requests_total Total HTTP requests by status\n"+
				"# TYPE oklavier_http_requests_total counter\n"+
				"oklavier_http_requests_total{status=\"2xx\"} %d\n"+
				"oklavier_http_requests_total{status=\"4xx\"} %d\n"+
				"oklavier_http_requests_total{status=\"5xx\"} %d\n"+
				"# HELP oklavier_db_healthy Database health status\n"+
				"# TYPE oklavier_db_healthy gauge\n"+
				"oklavier_db_healthy %d\n"+
				"# HELP oklavier_valkey_healthy Valkey/Redis health status\n"+
				"# TYPE oklavier_valkey_healthy gauge\n"+
				"oklavier_valkey_healthy %d\n"+
				"# HELP oklavier_guest_links_active Number of active (non-expired) guest links\n"+
				"# TYPE oklavier_guest_links_active gauge\n"+
				"oklavier_guest_links_active %d\n",
			activeSessions, totalUsers, totalWorkspaces,
			agentsConnected, agentsTotal,
			metrics.SessionsCreatedTotal.Load(),
			metrics.HTTPRequests2xx.Load(), metrics.HTTPRequests4xx.Load(), metrics.HTTPRequests5xx.Load(),
			dbHealthy, valkeyHealthy, guestLinksActive,
		)
		c.Set("Content-Type", "text/plain; version=0.0.4")
		return c.SendString(out)
	})

	api.Get("/login_settings", authHandler.LoginSettings)
	// SECURITY: legacy /authenticate path removed. It used single-round salted SHA-256
	// for password verification and issued tokens via auth.GenerateToken whose
	// ValidateToken keyfunc did not enforce the HMAC signing method (alg-confusion).
	// Use /api/auth/login (bcrypt + ValidateAccessToken with HMAC enforcement).
	api.Post("/authenticate", func(c *fiber.Ctx) error {
		return c.Status(410).JSON(fiber.Map{"error": "Endpoint removed. Use /api/auth/login."})
	})

	// Auth routes (public, rate-limited for login)
	api.Post("/auth/login", authHandlers.Login)
	api.Post("/auth/logout", authHandlers.Logout)
	api.Post("/auth/refresh", authHandlers.Refresh)
	api.Post("/auth/forgot-password", authHandlers.ForgotPassword)
	api.Post("/auth/reset-password", authHandlers.ResetPassword)

	// OIDC handler
	oidcHandler := &handlers.OIDCHandler{DB: database, SecureCookie: secureCookie, FrontendURL: frontendURL}
	api.Get("/auth/providers", oidcHandler.ListProviders)
	api.Get("/auth/oidc/:providerId", oidcHandler.Authorize)
	api.Get("/auth/oidc/:providerId/callback", oidcHandler.Callback)

	// Public branding endpoint (needed for login page) — cached 5min
	api.Get("/branding", func(c *fiber.Ctx) error {
		var b Branding
		if appCache.Get("branding", &b) {
			return c.JSON(fiber.Map{
				"app_name": b.AppName, "logo_url": b.LogoURL, "logo_dark_url": b.LogoDarkURL,
				"favicon_url": b.FaviconURL, "creator": b.Creator, "creator_url": b.CreatorURL,
				"primary_color": b.PrimaryColor, "accent_color": b.AccentColor, "login_bg": b.LoginBG,
				"version": Version,
			})
		}
		err := database.Get(&b, `SELECT app_name, COALESCE(logo_url,'') as logo_url, COALESCE(logo_dark_url,'') as logo_dark_url, COALESCE(favicon_url,'') as favicon_url, COALESCE(creator,'') as creator, COALESCE(creator_url,'') as creator_url, COALESCE(primary_color,'#4F46E5') as primary_color, COALESCE(accent_color,'#F97316') as accent_color, COALESCE(login_bg,'') as login_bg FROM branding WHERE id = 'default'`)
		if err != nil {
			return c.JSON(fiber.Map{"app_name": "Oklavier", "primary_color": "#4F46E5", "accent_color": "#F97316", "version": Version})
		}
		appCache.Set("branding", b, 5*time.Minute)
		return c.JSON(fiber.Map{
			"app_name": b.AppName, "logo_url": b.LogoURL, "logo_dark_url": b.LogoDarkURL,
			"favicon_url": b.FaviconURL, "creator": b.Creator, "creator_url": b.CreatorURL,
			"primary_color": b.PrimaryColor, "accent_color": b.AccentColor, "login_bg": b.LoginBG,
			"version": Version,
		})
	})

	// Guest access routes (public, no auth required)
	api.Get("/guest/:token", func(c *fiber.Ctx) error {
		token := c.Params("token")
		type GuestLinkInfo struct {
			ID           string    `db:"id"`
			WorkspaceID  string    `db:"workspace_id"`
			Label        string    `db:"label"`
			PasswordHash string    `db:"password_hash"`
			MaxUses      int       `db:"max_uses"`
			UsedCount    int       `db:"used_count"`
			ExpiresAt    time.Time `db:"expires_at"`
		}
		var link GuestLinkInfo
		err := database.Get(&link, `SELECT id, workspace_id, COALESCE(label,'') as label, COALESCE(password_hash,'') as password_hash, COALESCE(max_uses,1) as max_uses, COALESCE(used_count,0) as used_count, expires_at FROM guest_link WHERE token = $1`, token)
		if err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "Link not found"})
		}
		if time.Now().After(link.ExpiresAt) {
			return c.Status(410).JSON(fiber.Map{"error": "Link has expired"})
		}
		if link.MaxUses > 0 && link.UsedCount >= link.MaxUses {
			return c.Status(410).JSON(fiber.Map{"error": "Link has reached maximum uses"})
		}

		// Get workspace info
		workspace, err := sessionHandler.DB.GetWorkspace(link.WorkspaceID)
		if err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "Workspace not found"})
		}

		return c.JSON(fiber.Map{
			"workspace_id":   workspace.ID,
			"workspace_name": workspace.FriendlyName,
			"workspace_description": func() string {
				if workspace.Description != nil {
					return *workspace.Description
				}
				return ""
			}(),
			"workspace_image":      workspace.ImageSrc,
			"workspace_type":       workspace.WorkspaceType,
			"requires_password":    link.PasswordHash != "",
			"requires_credentials": workspace.ServerAuthMode == "prompt",
			"label":                link.Label,
		})
	})

	api.Post("/guest/:token/session", func(c *fiber.Ctx) error {
		if rateLimiter != nil && !rateLimiter.Allow(c.IP()) {
			return c.Status(429).JSON(fiber.Map{"error": "Too many requests. Please wait."})
		}
		token := c.Params("token")
		var req struct {
			Password       string `json:"password"`
			Lang           string `json:"lang"`
			ServerUsername string `json:"server_username"`
			ServerPassword string `json:"server_password"`
			ServerDomain   string `json:"server_domain"`
		}
		c.BodyParser(&req)

		type GuestLink struct {
			ID           string    `db:"id"`
			WorkspaceID  string    `db:"workspace_id"`
			PasswordHash string    `db:"password_hash"`
			MaxUses      int       `db:"max_uses"`
			UsedCount    int       `db:"used_count"`
			ExpiresAt    time.Time `db:"expires_at"`
			CreatedBy    string    `db:"created_by"`
		}
		var link GuestLink
		err := database.Get(&link, `SELECT id, workspace_id, COALESCE(password_hash,'') as password_hash, COALESCE(max_uses,1) as max_uses, COALESCE(used_count,0) as used_count, expires_at, COALESCE(created_by,'') as created_by FROM guest_link WHERE token = $1`, token)
		if err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "Link not found"})
		}
		if time.Now().After(link.ExpiresAt) {
			return c.Status(410).JSON(fiber.Map{"error": "Link has expired"})
		}

		// Check password BEFORE consuming a use (don't burn the use on a wrong password attempt)
		if link.PasswordHash != "" {
			if err := bcrypt.CompareHashAndPassword([]byte(link.PasswordHash), []byte(req.Password)); err != nil {
				return c.Status(401).JSON(fiber.Map{"error": "Invalid password"})
			}
		}

		// SECURITY: atomic increment+check guards against race-double-use of single-use links.
		atomicRes, atomicErr := database.Exec(`UPDATE guest_link
			SET used_count = used_count + 1
			WHERE id = $1 AND (max_uses = 0 OR used_count < max_uses) AND expires_at > NOW()`, link.ID)
		if atomicErr != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Internal error"})
		}
		if rows, _ := atomicRes.RowsAffected(); rows == 0 {
			return c.Status(410).JSON(fiber.Map{"error": "Link has reached maximum uses"})
		}

		// Get workspace
		workspace, err := sessionHandler.DB.GetWorkspace(link.WorkspaceID)
		if err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "Workspace not found"})
		}

		// Find agent (same logic as RequestSession)
		var agentID, agentEndpoint, agentToken string
		if workspace.RestrictToAgent != "" {
			agentID = workspace.RestrictToAgent
			database.QueryRow(`SELECT endpoint, token FROM agent WHERE id = $1`, agentID).Scan(&agentEndpoint, &agentToken)
		} else if workspace.RestrictToRegion != "" {
			database.QueryRow(`SELECT id, endpoint, token FROM agent WHERE status = 'connected' AND last_heartbeat > NOW() - INTERVAL '2 minutes' AND region = $1 ORDER BY active_sessions ASC LIMIT 1`, workspace.RestrictToRegion).Scan(&agentID, &agentEndpoint, &agentToken)
		} else {
			database.QueryRow(`SELECT id, endpoint, token FROM agent WHERE status = 'connected' AND last_heartbeat > NOW() - INTERVAL '2 minutes' ORDER BY active_sessions ASC LIMIT 1`).Scan(&agentID, &agentEndpoint, &agentToken)
		}
		if agentEndpoint == "" {
			return c.Status(500).JSON(fiber.Map{"error": "No agent available"})
		}

		sessionID := generateID()
		guestUserID := "guest:" + link.ID
		lang := req.Lang
		if lang == "" {
			lang = "en"
		}

		if workspace.WorkspaceType == "server" {
			srvUser := workspace.ServerUsername
			srvPass := workspace.ServerPassword
			srvDomain := workspace.ServerDomain
			if workspace.ServerAuthMode == "prompt" {
				srvUser = req.ServerUsername
				srvPass = req.ServerPassword
				if req.ServerDomain != "" {
					srvDomain = req.ServerDomain
				}
			}
			agentBody := map[string]interface{}{
				"session_id": sessionID, "user_id": guestUserID, "lang": lang,
				"protocol": workspace.ServerProtocol, "hostname": workspace.ServerHostname, "port": workspace.ServerPort,
				"username": srvUser, "password": srvPass, "domain": srvDomain,
				"ignore_cert": workspace.ServerIgnoreCert, "security": workspace.ServerSecurity,
				"default_settings": workspace.ServerDefaultSettings,
				"width":            workspace.XRes, "height": workspace.YRes,
				"record_sessions": workspace.RecordSessions, "workspace_name": workspace.FriendlyName,
			}
			if workspace.RecordSessions {
				agentBody["s3_endpoint"] = database.GetSetting("s3.endpoint")
				agentBody["s3_access_key"] = database.GetSetting("s3.access_key")
				agentBody["s3_secret_key"] = database.GetSetting("s3.secret_key")
				agentBody["s3_bucket"] = database.GetSetting("s3.bucket")
				agentBody["s3_region"] = database.GetSetting("s3.region")
			}
			agentReqBody, _ := json.Marshal(agentBody)
			if _, err := sessionHandler.CallAgent(agentEndpoint, agentToken, "/api/create-server-session", agentReqBody); err != nil {
				return c.Status(500).JSON(fiber.Map{"error": "Failed to connect to agent"})
			}
			expiration := time.Now().Add(1 * time.Hour)
			if workspace.SessionTimeLimit > 0 {
				expiration = time.Now().Add(time.Duration(workspace.SessionTimeLimit) * time.Second)
			}
			database.Exec(`INSERT INTO workspace_session (id, user_id, workspace_id, container_ip, status, agent_id, expires_at, session_type) VALUES ($1,$2,$3,$4,'running',$5,$6,'server')`,
				sessionID, guestUserID, workspace.ID, workspace.ServerHostname, agentID, expiration)
		} else {
			agentReqBody, _ := json.Marshal(map[string]interface{}{
				"session_id": sessionID, "docker_image": workspace.DockerImage, "cores": workspace.Cores,
				"memory": workspace.Memory, "shm_size": workspace.SHMSize, "persistent": workspace.Persistent,
				"persistent_size": workspace.PersistentSize, "user_id": guestUserID, "workspace_id": workspace.ID,
				"run_config": json.RawMessage(workspace.RunConfig), "exec_config": json.RawMessage(workspace.ExecConfig),
				"volume_mappings": json.RawMessage(workspace.VolumeMappings), "docker_registry": workspace.DockerRegistry,
				"docker_user": workspace.DockerUser, "docker_password": workspace.DockerPassword, "gpu_count": workspace.GPUCount,
			})
			agentResp, err := sessionHandler.CallAgent(agentEndpoint, agentToken, "/api/create-session", agentReqBody)
			if err != nil {
				return c.Status(500).JSON(fiber.Map{"error": "Failed to connect to agent"})
			}
			var podResp struct {
				PodName string `json:"pod_name"`
				PodIP   string `json:"pod_ip"`
			}
			json.Unmarshal(agentResp, &podResp)

			guacBody := map[string]interface{}{
				"session_id": sessionID, "user_id": guestUserID, "lang": lang,
				"protocol": "vnc", "hostname": podResp.PodIP, "port": 5900,
				"username": "", "password": "", "domain": "", "ignore_cert": false, "security": "",
				"default_settings": "{}", "width": workspace.XRes, "height": workspace.YRes,
				"record_sessions": workspace.RecordSessions, "workspace_name": workspace.FriendlyName,
			}
			if workspace.RecordSessions {
				guacBody["s3_endpoint"] = database.GetSetting("s3.endpoint")
				guacBody["s3_access_key"] = database.GetSetting("s3.access_key")
				guacBody["s3_secret_key"] = database.GetSetting("s3.secret_key")
				guacBody["s3_bucket"] = database.GetSetting("s3.bucket")
				guacBody["s3_region"] = database.GetSetting("s3.region")
			}
			guacReqBody, _ := json.Marshal(guacBody)
			if _, err := sessionHandler.CallAgent(agentEndpoint, agentToken, "/api/create-server-session", guacReqBody); err != nil {
				cleanupBody, _ := json.Marshal(map[string]string{"session_id": sessionID})
				sessionHandler.CallAgent(agentEndpoint, agentToken, "/api/destroy-session", cleanupBody)
				return c.Status(500).JSON(fiber.Map{"error": "Failed to register session"})
			}
			expiration := time.Now().Add(1 * time.Hour)
			if workspace.SessionTimeLimit > 0 {
				expiration = time.Now().Add(time.Duration(workspace.SessionTimeLimit) * time.Second)
			}
			database.Exec(`INSERT INTO workspace_session (id, user_id, workspace_id, pod_name, container_ip, status, agent_id, expires_at, session_type) VALUES ($1,$2,$3,$4,$5,'running',$6,$7,'container')`,
				sessionID, guestUserID, workspace.ID, podResp.PodName, podResp.PodIP, agentID, expiration)
		}

		// used_count was already atomically incremented above (race-safe)

		// Get agent public URL
		var agentPublicURL string
		database.QueryRow("SELECT COALESCE(public_url, '') FROM agent WHERE id = $1", agentID).Scan(&agentPublicURL)

		sessionToken, tokenErr := handlers.GenerateGuestSessionBearer(agentEndpoint, agentToken, guestUserID, sessionID)
		if tokenErr != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to issue session bearer"})
		}
		database.LogAudit(guestUserID, "", "create", "guest_session", sessionID, "guest_link="+link.ID, c.IP())

		return c.JSON(fiber.Map{
			"session_id":    sessionID,
			"status":        "running",
			"agent_url":     agentPublicURL,
			"session_type":  workspace.WorkspaceType,
			"session_token": sessionToken,
		})
	})

	// Agent routes (auth by agent token) — MUST be before authenticated group
	agentAPI := api.Group("/agent", middleware.AgentTokenRequired(database.DB))
	_ = agentAPI

	// Authenticated routes (session cookie, basic auth, or bearer token)
	// Skip auth for agent routes (they use AgentTokenRequired)
	authenticated := api.Group("", func(c *fiber.Ctx) error {
		if strings.HasPrefix(c.Path(), "/api/agent/") {
			return c.Next()
		}
		return middleware.AuthRequired(database.DB, blacklist)(c)
	})

	// Auth routes requiring session
	authenticated.Get("/auth/me", authHandlers.Me)
	authenticated.Post("/auth/change-password", authHandlers.ChangePassword)
	authenticated.Post("/auth/update-profile", authHandlers.UpdateProfile)
	authenticated.Post("/get_user_images", sessionHandler.GetWorkspaces)
	authenticated.Post("/get_user_sessions", sessionHandler.GetUserSessions)
	authenticated.Post("/request_session", sessionHandler.RequestSession)
	authenticated.Post("/destroy_session", sessionHandler.DestroySession)
	authenticated.Post("/session_readiness", sessionHandler.GetSessionReadiness)
	authenticated.Post("/session/connect", sessionHandler.ConnectSession)

	// Workspace favorites
	authenticated.Post("/favorites", func(c *fiber.Ctx) error {
		userID := c.Locals("user_id").(string)
		var ids []string
		database.Select(&ids, "SELECT workspace_id FROM workspace_favorite WHERE user_id=$1", userID)
		if ids == nil {
			ids = []string{}
		}
		return c.JSON(fiber.Map{"workspace_ids": ids})
	})
	authenticated.Post("/favorites/toggle", func(c *fiber.Ctx) error {
		userID := c.Locals("user_id").(string)
		var req struct {
			WorkspaceID string `json:"workspace_id"`
		}
		if err := c.BodyParser(&req); err != nil || req.WorkspaceID == "" {
			return c.Status(400).JSON(fiber.Map{"error": "workspace_id required"})
		}
		var exists int
		database.Get(&exists, "SELECT COUNT(*) FROM workspace_favorite WHERE user_id=$1 AND workspace_id=$2", userID, req.WorkspaceID)
		if exists > 0 {
			database.Exec("DELETE FROM workspace_favorite WHERE user_id=$1 AND workspace_id=$2", userID, req.WorkspaceID)
			return c.JSON(fiber.Map{"favorited": false})
		}
		database.Exec("INSERT INTO workspace_favorite (user_id, workspace_id) VALUES ($1, $2)", userID, req.WorkspaceID)
		return c.JSON(fiber.Map{"favorited": true})
	})

	// Admin routes (auth + admin role required)
	admin := authenticated.Group("/admin", middleware.AdminRequired())
	// Healthcheck dashboard
	admin.Post("/health", func(c *fiber.Ctx) error {
		health := fiber.Map{}

		// Database
		dbOK := database.Ping() == nil
		health["database"] = fiber.Map{"status": boolToStatus(dbOK), "type": "PostgreSQL"}

		// Valkey/Redis
		valkeyOK := rateLimiter != nil && rateLimiter.Ping()
		health["valkey"] = fiber.Map{"status": boolToStatus(valkeyOK), "type": "Valkey"}

		// Agents
		type AgentHealth struct {
			ID       string `db:"id" json:"id"`
			Name     string `db:"name" json:"name"`
			Status   string `db:"status" json:"status"`
			LastHB   string `db:"last_heartbeat" json:"last_heartbeat"`
			Sessions int    `db:"active_sessions" json:"active_sessions"`
		}
		var agents []AgentHealth
		database.Select(&agents, "SELECT id, name, status, last_heartbeat, COALESCE(active_sessions,0) as active_sessions FROM agent")
		if agents == nil {
			agents = []AgentHealth{}
		}
		health["agents"] = agents

		// General stats
		var totalSessions int
		database.Get(&totalSessions, "SELECT COUNT(*) FROM workspace_session WHERE status IN ('running','starting')")
		var totalUsers int
		database.Get(&totalUsers, `SELECT COUNT(*) FROM "user"`)
		health["stats"] = fiber.Map{"active_sessions": totalSessions, "total_users": totalUsers}

		return c.JSON(health)
	})

	admin.Post("/get_servers", adminHandler.GetServers)
	admin.Post("/get_images", adminHandler.GetImages)
	admin.Post("/get_sessions", adminHandler.GetSessions)
	admin.Post("/workspaces", adminHandler.GetWorkspaces)
	admin.Post("/workspaces/create", adminHandler.CreateWorkspace)
	admin.Post("/workspaces/update", adminHandler.UpdateWorkspace)
	admin.Post("/workspaces/delete", adminHandler.DeleteWorkspace)
	admin.Post("/workspaces/toggle", adminHandler.ToggleWorkspace)
	admin.Post("/sessions", adminHandler.GetActiveSessions)
	admin.Post("/sessions/shadow", sessionHandler.ShadowSession)
	admin.Post("/sessions/bulk-destroy", func(c *fiber.Ctx) error {
		var req struct {
			SessionIDs []string `json:"session_ids"`
		}
		if err := c.BodyParser(&req); err != nil || len(req.SessionIDs) == 0 {
			return c.Status(400).JSON(fiber.Map{"error": "session_ids required"})
		}
		destroyed := 0
		for _, sid := range req.SessionIDs {
			session, err := database.GetSession(sid)
			if err != nil {
				continue
			}
			if session.AgentID != "" {
				var endpoint, token string
				database.QueryRow("SELECT endpoint, token FROM agent WHERE id = $1", session.AgentID).Scan(&endpoint, &token)
				if endpoint != "" {
					body, _ := json.Marshal(map[string]string{"session_id": sid})
					destroyPath := "/api/destroy-session"
					if session.SessionType == "server" {
						destroyPath = "/api/destroy-server-session"
					}
					url := fmt.Sprintf("%s%s", strings.TrimRight(endpoint, "/"), destroyPath)
					httpReq, _ := http.NewRequest("POST", url, bytes.NewReader(body))
					httpReq.Header.Set("Content-Type", "application/json")
					httpReq.Header.Set("X-Agent-Token", token)
					client := &http.Client{Timeout: 15 * time.Second}
					client.Do(httpReq)
				}
			}
			database.DeleteSession(sid)
			destroyed++
		}
		adminID, _ := c.Locals("user_id").(string)
		database.LogAudit(adminID, "", "bulk_destroy", "session", "", fmt.Sprintf("destroyed %d sessions", destroyed), c.IP())
		return c.JSON(fiber.Map{"ok": true, "destroyed": destroyed})
	})
	admin.Post("/revoke-sessions", authHandlers.RevokeSessions)
	admin.Post("/signup", authHandlers.Signup) // Only admins can create users

	// User management
	admin.Post("/users", func(c *fiber.Ctx) error {
		var req struct {
			Page    int    `json:"page"`
			PerPage int    `json:"per_page"`
			Search  string `json:"search"`
		}
		c.BodyParser(&req)
		if req.PerPage == 0 {
			req.PerPage = 1000
		}
		if req.Page == 0 {
			req.Page = 1
		}
		offset := (req.Page - 1) * req.PerPage

		var total int
		if req.Search != "" {
			database.Get(&total, `SELECT COUNT(*) FROM "user" WHERE name ILIKE $1 OR email ILIKE $1`, "%"+req.Search+"%")
		} else {
			database.Get(&total, `SELECT COUNT(*) FROM "user"`)
		}

		type UserRow struct {
			ID           string `db:"id" json:"id"`
			Name         string `db:"name" json:"name"`
			Email        string `db:"email" json:"email"`
			Role         string `db:"role" json:"role"`
			Banned       bool   `db:"banned" json:"banned"`
			AuthProvider string `db:"auth_provider" json:"auth_provider"`
			CreatedAt    string `db:"createdAt" json:"createdAt"`
		}
		var users []UserRow
		query := `SELECT id, name, email, COALESCE(role,'user') as role, COALESCE(banned,false) as banned, COALESCE(auth_provider,'credential') as auth_provider, "createdAt" FROM "user"`
		if req.Search != "" {
			query += ` WHERE name ILIKE $1 OR email ILIKE $1 ORDER BY "createdAt" DESC LIMIT ` + fmt.Sprintf("%d OFFSET %d", req.PerPage, offset)
			database.Select(&users, query, "%"+req.Search+"%")
		} else {
			query += ` ORDER BY "createdAt" DESC LIMIT ` + fmt.Sprintf("%d OFFSET %d", req.PerPage, offset)
			database.Select(&users, query)
		}
		if users == nil {
			users = []UserRow{}
		}

		return c.JSON(fiber.Map{"users": users, "total": total, "page": req.Page, "per_page": req.PerPage})
	})
	admin.Post("/users/delete", func(c *fiber.Ctx) error {
		var req struct {
			ID string `json:"id"`
		}
		c.BodyParser(&req)
		database.Exec(`DELETE FROM refresh_token WHERE user_id = $1`, req.ID)
		database.Exec(`DELETE FROM "account" WHERE "userId" = $1`, req.ID)
		database.Exec(`DELETE FROM user_group WHERE user_id = $1`, req.ID)
		database.Exec(`DELETE FROM user_oidc_role WHERE user_id = $1`, req.ID)
		database.Exec(`DELETE FROM "user" WHERE id = $1`, req.ID)
		database.LogAudit(c.Locals("user_id").(string), c.Locals("user_email").(string), "delete", "user", req.ID, "", c.IP())
		return c.JSON(fiber.Map{"ok": true})
	})
	admin.Post("/users/update", func(c *fiber.Ctx) error {
		var req struct {
			ID     string `json:"id"`
			Role   string `json:"role"`
			Banned bool   `json:"banned"`
		}
		c.BodyParser(&req)
		if req.Role != "" {
			database.Exec(`UPDATE "user" SET role = $1 WHERE id = $2`, req.Role, req.ID)
		}
		database.Exec(`UPDATE "user" SET banned = $1 WHERE id = $2`, req.Banned, req.ID)
		database.LogAudit(c.Locals("user_id").(string), c.Locals("user_email").(string), "update", "user", req.ID, fmt.Sprintf("role=%s banned=%v", req.Role, req.Banned), c.IP())
		return c.JSON(fiber.Map{"ok": true})
	})

	admin.Post("/users/bulk-delete", func(c *fiber.Ctx) error {
		var req struct {
			UserIDs []string `json:"user_ids"`
		}
		if err := c.BodyParser(&req); err != nil || len(req.UserIDs) == 0 {
			return c.Status(400).JSON(fiber.Map{"error": "user_ids required"})
		}
		deleted := 0
		for _, uid := range req.UserIDs {
			database.Exec(`DELETE FROM refresh_token WHERE user_id = $1`, uid)
			database.Exec(`DELETE FROM "account" WHERE "userId" = $1`, uid)
			database.Exec(`DELETE FROM user_group WHERE user_id = $1`, uid)
			database.Exec(`DELETE FROM user_oidc_role WHERE user_id = $1`, uid)
			database.Exec(`DELETE FROM "user" WHERE id = $1`, uid)
			deleted++
		}
		adminID, _ := c.Locals("user_id").(string)
		database.LogAudit(adminID, c.Locals("user_email").(string), "bulk_delete", "user", "", fmt.Sprintf("deleted %d users", deleted), c.IP())
		return c.JSON(fiber.Map{"ok": true, "deleted": deleted})
	})
	admin.Post("/users/bulk-update", func(c *fiber.Ctx) error {
		var req struct {
			UserIDs []string `json:"user_ids"`
			Role    string   `json:"role"`
			Banned  *bool    `json:"banned"`
		}
		if err := c.BodyParser(&req); err != nil || len(req.UserIDs) == 0 {
			return c.Status(400).JSON(fiber.Map{"error": "user_ids required"})
		}
		updated := 0
		for _, uid := range req.UserIDs {
			if req.Role != "" {
				database.Exec(`UPDATE "user" SET role = $1 WHERE id = $2`, req.Role, uid)
			}
			if req.Banned != nil {
				database.Exec(`UPDATE "user" SET banned = $1 WHERE id = $2`, *req.Banned, uid)
			}
			updated++
		}
		adminID, _ := c.Locals("user_id").(string)
		detail := fmt.Sprintf("updated %d users", updated)
		if req.Role != "" {
			detail += fmt.Sprintf(" role=%s", req.Role)
		}
		if req.Banned != nil {
			detail += fmt.Sprintf(" banned=%v", *req.Banned)
		}
		database.LogAudit(adminID, c.Locals("user_email").(string), "bulk_update", "user", "", detail, c.IP())
		return c.JSON(fiber.Map{"ok": true, "updated": updated})
	})
	admin.Post("/users/reset-password", func(c *fiber.Ctx) error {
		var req struct {
			ID          string `json:"id"`
			NewPassword string `json:"new_password"`
		}
		c.BodyParser(&req)
		if err := auth.ValidatePassword(req.NewPassword); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": err.Error()})
		}
		hash, err := auth.HashPassword(req.NewPassword)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Internal error"})
		}
		database.Exec(`UPDATE "account" SET password = $1, "updatedAt" = NOW() WHERE "userId" = $2 AND "providerId" = 'credential'`, hash, req.ID)
		// Invalidate all sessions for this user
		database.Exec(`DELETE FROM refresh_token WHERE user_id = $1`, req.ID)
		database.LogAudit(c.Locals("user_id").(string), c.Locals("user_email").(string), "reset_password", "user", req.ID, "admin reset", c.IP())
		return c.JSON(fiber.Map{"ok": true})
	})

	// Agent management
	agentHandler := &handlers.AgentHandler{DB: database}
	agentAPI.Post("/heartbeat", agentHandler.Heartbeat)
	agentAPI.Post("/destroy-session", func(c *fiber.Ctx) error {
		var req struct {
			SessionID string `json:"session_id"`
			UserID    string `json:"user_id"`
		}
		if err := c.BodyParser(&req); err != nil || req.SessionID == "" {
			return c.Status(400).JSON(fiber.Map{"error": "session_id required"})
		}
		// SECURITY: bind to the calling agent. Previously, any agent token could destroy
		// any session cluster-wide.
		callerAgentID, _ := c.Locals("agent_id").(string)
		if callerAgentID == "" {
			return c.Status(401).JSON(fiber.Map{"error": "Agent identity required"})
		}
		// Re-derive user_id from the session row; do not trust req.UserID for audit attribution.
		var sessionAgentID, sessionUserID string
		_ = database.QueryRow(`SELECT agent_id, COALESCE(user_id,'') FROM workspace_session WHERE id = $1`,
			req.SessionID).Scan(&sessionAgentID, &sessionUserID)
		if sessionAgentID == "" {
			// Already gone — idempotent success
			return c.JSON(fiber.Map{"status": "ok"})
		}
		if sessionAgentID != callerAgentID {
			log.Printf("[security] agent %s tried to destroy session %s owned by agent %s",
				callerAgentID, req.SessionID, sessionAgentID)
			return c.Status(403).JSON(fiber.Map{"error": "Session does not belong to this agent"})
		}
		database.DeleteSession(req.SessionID)
		database.LogAudit(sessionUserID, "", "destroy", "session", req.SessionID, "viewer", c.IP())
		log.Printf("Agent %s destroyed session %s (user=%s)", callerAgentID, req.SessionID, sessionUserID)
		return c.JSON(fiber.Map{"status": "ok"})
	})
	agentAPI.Post("/recording-uploaded", func(c *fiber.Ctx) error {
		var req struct {
			SessionID     string `json:"session_id"`
			S3Key         string `json:"s3_key"`
			FileSize      int64  `json:"file_size"`
			UserID        string `json:"user_id"`
			WorkspaceName string `json:"workspace_name"`
			StorageType   string `json:"storage_type"`
		}
		if err := c.BodyParser(&req); err != nil || req.SessionID == "" {
			return c.Status(400).JSON(fiber.Map{"error": "session_id required"})
		}
		if req.StorageType == "" {
			req.StorageType = "s3"
		}
		// SECURITY: verify the session belongs to the calling agent before recording metadata.
		// Otherwise any agent token could forge recording rows attributed to other users
		// (audit-log poisoning + s3_key smuggling that exfils admin-side S3 creds via download).
		callerAgentID, _ := c.Locals("agent_id").(string)
		if callerAgentID == "" {
			return c.Status(401).JSON(fiber.Map{"error": "Agent identity required"})
		}
		var sessionAgentID, sessionUserID, sessionWS string
		err := database.QueryRow(`SELECT ws.agent_id, COALESCE(ws.user_id,''), COALESCE(w.friendly_name,'')
			FROM workspace_session ws LEFT JOIN workspace w ON w.id = ws.workspace_id
			WHERE ws.id = $1`, req.SessionID).Scan(&sessionAgentID, &sessionUserID, &sessionWS)
		if err != nil || sessionAgentID == "" {
			return c.Status(404).JSON(fiber.Map{"error": "Session not found"})
		}
		if sessionAgentID != callerAgentID {
			log.Printf("[security] agent %s tried to record session %s owned by agent %s",
				callerAgentID, req.SessionID, sessionAgentID)
			return c.Status(403).JSON(fiber.Map{"error": "Session does not belong to this agent"})
		}
		// Use server-side values, not body-supplied (prevents user_id / workspace_name spoof).
		database.Exec(`INSERT INTO session_recording (session_id, user_id, workspace_name, s3_key, file_size, storage_type, agent_id) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			req.SessionID, sessionUserID, sessionWS, req.S3Key, req.FileSize, req.StorageType, callerAgentID)
		log.Printf("Recording saved for session %s: storage=%s (%d bytes)", req.SessionID, req.StorageType, req.FileSize)
		return c.JSON(fiber.Map{"status": "ok"})
	})

	// OIDC group sync — admin-only. Mounting on `authenticated` previously let
	// any low-priv user POST their user_id with crafted oidc_roles to self-promote.
	admin.Post("/sync-oidc-groups", func(c *fiber.Ctx) error {
		var req struct {
			UserID       string   `json:"user_id"`
			OIDCRoles    []string `json:"oidc_roles"`
			ProviderName string   `json:"provider_name"`
		}
		c.BodyParser(&req)
		database.SyncUserGroupsFromOIDC(req.UserID, req.OIDCRoles, req.ProviderName)
		return c.JSON(fiber.Map{"ok": true, "roles_synced": len(req.OIDCRoles)})
	})

	// Get all discovered OIDC roles
	admin.Post("/oidc-roles", func(c *fiber.Ctx) error {
		roles, _ := database.GetAllOIDCRoles()
		return c.JSON(fiber.Map{"roles": roles})
	})

	admin.Post("/agents", agentHandler.ListAgents)
	admin.Post("/agents/create", agentHandler.CreateAgent)
	admin.Post("/agents/delete", agentHandler.DeleteAgent)
	admin.Post("/agents/deploy-manifest", agentHandler.GetDeployManifest)
	admin.Post("/agents/deploy", agentHandler.DeployAgent)

	// Registry
	admin.Get("/registry", func(c *fiber.Ctx) error {
		category := c.Query("category", "all")
		items, _ := database.GetRegistry(category)
		cats, _ := database.GetRegistryCategories()
		return c.JSON(fiber.Map{"items": items, "categories": cats})
	})
	admin.Post("/registry/install", func(c *fiber.Ctx) error {
		var req struct {
			RegistryID string `json:"registry_id"`
		}
		c.BodyParser(&req)
		err := database.InstallFromRegistry(req.RegistryID)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"ok": true})
	})
	admin.Post("/registry/uninstall", func(c *fiber.Ctx) error {
		var req struct {
			RegistryID string `json:"registry_id"`
		}
		c.BodyParser(&req)
		database.UninstallFromRegistry(req.RegistryID)
		return c.JSON(fiber.Map{"ok": true})
	})

	// Settings (SMTP, etc.)
	admin.Post("/settings", func(c *fiber.Ctx) error {
		settings := database.GetSettings("")
		return c.JSON(settings)
	})
	admin.Post("/settings/update", func(c *fiber.Ctx) error {
		var req map[string]string
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
		}
		for k, v := range req {
			database.SetSetting(k, v)
		}
		appCache.Invalidate("settings")
		database.LogAudit(
			c.Locals("user_id").(string),
			c.Locals("user_email").(string),
			"update", "settings", "", "", c.IP(),
		)
		return c.JSON(fiber.Map{"ok": true})
	})

	// S3 Storage settings
	admin.Post("/settings/s3", func(c *fiber.Ctx) error {
		s3Settings := database.GetSettings("s3.")
		// Mask secret key
		if _, ok := s3Settings["s3.secret_key"]; ok && s3Settings["s3.secret_key"] != "" {
			s3Settings["s3.secret_key"] = "••••••"
		}
		return c.JSON(s3Settings)
	})
	admin.Post("/settings/s3/save", func(c *fiber.Ctx) error {
		var req struct {
			Endpoint  string `json:"endpoint"`
			AccessKey string `json:"access_key"`
			SecretKey string `json:"secret_key"`
			Bucket    string `json:"bucket"`
			Region    string `json:"region"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
		}
		database.SetSetting("s3.endpoint", req.Endpoint)
		database.SetSetting("s3.access_key", req.AccessKey)
		if req.SecretKey != "••••••" {
			database.SetSetting("s3.secret_key", req.SecretKey)
		}
		database.SetSetting("s3.bucket", req.Bucket)
		database.SetSetting("s3.region", req.Region)
		appCache.Invalidate("settings")
		database.LogAudit(c.Locals("user_id").(string), c.Locals("user_email").(string), "update", "s3_settings", "", "", c.IP())
		return c.JSON(fiber.Map{"ok": true})
	})

	// Session recordings (supports both cursor and offset pagination)
	admin.Post("/recordings", func(c *fiber.Ctx) error {
		var req struct {
			Page    int    `json:"page"`
			PerPage int    `json:"per_page"`
			Cursor  string `json:"cursor"`
		}
		c.BodyParser(&req)
		if req.PerPage == 0 {
			req.PerPage = 50
		}

		type Recording struct {
			ID              string `db:"id" json:"id"`
			SessionID       string `db:"session_id" json:"session_id"`
			UserID          string `db:"user_id" json:"user_id"`
			WorkspaceName   string `db:"workspace_name" json:"workspace_name"`
			S3Key           string `db:"s3_key" json:"s3_key"`
			FileSize        int64  `db:"file_size" json:"file_size"`
			DurationSeconds int    `db:"duration_seconds" json:"duration_seconds"`
			StorageType     string `db:"storage_type" json:"storage_type"`
			AgentID         string `db:"agent_id" json:"agent_id"`
			CreatedAt       string `db:"created_at" json:"created_at"`
		}
		type RecordingWithUser struct {
			Recording
			UserEmail string `json:"user_email"`
		}
		enrichWithEmail := func(recs []Recording) []RecordingWithUser {
			out := make([]RecordingWithUser, len(recs))
			for i, r := range recs {
				var email string
				database.Get(&email, `SELECT COALESCE(email,'') FROM "user" WHERE id = `, r.UserID)
				out[i] = RecordingWithUser{Recording: r, UserEmail: email}
			}
			return out
		}

		// Cursor-based pagination
		if req.Cursor != "" {
			cursorTime, err := time.Parse(time.RFC3339Nano, req.Cursor)
			if err != nil {
				return c.Status(400).JSON(fiber.Map{"error": "invalid cursor"})
			}
			var recordings []Recording
			database.Select(&recordings, `SELECT id, session_id, COALESCE(user_id,'') as user_id, COALESCE(workspace_name,'') as workspace_name, COALESCE(s3_key,'') as s3_key, COALESCE(file_size,0) as file_size, COALESCE(duration_seconds,0) as duration_seconds, COALESCE(storage_type,'s3') as storage_type, COALESCE(agent_id,'') as agent_id, created_at FROM session_recording WHERE created_at <  ORDER BY created_at DESC LIMIT `, cursorTime, req.PerPage+1)
			if recordings == nil {
				recordings = []Recording{}
			}
			var nextCursor string
			if len(recordings) > req.PerPage {
				nextCursor = recordings[req.PerPage-1].CreatedAt
				recordings = recordings[:req.PerPage]
			}
			resp := fiber.Map{"recordings": enrichWithEmail(recordings), "per_page": req.PerPage}
			if nextCursor != "" {
				resp["next_cursor"] = nextCursor
			}
			return c.JSON(resp)
		}

		// Offset-based pagination (backward compat)
		if req.Page == 0 {
			req.Page = 1
		}
		offset := (req.Page - 1) * req.PerPage

		var total int
		database.Get(&total, `SELECT COUNT(*) FROM session_recording`)

		var recordings []Recording
		database.Select(&recordings, `SELECT id, session_id, COALESCE(user_id,'') as user_id, COALESCE(workspace_name,'') as workspace_name, COALESCE(s3_key,'') as s3_key, COALESCE(file_size,0) as file_size, COALESCE(duration_seconds,0) as duration_seconds, COALESCE(storage_type,'s3') as storage_type, COALESCE(agent_id,'') as agent_id, created_at FROM session_recording ORDER BY created_at DESC LIMIT  OFFSET `, req.PerPage, offset)
		if recordings == nil {
			recordings = []Recording{}
		}

		return c.JSON(fiber.Map{"recordings": enrichWithEmail(recordings), "total": total, "page": req.Page, "per_page": req.PerPage})
	})

	admin.Get("/recordings/:id/download", func(c *fiber.Ctx) error {
		recordingID := c.Params("id")
		type RecInfo struct {
			SessionID   string `db:"session_id"`
			S3Key       string `db:"s3_key"`
			StorageType string `db:"storage_type"`
			AgentID     string `db:"agent_id"`
		}
		var rec RecInfo
		err := database.Get(&rec, `SELECT session_id, COALESCE(s3_key,'') as s3_key, COALESCE(storage_type,'s3') as storage_type, COALESCE(agent_id,'') as agent_id FROM session_recording WHERE id = $1`, recordingID)
		if err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "Recording not found"})
		}

		if rec.StorageType == "local" && rec.AgentID != "" {
			// Proxy download from the agent
			var endpoint, token string
			database.QueryRow(`SELECT endpoint, token FROM agent WHERE id = $1`, rec.AgentID).Scan(&endpoint, &token)
			if endpoint == "" {
				return c.Status(404).JSON(fiber.Map{"error": "Agent not available"})
			}
			agentURL := fmt.Sprintf("%s/api/recordings/%s/download", strings.TrimRight(endpoint, "/"), rec.SessionID)
			proxyReq, _ := http.NewRequest("GET", agentURL, nil)
			proxyReq.Header.Set("X-Agent-Token", token)
			resp, err := (&http.Client{Timeout: 2 * time.Minute}).Do(proxyReq)
			if err != nil {
				return c.Status(502).JSON(fiber.Map{"error": "Failed to reach agent"})
			}
			defer resp.Body.Close()
			c.Set("Content-Type", "application/octet-stream")
			c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.guac\"", rec.SessionID))
			data, _ := io.ReadAll(resp.Body)
			return c.Send(data)
		}

		// S3 storage — return info for client-side download
		if rec.S3Key == "" {
			return c.Status(404).JSON(fiber.Map{"error": "Recording not found"})
		}
		s3Settings := database.GetSettings("s3.")
		return c.JSON(fiber.Map{
			"s3_key":       rec.S3Key,
			"endpoint":     s3Settings["s3.endpoint"],
			"bucket":       s3Settings["s3.bucket"],
			"region":       s3Settings["s3.region"],
			"access_key":   s3Settings["s3.access_key"],
			"storage_type": "s3",
		})
	})

	// Auth methods
	authMethodHandler := &handlers.AuthMethodHandler{DB: database}
	admin.Post("/auth-methods", authMethodHandler.List)
	admin.Post("/auth-methods/create", authMethodHandler.Create)
	admin.Post("/auth-methods/update", authMethodHandler.Update)
	admin.Post("/auth-methods/delete", authMethodHandler.Delete)
	admin.Post("/auth-methods/toggle", authMethodHandler.Toggle)

	// Guest links management
	admin.Post("/guest-links", func(c *fiber.Ctx) error {
		var req struct {
			Page    int `json:"page"`
			PerPage int `json:"per_page"`
		}
		c.BodyParser(&req)
		if req.PerPage == 0 {
			req.PerPage = 50
		}
		if req.Page == 0 {
			req.Page = 1
		}
		offset := (req.Page - 1) * req.PerPage

		var total int
		database.Get(&total, `SELECT COUNT(*) FROM guest_link`)

		type GuestLinkRow struct {
			ID            string `db:"id" json:"id"`
			WorkspaceID   string `db:"workspace_id" json:"workspace_id"`
			WorkspaceName string `db:"workspace_name" json:"workspace_name"`
			Token         string `db:"token" json:"token"`
			CreatedBy     string `db:"created_by" json:"created_by"`
			Label         string `db:"label" json:"label"`
			HasPassword   bool   `json:"has_password"`
			PasswordHash  string `db:"password_hash" json:"-"`
			MaxUses       int    `db:"max_uses" json:"max_uses"`
			UsedCount     int    `db:"used_count" json:"used_count"`
			ExpiresAt     string `db:"expires_at" json:"expires_at"`
			CreatedAt     string `db:"created_at" json:"created_at"`
		}
		var links []GuestLinkRow
		database.Select(&links, `SELECT gl.id, gl.workspace_id, COALESCE(w.friendly_name,'') as workspace_name, gl.token, COALESCE(gl.created_by,'') as created_by, COALESCE(gl.label,'') as label, COALESCE(gl.password_hash,'') as password_hash, COALESCE(gl.max_uses,1) as max_uses, COALESCE(gl.used_count,0) as used_count, gl.expires_at, gl.created_at
			FROM guest_link gl LEFT JOIN workspace w ON w.id = gl.workspace_id
			ORDER BY gl.created_at DESC LIMIT $1 OFFSET $2`, req.PerPage, offset)
		if links == nil {
			links = []GuestLinkRow{}
		}
		for i := range links {
			links[i].HasPassword = links[i].PasswordHash != ""
		}
		return c.JSON(fiber.Map{"links": links, "total": total, "page": req.Page, "per_page": req.PerPage})
	})
	admin.Post("/guest-links/create", func(c *fiber.Ctx) error {
		var req struct {
			WorkspaceID string `json:"workspace_id"`
			Label       string `json:"label"`
			Password    string `json:"password"`
			MaxUses     int    `json:"max_uses"`
			Duration    string `json:"duration"` // "1h", "8h", "24h", "7d", "30d"
		}
		if err := c.BodyParser(&req); err != nil || req.WorkspaceID == "" {
			return c.Status(400).JSON(fiber.Map{"error": "workspace_id required"})
		}
		var dur time.Duration
		switch req.Duration {
		case "1h":
			dur = 1 * time.Hour
		case "8h":
			dur = 8 * time.Hour
		case "24h":
			dur = 24 * time.Hour
		case "7d":
			dur = 7 * 24 * time.Hour
		case "30d":
			dur = 30 * 24 * time.Hour
		default:
			dur = 24 * time.Hour
		}
		expiresAt := time.Now().Add(dur)
		var passwordHash string
		if req.Password != "" {
			hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
			if err != nil {
				return c.Status(500).JSON(fiber.Map{"error": "Failed to hash password"})
			}
			passwordHash = string(hash)
		}
		if req.MaxUses == 0 {
			req.MaxUses = 1
		}
		userID, _ := c.Locals("user_id").(string)
		var token string
		err := database.QueryRow(`INSERT INTO guest_link (workspace_id, created_by, label, password_hash, max_uses, expires_at) VALUES ($1, $2, $3, $4, $5, $6) RETURNING token`,
			req.WorkspaceID, userID, req.Label, passwordHash, req.MaxUses, expiresAt).Scan(&token)
		if err != nil {
			log.Printf("CreateGuestLink error: %v", err)
			return c.Status(500).JSON(fiber.Map{"error": "Failed to create guest link"})
		}
		database.LogAudit(userID, "", "create", "guest_link", token, req.Label, c.IP())
		return c.JSON(fiber.Map{"ok": true, "token": token})
	})
	admin.Post("/guest-links/delete", func(c *fiber.Ctx) error {
		var req struct {
			ID string `json:"id"`
		}
		c.BodyParser(&req)
		database.Exec(`DELETE FROM guest_link WHERE id = $1`, req.ID)
		database.LogAudit(c.Locals("user_id").(string), "", "delete", "guest_link", req.ID, "", c.IP())
		return c.JSON(fiber.Map{"ok": true})
	})

	// Backup/Restore
	admin.Post("/backup/export", func(c *fiber.Ctx) error {
		workspaces, _ := database.GetAllWorkspaces()
		groups, _ := database.GetGroups()
		var b Branding
		database.Get(&b, `SELECT app_name, COALESCE(logo_url,'') as logo_url, COALESCE(logo_dark_url,'') as logo_dark_url, COALESCE(favicon_url,'') as favicon_url, COALESCE(creator,'') as creator, COALESCE(creator_url,'') as creator_url, COALESCE(primary_color,'#4F46E5') as primary_color, COALESCE(accent_color,'#F97316') as accent_color, COALESCE(login_bg,'') as login_bg FROM branding WHERE id = 'default'`)
		settings := database.GetSettings("")
		maskedSettings := make(map[string]string)
		for k, v := range settings {
			if strings.Contains(k, "secret") || strings.Contains(k, "password") {
				if v != "" {
					maskedSettings[k] = "••••••"
				} else {
					maskedSettings[k] = ""
				}
			} else {
				maskedSettings[k] = v
			}
		}
		type AuthMethod struct {
			ID      string          `db:"id" json:"id"`
			Name    string          `db:"name" json:"name"`
			Type    string          `db:"type" json:"type"`
			Enabled bool            `db:"enabled" json:"enabled"`
			Config  json.RawMessage `db:"config" json:"config"`
		}
		var authMethods []AuthMethod
		database.Select(&authMethods, `SELECT id, name, type, enabled, config FROM auth_method ORDER BY name`)
		if authMethods == nil {
			authMethods = []AuthMethod{}
		}
		type GuestLinkExport struct {
			WorkspaceName string `db:"workspace_name" json:"workspace_name"`
			Label         string `db:"label" json:"label"`
			MaxUses       int    `db:"max_uses" json:"max_uses"`
			UsedCount     int    `db:"used_count" json:"used_count"`
			ExpiresAt     string `db:"expires_at" json:"expires_at"`
		}
		var guestLinks []GuestLinkExport
		database.Select(&guestLinks, `SELECT COALESCE(w.friendly_name,'') as workspace_name, COALESCE(gl.label,'') as label, COALESCE(gl.max_uses,1) as max_uses, COALESCE(gl.used_count,0) as used_count, gl.expires_at FROM guest_link gl LEFT JOIN workspace w ON w.id = gl.workspace_id ORDER BY gl.created_at DESC`)
		if guestLinks == nil {
			guestLinks = []GuestLinkExport{}
		}
		type WGAssoc struct {
			WorkspaceName string   `json:"workspace_name"`
			GroupNames    []string `json:"group_names"`
		}
		var wgAssocs []WGAssoc
		for _, w := range workspaces {
			wGroups, _ := database.GetWorkspaceGroups(w.ID)
			names := make([]string, len(wGroups))
			for i, g := range wGroups {
				names[i] = g.Name
			}
			if len(names) > 0 {
				wgAssocs = append(wgAssocs, WGAssoc{WorkspaceName: w.FriendlyName, GroupNames: names})
			}
		}
		if wgAssocs == nil {
			wgAssocs = []WGAssoc{}
		}
		type ExportWorkspace struct {
			Name                  string  `json:"name"`
			FriendlyName          string  `json:"friendly_name"`
			Description           string  `json:"description"`
			ImageSrc              string  `json:"image_src"`
			DockerImage           string  `json:"docker_image"`
			Cores                 float64 `json:"cores"`
			Memory                int64   `json:"memory"`
			SHMSize               string  `json:"shm_size"`
			Enabled               bool    `json:"enabled"`
			Category              string  `json:"category"`
			DockerRegistry        string  `json:"docker_registry"`
			DockerUser            string  `json:"docker_user"`
			SessionTimeLimit      int     `json:"session_time_limit"`
			GPUCount              int     `json:"gpu_count"`
			RestrictToRegion      string  `json:"restrict_to_region"`
			RunConfig             string  `json:"run_config"`
			ExecConfig            string  `json:"exec_config"`
			VolumeMappings        string  `json:"volume_mappings"`
			Persistent            bool    `json:"persistent"`
			PersistentSize        string  `json:"persistent_size"`
			WorkspaceType         string  `json:"workspace_type"`
			ServerHostname        string  `json:"server_hostname"`
			ServerPort            int     `json:"server_port"`
			ServerProtocol        string  `json:"server_protocol"`
			ServerDomain          string  `json:"server_domain"`
			ServerIgnoreCert      bool    `json:"server_ignore_cert"`
			ServerSecurity        string  `json:"server_security"`
			ServerAuthMode        string  `json:"server_auth_mode"`
			ServerAllowRemember   bool    `json:"server_allow_remember"`
			ServerDefaultSettings string  `json:"server_default_settings"`
			RecordSessions        bool    `json:"record_sessions"`
		}
		exportWorkspaces := make([]ExportWorkspace, len(workspaces))
		for i, w := range workspaces {
			desc := ""
			if w.Description != nil {
				desc = *w.Description
			}
			exportWorkspaces[i] = ExportWorkspace{
				Name: w.Name, FriendlyName: w.FriendlyName, Description: desc, ImageSrc: w.ImageSrc, DockerImage: w.DockerImage,
				Cores: w.Cores, Memory: w.Memory, SHMSize: w.SHMSize, Enabled: w.Enabled, Category: w.Category,
				DockerRegistry: w.DockerRegistry, DockerUser: w.DockerUser, SessionTimeLimit: w.SessionTimeLimit, GPUCount: w.GPUCount,
				RestrictToRegion: w.RestrictToRegion, RunConfig: w.RunConfig, ExecConfig: w.ExecConfig, VolumeMappings: w.VolumeMappings,
				Persistent: w.Persistent, PersistentSize: w.PersistentSize, WorkspaceType: w.WorkspaceType,
				ServerHostname: w.ServerHostname, ServerPort: w.ServerPort, ServerProtocol: w.ServerProtocol,
				ServerDomain: w.ServerDomain, ServerIgnoreCert: w.ServerIgnoreCert, ServerSecurity: w.ServerSecurity,
				ServerAuthMode: w.ServerAuthMode, ServerAllowRemember: w.ServerAllowRemember,
				ServerDefaultSettings: w.ServerDefaultSettings, RecordSessions: w.RecordSessions,
			}
		}
		export := fiber.Map{
			"version": Version, "exported_at": time.Now().Format(time.RFC3339),
			"workspaces": exportWorkspaces, "groups": groups, "branding": b, "settings": maskedSettings,
			"auth_methods": authMethods, "guest_links": guestLinks, "workspace_groups": wgAssocs,
		}
		database.LogAudit(c.Locals("user_id").(string), "", "export", "backup", "", "", c.IP())
		c.Set("Content-Disposition", "attachment; filename=oklavier-backup.json")
		return c.JSON(export)
	})

	admin.Post("/backup/import", func(c *fiber.Ctx) error {
		var body map[string]json.RawMessage
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid JSON"})
		}
		summary := fiber.Map{}
		// Import workspaces
		if raw, ok := body["workspaces"]; ok {
			var workspaces []struct {
				Name                  string  `json:"name"`
				FriendlyName          string  `json:"friendly_name"`
				Description           string  `json:"description"`
				ImageSrc              string  `json:"image_src"`
				DockerImage           string  `json:"docker_image"`
				Cores                 float64 `json:"cores"`
				Memory                int64   `json:"memory"`
				SHMSize               string  `json:"shm_size"`
				Enabled               bool    `json:"enabled"`
				Category              string  `json:"category"`
				DockerRegistry        string  `json:"docker_registry"`
				DockerUser            string  `json:"docker_user"`
				SessionTimeLimit      int     `json:"session_time_limit"`
				GPUCount              int     `json:"gpu_count"`
				RestrictToRegion      string  `json:"restrict_to_region"`
				RunConfig             string  `json:"run_config"`
				ExecConfig            string  `json:"exec_config"`
				VolumeMappings        string  `json:"volume_mappings"`
				Persistent            bool    `json:"persistent"`
				PersistentSize        string  `json:"persistent_size"`
				WorkspaceType         string  `json:"workspace_type"`
				ServerHostname        string  `json:"server_hostname"`
				ServerPort            int     `json:"server_port"`
				ServerProtocol        string  `json:"server_protocol"`
				ServerDomain          string  `json:"server_domain"`
				ServerIgnoreCert      bool    `json:"server_ignore_cert"`
				ServerSecurity        string  `json:"server_security"`
				ServerAuthMode        string  `json:"server_auth_mode"`
				ServerAllowRemember   bool    `json:"server_allow_remember"`
				ServerDefaultSettings string  `json:"server_default_settings"`
				RecordSessions        bool    `json:"record_sessions"`
			}
			json.Unmarshal(raw, &workspaces)
			created, updated := 0, 0
			for _, w := range workspaces {
				var existingID string
				database.Get(&existingID, `SELECT id FROM workspace WHERE name = $1`, w.Name)
				wType := w.WorkspaceType
				if wType == "" {
					wType = "container"
				}
				rc := w.RunConfig
				if rc == "" {
					rc = "{}"
				}
				ec := w.ExecConfig
				if ec == "" {
					ec = "{}"
				}
				vm := w.VolumeMappings
				if vm == "" {
					vm = "{}"
				}
				sh := w.SHMSize
				if sh == "" {
					sh = "512m"
				}
				sds := w.ServerDefaultSettings
				if sds == "" {
					sds = "{}"
				}
				if existingID != "" {
					database.Exec(`UPDATE workspace SET friendly_name=$2, description=$3, image_src=$4, docker_image=$5, cores=$6, memory=$7, category=$8, docker_registry=$9, docker_user=$10, session_time_limit=$11, gpu_count=$12, restrict_to_region=$13, run_config=$14, exec_config=$15, volume_mappings=$16, persistent=$17, persistent_size=$18, workspace_type=$19, server_hostname=$20, server_port=$21, server_protocol=$22, server_domain=$23, server_ignore_cert=$24, server_security=$25, server_auth_mode=$26, server_allow_remember=$27, server_default_settings=$28, record_sessions=$29, shm_size=$30, enabled=$31, updated_at=NOW() WHERE id=$1`,
						existingID, w.FriendlyName, w.Description, w.ImageSrc, w.DockerImage, w.Cores, w.Memory, w.Category, w.DockerRegistry, w.DockerUser, w.SessionTimeLimit, w.GPUCount, w.RestrictToRegion, rc, ec, vm, w.Persistent, w.PersistentSize, wType, w.ServerHostname, w.ServerPort, w.ServerProtocol, w.ServerDomain, w.ServerIgnoreCert, w.ServerSecurity, w.ServerAuthMode, w.ServerAllowRemember, sds, w.RecordSessions, sh, w.Enabled)
					updated++
				} else {
					database.Exec(`INSERT INTO workspace (name, friendly_name, description, image_src, docker_image, cores, memory, category, docker_registry, docker_user, session_time_limit, gpu_count, restrict_to_region, run_config, exec_config, volume_mappings, persistent, persistent_size, workspace_type, server_hostname, server_port, server_protocol, server_domain, server_ignore_cert, server_security, server_auth_mode, server_allow_remember, server_default_settings, record_sessions, shm_size, enabled) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28,$29,$30,$31)`,
						w.Name, w.FriendlyName, w.Description, w.ImageSrc, w.DockerImage, w.Cores, w.Memory, w.Category, w.DockerRegistry, w.DockerUser, w.SessionTimeLimit, w.GPUCount, w.RestrictToRegion, rc, ec, vm, w.Persistent, w.PersistentSize, wType, w.ServerHostname, w.ServerPort, w.ServerProtocol, w.ServerDomain, w.ServerIgnoreCert, w.ServerSecurity, w.ServerAuthMode, w.ServerAllowRemember, sds, w.RecordSessions, sh, w.Enabled)
					created++
				}
			}
			summary["workspaces"] = fiber.Map{"created": created, "updated": updated}
		}
		// Import groups
		if raw, ok := body["groups"]; ok {
			var groups []struct {
				Name        string  `json:"name"`
				Description string  `json:"description"`
				Color       string  `json:"color"`
				MaxSessions int     `json:"max_sessions"`
				MaxCPU      float64 `json:"max_cpu"`
				MaxMemory   int64   `json:"max_memory"`
			}
			json.Unmarshal(raw, &groups)
			created, updated := 0, 0
			for _, g := range groups {
				var existingID string
				database.Get(&existingID, `SELECT id FROM oklavier_group WHERE name = $1`, g.Name)
				color := g.Color
				if color == "" {
					color = "#6366f1"
				}
				if existingID != "" {
					database.Exec(`UPDATE oklavier_group SET description=$2, color=$3, max_sessions=$4, max_cpu=$5, max_memory=$6 WHERE id=$1`, existingID, g.Description, color, g.MaxSessions, g.MaxCPU, g.MaxMemory)
					updated++
				} else {
					database.CreateGroup(g.Name, g.Description, color, g.MaxSessions, g.MaxCPU, g.MaxMemory)
					created++
				}
			}
			summary["groups"] = fiber.Map{"created": created, "updated": updated}
		}
		// Import branding
		if raw, ok := body["branding"]; ok {
			var b Branding
			json.Unmarshal(raw, &b)
			if b.AppName != "" {
				database.Exec(`UPDATE branding SET app_name=$1, logo_url=$2, logo_dark_url=$3, favicon_url=$4, creator=$5, creator_url=$6, primary_color=$7, accent_color=$8, login_bg=$9, updated_at=NOW() WHERE id='default'`,
					b.AppName, b.LogoURL, b.LogoDarkURL, b.FaviconURL, b.Creator, b.CreatorURL, b.PrimaryColor, b.AccentColor, b.LoginBG)
				appCache.Delete("branding")
				summary["branding"] = "imported"
			}
		}
		// Import settings (skip masked)
		if raw, ok := body["settings"]; ok {
			var settings map[string]string
			json.Unmarshal(raw, &settings)
			imported := 0
			for k, v := range settings {
				if v == "••••••" {
					continue
				}
				database.SetSetting(k, v)
				imported++
			}
			summary["settings"] = fiber.Map{"imported": imported}
		}
		// Import auth methods
		if raw, ok := body["auth_methods"]; ok {
			var methods []struct {
				Name    string          `json:"name"`
				Type    string          `json:"type"`
				Enabled bool            `json:"enabled"`
				Config  json.RawMessage `json:"config"`
			}
			json.Unmarshal(raw, &methods)
			created, updated := 0, 0
			for _, m := range methods {
				var existingID string
				database.Get(&existingID, `SELECT id FROM auth_method WHERE name = $1`, m.Name)
				if existingID != "" {
					database.Exec(`UPDATE auth_method SET type=$2, enabled=$3, config=$4 WHERE id=$1`, existingID, m.Type, m.Enabled, m.Config)
					updated++
				} else {
					database.Exec(`INSERT INTO auth_method (name, type, enabled, config) VALUES ($1, $2, $3, $4)`, m.Name, m.Type, m.Enabled, m.Config)
					created++
				}
			}
			summary["auth_methods"] = fiber.Map{"created": created, "updated": updated}
		}
		// Import workspace-group associations
		if raw, ok := body["workspace_groups"]; ok {
			var assocs []struct {
				WorkspaceName string   `json:"workspace_name"`
				GroupNames    []string `json:"group_names"`
			}
			json.Unmarshal(raw, &assocs)
			linked := 0
			for _, a := range assocs {
				var wID string
				database.Get(&wID, `SELECT id FROM workspace WHERE friendly_name = $1`, a.WorkspaceName)
				if wID == "" {
					continue
				}
				var groupIDs []string
				for _, gn := range a.GroupNames {
					var gID string
					database.Get(&gID, `SELECT id FROM oklavier_group WHERE name = $1`, gn)
					if gID != "" {
						groupIDs = append(groupIDs, gID)
					}
				}
				if len(groupIDs) > 0 {
					database.SetWorkspaceGroups(wID, groupIDs)
					linked++
				}
			}
			summary["workspace_groups"] = fiber.Map{"linked": linked}
		}
		database.LogAudit(c.Locals("user_id").(string), "", "import", "backup", "", fmt.Sprintf("%v", summary), c.IP())
		return c.JSON(fiber.Map{"ok": true, "summary": summary})
	})

	// Clusters (stub endpoints - cluster table to be added later)
	admin.Post("/clusters", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"clusters": []interface{}{}}) })
	admin.Post("/clusters/create", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"ok": true}) })
	admin.Post("/clusters/update", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"ok": true}) })
	admin.Post("/clusters/delete", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"ok": true}) })
	admin.Post("/clusters/toggle-default", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"ok": true}) })
	admin.Post("/clusters/test", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"ok": true}) })

	// Analytics (optimized: parameterized cutoffs, ::date cast for index usage)
	admin.Post("/analytics", func(c *fiber.Ctx) error {
		cutoff30 := time.Now().AddDate(0, 0, -30)
		cutoff7 := time.Now().AddDate(0, 0, -7)

		type DayStat struct {
			Date  string `db:"date" json:"date"`
			Count int    `db:"count" json:"count"`
		}
		var sessionsPerDay []DayStat
		database.Select(&sessionsPerDay, `
			SELECT created_at::date as date, COUNT(*) as count
			FROM workspace_session
			WHERE created_at > $1
			GROUP BY created_at::date
			ORDER BY date
		`, cutoff30)
		if sessionsPerDay == nil {
			sessionsPerDay = []DayStat{}
		}

		type WorkspaceStat struct {
			Name  string `db:"name" json:"name"`
			Count int    `db:"count" json:"count"`
		}
		var topWorkspaces []WorkspaceStat
		database.Select(&topWorkspaces, `
			SELECT w.friendly_name as name, COUNT(*) as count
			FROM workspace_session ws
			JOIN workspace w ON w.id = ws.workspace_id
			WHERE ws.created_at > $1
			GROUP BY w.friendly_name
			ORDER BY count DESC
			LIMIT 10
		`, cutoff30)
		if topWorkspaces == nil {
			topWorkspaces = []WorkspaceStat{}
		}

		type HourStat struct {
			Hour  int `db:"hour" json:"hour"`
			Count int `db:"count" json:"count"`
		}
		var peakHours []HourStat
		database.Select(&peakHours, `
			SELECT EXTRACT(HOUR FROM created_at)::int as hour, COUNT(*) as count
			FROM workspace_session
			WHERE created_at > $1
			GROUP BY hour
			ORDER BY hour
		`, cutoff30)
		if peakHours == nil {
			peakHours = []HourStat{}
		}

		var avgDuration float64
		database.Get(&avgDuration, `
			SELECT COALESCE(AVG(EXTRACT(EPOCH FROM (COALESCE(updated_at, NOW()) - created_at)) / 60), 0)
			FROM workspace_session
			WHERE created_at > $1 AND status = 'deleted'
		`, cutoff30)

		var activeUsersWeek int
		database.Get(&activeUsersWeek, `
			SELECT COUNT(DISTINCT user_id)
			FROM workspace_session
			WHERE created_at > $1
		`, cutoff7)

		var totalSessionsMonth int
		database.Get(&totalSessionsMonth, `
			SELECT COUNT(*) FROM workspace_session
			WHERE created_at > $1
		`, cutoff30)

		return c.JSON(fiber.Map{
			"sessions_per_day":     sessionsPerDay,
			"top_workspaces":       topWorkspaces,
			"peak_hours":           peakHours,
			"avg_duration_minutes": avgDuration,
			"active_users_week":    activeUsersWeek,
			"total_sessions_month": totalSessionsMonth,
		})
	})

	// Audit log (supports both cursor and offset pagination)
	admin.Post("/audit-log", func(c *fiber.Ctx) error {
		var req struct {
			Page         int    `json:"page"`
			PerPage      int    `json:"per_page"`
			Cursor       string `json:"cursor"`
			ResourceType string `json:"resource_type"`
			Action       string `json:"action"`
		}
		c.BodyParser(&req)
		if req.PerPage == 0 {
			req.PerPage = 50
		}

		// Cursor-based pagination when cursor is provided
		if req.Cursor != "" {
			result := database.GetAuditLogCursor(req.Cursor, req.PerPage, req.ResourceType, req.Action)
			return c.JSON(result)
		}

		result := database.GetAuditLog(req.Page, req.PerPage, req.ResourceType, req.Action)
		return c.JSON(result)
	})

	// Audit log CSV export
	admin.Post("/audit/export", func(c *fiber.Ctx) error {
		var req struct {
			ResourceType string `json:"resource_type"`
			Action       string `json:"action"`
		}
		c.BodyParser(&req)
		result := database.GetAuditLog(1, 100000, req.ResourceType, req.Action)

		var buf bytes.Buffer
		w := csv.NewWriter(&buf)
		w.Write([]string{"Date", "User", "Action", "Resource Type", "Resource ID", "Details", "IP Address"})
		for _, e := range result.Entries {
			w.Write([]string{
				e.CreatedAt.Format(time.RFC3339),
				e.UserEmail,
				e.Action,
				e.ResourceType,
				e.ResourceID,
				e.Details,
				e.IPAddress,
			})
		}
		w.Flush()

		c.Set("Content-Type", "text/csv")
		c.Set("Content-Disposition", "attachment; filename=audit-log.csv")
		return c.Send(buf.Bytes())
	})

	// Branding (admin)
	admin.Post("/branding", func(c *fiber.Ctx) error {
		var b Branding
		err := database.Get(&b, `SELECT app_name, COALESCE(logo_url,'') as logo_url, COALESCE(logo_dark_url,'') as logo_dark_url, COALESCE(favicon_url,'') as favicon_url, COALESCE(creator,'') as creator, COALESCE(creator_url,'') as creator_url, COALESCE(primary_color,'#4F46E5') as primary_color, COALESCE(accent_color,'#F97316') as accent_color, COALESCE(login_bg,'') as login_bg FROM branding WHERE id = 'default'`)
		if err != nil {
			b = Branding{AppName: "Oklavier", PrimaryColor: "#4F46E5", AccentColor: "#F97316"}
		}
		return c.JSON(b)
	})
	admin.Post("/branding/update", func(c *fiber.Ctx) error {
		var req struct {
			AppName      string `json:"app_name"`
			LogoURL      string `json:"logo_url"`
			LogoDarkURL  string `json:"logo_dark_url"`
			FaviconURL   string `json:"favicon_url"`
			Creator      string `json:"creator"`
			CreatorURL   string `json:"creator_url"`
			PrimaryColor string `json:"primary_color"`
			AccentColor  string `json:"accent_color"`
			LoginBG      string `json:"login_bg"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
		}
		database.Exec(`UPDATE branding SET app_name=$1, logo_url=$2, logo_dark_url=$3, favicon_url=$4, creator=$5, creator_url=$6, primary_color=$7, accent_color=$8, login_bg=$9, updated_at=NOW() WHERE id='default'`,
			req.AppName, req.LogoURL, req.LogoDarkURL, req.FaviconURL, req.Creator, req.CreatorURL, req.PrimaryColor, req.AccentColor, req.LoginBG)
		appCache.Delete("branding")
		database.LogAudit(c.Locals("user_id").(string), c.Locals("user_email").(string), "update", "branding", "default", req.AppName, c.IP())
		return c.JSON(fiber.Map{"ok": true})
	})

	// File upload (images for branding)
	admin.Post("/upload", func(c *fiber.Ctx) error {
		file, err := c.FormFile("file")
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "No file uploaded"})
		}

		// Validate size (max 500KB)
		if file.Size > 500*1024 {
			return c.Status(400).JSON(fiber.Map{"error": "File too large (max 500KB)"})
		}

		// Validate type
		contentType := file.Header.Get("Content-Type")
		allowed := map[string]string{
			"image/svg+xml": "svg",
			"image/png":     "png",
			"image/jpeg":    "jpg",
			"image/webp":    "webp",
			"image/x-icon":  "ico",
		}
		if _, ok := allowed[contentType]; !ok {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid file type. Allowed: SVG, PNG, JPG, WEBP"})
		}

		// Read file
		f, err := file.Open()
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to read file"})
		}
		defer f.Close()

		data, err := io.ReadAll(f)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to read file"})
		}

		// Convert to base64 data URI
		b64 := base64.StdEncoding.EncodeToString(data)
		dataURI := fmt.Sprintf("data:%s;base64,%s", contentType, b64)

		return c.JSON(fiber.Map{"url": dataURI})
	})

	// Logs
	logHandler := &handlers.LogHandler{DB: database}
	handlers.InstallLogCapture()
	admin.Get("/logs", logHandler.GetLogs)

	// Groups
	admin.Post("/groups", func(c *fiber.Ctx) error {
		var req struct {
			Page    int    `json:"page"`
			PerPage int    `json:"per_page"`
			Search  string `json:"search"`
		}
		c.BodyParser(&req)
		if req.PerPage == 0 {
			req.PerPage = 1000
		}
		if req.Page == 0 {
			req.Page = 1
		}
		groups, total, err := database.GetGroupsPaginated(req.Page, req.PerPage, req.Search)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "DB error"})
		}
		return c.JSON(fiber.Map{"groups": groups, "total": total, "page": req.Page, "per_page": req.PerPage})
	})
	admin.Post("/groups/create", func(c *fiber.Ctx) error {
		var req struct {
			Name        string  `json:"name"`
			Description string  `json:"description"`
			Color       string  `json:"color"`
			MaxSessions int     `json:"max_sessions"`
			MaxCPU      float64 `json:"max_cpu"`
			MaxMemory   int64   `json:"max_memory"`
		}
		c.BodyParser(&req)
		database.CreateGroup(req.Name, req.Description, req.Color, req.MaxSessions, req.MaxCPU, req.MaxMemory)
		return c.JSON(fiber.Map{"ok": true})
	})
	admin.Post("/groups/update", func(c *fiber.Ctx) error {
		var req struct {
			ID          string  `json:"id"`
			Name        string  `json:"name"`
			Description string  `json:"description"`
			Color       string  `json:"color"`
			MaxSessions int     `json:"max_sessions"`
			MaxCPU      float64 `json:"max_cpu"`
			MaxMemory   int64   `json:"max_memory"`
		}
		c.BodyParser(&req)
		database.UpdateGroup(req.ID, req.Name, req.Description, req.Color, req.MaxSessions, req.MaxCPU, req.MaxMemory)
		return c.JSON(fiber.Map{"ok": true})
	})
	admin.Post("/groups/delete", func(c *fiber.Ctx) error {
		var req struct {
			ID string `json:"id"`
		}
		c.BodyParser(&req)
		database.DeleteGroup(req.ID)
		return c.JSON(fiber.Map{"ok": true})
	})
	admin.Post("/groups/set-workspace", func(c *fiber.Ctx) error {
		var req struct {
			WorkspaceID string   `json:"workspace_id"`
			GroupIDs    []string `json:"group_ids"`
		}
		c.BodyParser(&req)
		database.SetWorkspaceGroups(req.WorkspaceID, req.GroupIDs)
		return c.JSON(fiber.Map{"ok": true})
	})
	admin.Post("/groups/set-user", func(c *fiber.Ctx) error {
		var req struct {
			UserID   string   `json:"user_id"`
			GroupIDs []string `json:"group_ids"`
		}
		c.BodyParser(&req)
		database.SetUserGroups(req.UserID, req.GroupIDs)
		return c.JSON(fiber.Map{"ok": true})
	})

	// OIDC mappings
	admin.Post("/oidc-mappings", func(c *fiber.Ctx) error {
		mappings, _ := database.GetOIDCMappings()
		groups, _ := database.GetGroups()
		return c.JSON(fiber.Map{"mappings": mappings, "groups": groups})
	})
	admin.Post("/oidc-mappings/create", func(c *fiber.Ctx) error {
		var req struct {
			OIDCRole string `json:"oidc_role"`
			GroupID  string `json:"group_id"`
		}
		c.BodyParser(&req)
		database.CreateOIDCMapping(req.OIDCRole, req.GroupID)
		return c.JSON(fiber.Map{"ok": true})
	})
	admin.Post("/oidc-mappings/delete", func(c *fiber.Ctx) error {
		var req struct {
			ID string `json:"id"`
		}
		c.BodyParser(&req)
		database.DeleteOIDCMapping(req.ID)
		return c.JSON(fiber.Map{"ok": true})
	})
	// Users managed by BetterAuth in Next.js, not Go

	// Proxy Engine v2 routes (forward to agent)
	proxyAuth := middleware.AuthRequired(database.DB, blacklist)

	// Start proxy for a session (called when user opens the viewer)
	app.Post("/api/proxy/connect/:sessionId", proxyAuth, func(c *fiber.Ctx) error {
		sessionID := c.Params("sessionId")
		userID, _ := c.Locals("user_id").(string)
		userRole, _ := c.Locals("user_role").(string)

		// Get session info
		var agentEndpoint, agentToken, containerIP, vncPassword, sessionUserID string
		err := database.QueryRow(`
			SELECT a.endpoint, a.token, ws.container_ip, ws.vnc_password, ws.user_id
			FROM workspace_session ws
			JOIN agent a ON a.id = ws.agent_id
			WHERE ws.id = $1 AND ws.status = 'running'`, sessionID).Scan(
			&agentEndpoint, &agentToken, &containerIP, &vncPassword, &sessionUserID)
		if err != nil {
			return c.Status(503).JSON(fiber.Map{"error": "session not found or not running"})
		}

		// IDOR check
		if userRole != "admin" && userID != sessionUserID {
			return c.Status(403).JSON(fiber.Map{"error": "access denied"})
		}

		// Tell agent to start proxy session
		connectReq := map[string]interface{}{
			"session_id":   sessionID,
			"protocol":     "vnc",
			"target_host":  containerIP,
			"target_port":  5900,
			"vnc_password": vncPassword,
			"width":        1920,
			"height":       1080,
		}
		body, _ := json.Marshal(connectReq)

		agentURL := fmt.Sprintf("%s/api/proxy/connect", strings.TrimRight(agentEndpoint, "/"))
		req, _ := http.NewRequest("POST", agentURL, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Agent-Token", agentToken)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return c.Status(502).JSON(fiber.Map{"error": "agent unreachable"})
		}
		defer resp.Body.Close()

		respBody, _ := io.ReadAll(resp.Body)
		c.Set("Content-Type", "application/json")
		return c.Status(resp.StatusCode).Send(respBody)
	})

	// WebRTC signaling
	app.Post("/api/proxy/webrtc/offer/:sessionId", proxyAuth, func(c *fiber.Ctx) error {
		sessionID := c.Params("sessionId")
		userID, _ := c.Locals("user_id").(string)
		userRole, _ := c.Locals("user_role").(string)

		// Find agent for this session
		var agentEndpoint, agentToken, sessionUserID string
		err := database.QueryRow(`
			SELECT a.endpoint, a.token, ws.user_id
			FROM workspace_session ws
			JOIN agent a ON a.id = ws.agent_id
			WHERE ws.id = $1`, sessionID).Scan(&agentEndpoint, &agentToken, &sessionUserID)
		if err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "session not found"})
		}

		// IDOR check
		if userRole != "admin" && userID != sessionUserID {
			return c.Status(403).JSON(fiber.Map{"error": "access denied"})
		}

		// Forward to agent
		body := c.Body()
		agentURL := fmt.Sprintf("%s/api/proxy/webrtc/offer/%s", strings.TrimRight(agentEndpoint, "/"), sessionID)
		req, _ := http.NewRequest("POST", agentURL, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Agent-Token", agentToken)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return c.Status(502).JSON(fiber.Map{"error": "agent unreachable"})
		}
		defer resp.Body.Close()

		respBody, _ := io.ReadAll(resp.Body)
		c.Set("Content-Type", "application/json")
		return c.Status(resp.StatusCode).Send(respBody)
	})

	app.Post("/api/proxy/webrtc/ice/:sessionId", proxyAuth, func(c *fiber.Ctx) error {
		sessionID := c.Params("sessionId")
		userID, _ := c.Locals("user_id").(string)
		userRole, _ := c.Locals("user_role").(string)

		var agentEndpoint, agentToken, sessionUserID string
		err := database.QueryRow(`
			SELECT a.endpoint, a.token, ws.user_id
			FROM workspace_session ws
			JOIN agent a ON a.id = ws.agent_id
			WHERE ws.id = $1`, sessionID).Scan(&agentEndpoint, &agentToken, &sessionUserID)
		if err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "session not found"})
		}

		if userRole != "admin" && userID != sessionUserID {
			return c.Status(403).JSON(fiber.Map{"error": "access denied"})
		}

		body := c.Body()
		agentURL := fmt.Sprintf("%s/api/proxy/webrtc/ice/%s", strings.TrimRight(agentEndpoint, "/"), sessionID)
		req, _ := http.NewRequest("POST", agentURL, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Agent-Token", agentToken)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return c.Status(502).JSON(fiber.Map{"error": "agent unreachable"})
		}
		defer resp.Body.Close()

		respBody, _ := io.ReadAll(resp.Body)
		c.Set("Content-Type", "application/json")
		return c.Status(resp.StatusCode).Send(respBody)
	})

	// Health
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "service": "oklavier-api"})
	})

	// Start session cleanup goroutine
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			cleanupExpiredSessions(database)
		}
	}()

	// Cleanup expired refresh tokens every hour
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			result, err := database.Exec(`DELETE FROM refresh_token WHERE expires_at < NOW()`)
			if err != nil {
				log.Printf("Refresh token cleanup error: %v", err)
				continue
			}
			if rows, _ := result.RowsAffected(); rows > 0 {
				log.Printf("Cleaned up %d expired refresh tokens", rows)
			}
		}
	}()

	// Start agent health check goroutine (circuit breaker)
	// Marks agents as disconnected if they miss 3 consecutive heartbeats (90s)
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			result, err := database.Exec(`UPDATE agent SET status = 'disconnected' WHERE status = 'connected' AND last_heartbeat < NOW() - INTERVAL '90 seconds'`)
			if err != nil {
				log.Printf("Agent health check error: %v", err)
				continue
			}
			if rows, _ := result.RowsAffected(); rows > 0 {
				log.Printf("Marked %d agent(s) as disconnected (missed heartbeats)", rows)
			}
		}
	}()

	log.Printf("Oklavier API starting on %s", listenAddr)
	if tlsCfg, err := mtlsLib.ServerConfig(); err != nil {
		log.Fatalf("mtls config: %v", err)
	} else if tlsCfg != nil {
		log.Printf("mTLS enabled (peer must present a client certificate signed by %s)", os.Getenv("MTLS_CA_FILE"))
		log.Fatal(app.ListenMutualTLSWithCertificate(listenAddr, tlsCfg.Certificates[0], tlsCfg.ClientCAs))
	} else {
		log.Fatal(app.Listen(listenAddr))
	}
}

func cleanupExpiredSessions(database *db.DB) {
	sessions, err := database.GetExpiredSessions()
	if err != nil {
		log.Printf("cleanupExpiredSessions: query error: %v", err)
		return
	}
	for _, s := range sessions {
		// Get agent endpoint and token
		var endpoint, token string
		if s.AgentID != "" {
			database.QueryRow("SELECT endpoint, token FROM agent WHERE id = $1", s.AgentID).Scan(&endpoint, &token)
		}

		// Call agent to destroy the pod
		if endpoint != "" {
			body, _ := json.Marshal(map[string]string{"session_id": s.ID})
			url := fmt.Sprintf("%s/api/destroy-session", strings.TrimRight(endpoint, "/"))
			req, err := http.NewRequest("POST", url, bytes.NewReader(body))
			if err == nil {
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("X-Agent-Token", token)
				client := &http.Client{Timeout: 30 * time.Second}
				if resp, err := client.Do(req); err != nil {
					log.Printf("cleanupExpiredSessions: agent call failed for session %s: %v", s.ID, err)
				} else {
					resp.Body.Close()
				}
			}
		}

		// Delete from DB
		database.DeleteExpiredSession(s.ID)
		log.Printf("Cleaned up expired session %s", s.ID)
	}
}

func boolToStatus(ok bool) string {
	if ok {
		return "healthy"
	}
	return "unhealthy"
}

func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
