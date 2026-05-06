package handlers

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gofiber/fiber/v2"
	"golang.org/x/oauth2"
	"oklavier-api/internal/auth"
	"oklavier-api/internal/db"
)

// safeOutboundClient returns an *http.Client whose dialer rejects connections
// to loopback / link-local / private / multicast / unspecified IPs. It mitigates
// SSRF (e.g. attacker-controlled OIDC issuer URL pointing at 169.254.169.254
// for cloud metadata) and DNS rebinding (re-resolution per redirect cannot
// land on a private IP). Hostnames are resolved at dial time and every
// returned address is checked.
func safeOutboundClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{Timeout: timeout, KeepAlive: 30 * time.Second}
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				host, port, err := net.SplitHostPort(addr)
				if err != nil {
					return nil, err
				}
				ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
				if err != nil {
					return nil, err
				}
				for _, ip := range ips {
					if isPrivateOrSpecialIP(ip.IP) {
						return nil, fmt.Errorf("blocked request to private/special IP %s", ip.IP)
					}
				}
				// Use the first acceptable IP explicitly so the dial cannot
				// re-resolve to a different address.
				return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
			},
		},
	}
}

func isPrivateOrSpecialIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() || ip.IsUnspecified() || ip.IsPrivate() {
		return true
	}
	// Cloud metadata, CGNAT, broadcast.
	for _, cidr := range []string{
		"169.254.0.0/16", // link-local incl. AWS/GCP/Azure metadata
		"100.64.0.0/10",  // CGNAT
		"::ffff:169.254.169.254/128",
	} {
		_, n, _ := net.ParseCIDR(cidr)
		if n != nil && n.Contains(ip) {
			return true
		}
	}
	return false
}

// isSafeIssuerURL validates an OIDC issuer URL: must be https, must have a
// host, must not point at a private/special IP literal. (Hostnames are
// re-checked at dial time by safeOutboundClient.)
func isSafeIssuerURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return fmt.Errorf("issuer must be http(s)")
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("issuer must have a host")
	}
	if ip := net.ParseIP(host); ip != nil && isPrivateOrSpecialIP(ip) {
		return fmt.Errorf("issuer host is a private/special IP")
	}
	return nil
}

type OIDCHandler struct {
	DB           *db.DB
	SecureCookie bool
	FrontendURL  string
}

type oidcProviderConfig struct {
	ID           string
	Name         string
	ClientID     string
	ClientSecret string
	IssuerURL    string
	Provider     *oidc.Provider
	OAuth2Config *oauth2.Config
}

// GET /api/auth/providers - list enabled OIDC providers
func (h *OIDCHandler) ListProviders(c *fiber.Ctx) error {
	type authMethod struct {
		ID      string          `db:"id"`
		Name    string          `db:"name"`
		Type    string          `db:"type"`
		Enabled bool            `db:"enabled"`
		Config  json.RawMessage `db:"config"`
	}

	var methods []authMethod
	err := h.DB.Select(&methods, `SELECT id, name, type, enabled, config FROM auth_method WHERE type = 'oidc' AND enabled = true`)
	if err != nil {
		return c.JSON([]interface{}{})
	}

	providers := make([]fiber.Map, 0)
	for _, m := range methods {
		var cfg map[string]string
		json.Unmarshal(m.Config, &cfg)
		providers = append(providers, fiber.Map{
			"id":   m.ID,
			"name": cfg["display_name"],
			"logo": cfg["logo_url"],
		})
	}
	return c.JSON(providers)
}

// GET /api/auth/oidc/:providerId - initiate OIDC login (redirect to IdP)
func (h *OIDCHandler) Authorize(c *fiber.Ctx) error {
	providerID := c.Params("providerId")

	cfg, err := h.getProviderConfig(providerID, c)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Provider not found"})
	}

	// Generate state + nonce parameters (CSRF + ID-token replay protection).
	// SECURITY: previously no nonce was passed at all; without it, ID tokens
	// can be replayed across login attempts on IdPs that don't otherwise bind them.
	_, stateToken, _ := auth.GenerateSessionToken()
	if len(stateToken) < 32 {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to generate state"})
	}
	state := stateToken[:32]
	nonce := oidcGenerateID()

	c.Cookie(&fiber.Cookie{
		Name:     "oklavier_oidc_state",
		Value:    state,
		Path:     "/",
		MaxAge:   300, // 5 minutes
		HTTPOnly: true,
		Secure:   h.SecureCookie,
		SameSite: "Lax",
	})
	c.Cookie(&fiber.Cookie{
		Name:     "oklavier_oidc_nonce",
		Value:    nonce,
		Path:     "/",
		MaxAge:   300,
		HTTPOnly: true,
		Secure:   h.SecureCookie,
		SameSite: "Lax",
	})

	authURL := cfg.OAuth2Config.AuthCodeURL(state, oidc.Nonce(nonce))
	return c.Redirect(authURL, 302)
}

// GET /api/auth/oidc/:providerId/callback - handle OIDC callback
func (h *OIDCHandler) Callback(c *fiber.Ctx) error {
	providerID := c.Params("providerId")
	code := c.Query("code")
	state := c.Query("state")

	// Verify state (CSRF) — constant-time compare so a leaked timing channel
	// cannot help an attacker craft a matching state.
	storedState := c.Cookies("oklavier_oidc_state")
	if storedState == "" || subtle.ConstantTimeCompare([]byte(storedState), []byte(state)) != 1 {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid state parameter"})
	}

	// Clear state cookie
	c.Cookie(&fiber.Cookie{
		Name:   "oklavier_oidc_state",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})

	// Read & clear the nonce cookie up-front so we can verify it after token verify.
	storedNonce := c.Cookies("oklavier_oidc_nonce")
	c.Cookie(&fiber.Cookie{
		Name:   "oklavier_oidc_nonce",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})

	cfg, err := h.getProviderConfig(providerID, c)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Provider not found"})
	}

	// Exchange code for token
	ctx := context.Background()
	token, err := cfg.OAuth2Config.Exchange(ctx, code)
	if err != nil {
		log.Printf("OIDC token exchange error: %v", err)
		return c.Status(400).JSON(fiber.Map{"error": "Failed to exchange code"})
	}

	// Verify ID token
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return c.Status(400).JSON(fiber.Map{"error": "No id_token in response"})
	}

	verifier := cfg.Provider.Verifier(&oidc.Config{ClientID: cfg.ClientID})
	idToken, err := verifier.Verify(ctx, rawIDToken)
	if err != nil {
		log.Printf("OIDC token verification error: %v", err)
		return c.Status(400).JSON(fiber.Map{"error": "Invalid ID token"})
	}

	// Verify nonce binding (CSRF + replay protection).
	if storedNonce == "" || idToken.Nonce == "" ||
		subtle.ConstantTimeCompare([]byte(storedNonce), []byte(idToken.Nonce)) != 1 {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid nonce"})
	}

	// Extract claims
	var claims struct {
		Email             string   `json:"email"`
		Name              string   `json:"name"`
		PreferredUsername string   `json:"preferred_username"`
		Picture           string   `json:"picture"`
		Groups            []string `json:"groups"`
		Roles             []string `json:"roles"`
		RealmAccess       struct {
			Roles []string `json:"roles"`
		} `json:"realm_access"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Failed to parse claims"})
	}

	email := claims.Email
	name := claims.Name
	if name == "" {
		name = claims.PreferredUsername
	}
	if name == "" {
		name = email
	}

	// Collect all roles
	allRoles := append(claims.Roles, claims.Groups...)
	allRoles = append(allRoles, claims.RealmAccess.Roles...)

	// Find or create user
	var userID, role string
	err = h.DB.QueryRow(`SELECT id, COALESCE(role, 'user') FROM "user" WHERE email = $1`, email).Scan(&userID, &role)
	if err != nil {
		// Create new user
		userID = oidcGenerateID()
		role = "user"
		now := time.Now()
		_, err = h.DB.Exec(`INSERT INTO "user" (id, name, email, "emailVerified", "createdAt", "updatedAt", role, auth_provider, oidc_provider_name)
			VALUES ($1, $2, $3, true, $4, $4, 'user', 'oidc', $5)`,
			userID, name, email, now, cfg.Name)
		if err != nil {
			log.Printf("OIDC user create error: %v", err)
			return c.Status(500).JSON(fiber.Map{"error": "Failed to create user"})
		}
		// Add to default group
		h.DB.Exec(`INSERT INTO user_group (user_id, group_id) SELECT $1, id FROM oklavier_group WHERE is_default = true`, userID)
	} else {
		// Update existing user
		h.DB.Exec(`UPDATE "user" SET name = $1, auth_provider = 'oidc', oidc_provider_name = $2, last_login = NOW() WHERE id = $3`,
			name, cfg.Name, userID)
	}

	// Sync OIDC roles
	if len(allRoles) > 0 {
		// Store discovered roles
		h.DB.Exec(`DELETE FROM user_oidc_role WHERE user_id = $1`, userID)
		for _, r := range allRoles {
			r = strings.TrimSpace(r)
			if r != "" {
				h.DB.Exec(`INSERT INTO user_oidc_role (user_id, role) VALUES ($1, $2) ON CONFLICT DO NOTHING`, userID, r)
			}
		}
		// Apply role mappings to groups
		h.DB.Exec(`DELETE FROM user_group WHERE user_id = $1 AND group_id IN (SELECT group_id FROM oidc_role_mapping)`, userID)
		h.DB.Exec(`INSERT INTO user_group (user_id, group_id) SELECT DISTINCT $1, m.group_id FROM oidc_role_mapping m JOIN user_oidc_role r ON r.role = m.oidc_role WHERE r.user_id = $1 ON CONFLICT DO NOTHING`, userID)
	}

	// Generate JWT tokens
	accessToken, _, err := auth.GenerateAccessToken(userID, email, role)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Internal error"})
	}

	refreshToken, refreshJTI, err := auth.GenerateRefreshToken(userID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Internal error"})
	}

	// Store refresh token in DB
	h.DB.Exec(`INSERT INTO refresh_token (jti, user_id, expires_at, ip_address, user_agent) VALUES ($1, $2, $3, $4, $5)`,
		refreshJTI, userID, time.Now().Add(auth.RefreshTokenTTL), c.IP(), c.Get("User-Agent"))

	// Audit
	h.DB.LogAudit(userID, email, "oidc_login", "auth", providerID, cfg.Name, c.IP())

	// SECURITY: deliver tokens via short-lived httpOnly cookies + a one-time
	// pickup code, NOT in the URL fragment. Fragments don't go into HTTP logs
	// but they DO go into browser history, password managers, third-party JS
	// on the callback page, and `window.opener` if a popup was used. The
	// long-lived (7-day) refresh token in particular must never sit in URL state.
	c.Cookie(&fiber.Cookie{
		Name:     "oklavier_access",
		Value:    accessToken,
		Path:     "/",
		MaxAge:   int(auth.AccessTokenTTL.Seconds()),
		HTTPOnly: true,
		Secure:   h.SecureCookie,
		SameSite: "Lax",
	})
	c.Cookie(&fiber.Cookie{
		Name:     "oklavier_refresh",
		Value:    refreshToken,
		Path:     "/",
		MaxAge:   int(auth.RefreshTokenTTL.Seconds()),
		HTTPOnly: true,
		Secure:   h.SecureCookie,
		SameSite: "Lax",
	})
	// The frontend callback reads the cookies via /api/auth/me and then clears them.
	return c.Redirect(h.FrontendURL+"/auth/callback?oidc=ok", 302)
}

func (h *OIDCHandler) getProviderConfig(providerID string, c *fiber.Ctx) (*oidcProviderConfig, error) {
	var method struct {
		ID     string          `db:"id"`
		Name   string          `db:"name"`
		Config json.RawMessage `db:"config"`
	}
	err := h.DB.Get(&method, `SELECT id, name, config FROM auth_method WHERE id = $1 AND type = 'oidc' AND enabled = true`, providerID)
	if err != nil {
		return nil, err
	}

	var cfg map[string]string
	json.Unmarshal(method.Config, &cfg)

	clientID := cfg["client_id"]
	clientSecret := cfg["client_secret"]
	issuerURL := cfg["issuer"]

	// SECURITY: validate the issuer URL (admin-controlled but should still be
	// constrained) and use an HTTP client that refuses connections to private
	// / link-local / cloud-metadata IPs to prevent SSRF + DNS rebinding.
	if err := isSafeIssuerURL(issuerURL); err != nil {
		return nil, fmt.Errorf("unsafe issuer URL: %w", err)
	}
	ctx := oidc.ClientContext(context.Background(), safeOutboundClient(10*time.Second))
	provider, err := oidc.NewProvider(ctx, issuerURL)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize OIDC provider: %w", err)
	}

	// Build callback URL from configured frontend URL (never trust X-Forwarded-Host)
	callbackURL := fmt.Sprintf("%s/api/auth/oidc/%s/callback", h.FrontendURL, providerID)

	oauth2Config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     provider.Endpoint(),
		RedirectURL:  callbackURL,
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
	}

	return &oidcProviderConfig{
		ID:           method.ID,
		Name:         cfg["display_name"],
		ClientID:     clientID,
		ClientSecret: clientSecret,
		IssuerURL:    issuerURL,
		Provider:     provider,
		OAuth2Config: oauth2Config,
	}, nil
}

func oidcGenerateID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand failure: %v", err))
	}
	return hex.EncodeToString(b)
}
