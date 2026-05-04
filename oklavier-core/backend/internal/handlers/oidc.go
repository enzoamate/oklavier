package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gofiber/fiber/v2"
	"golang.org/x/oauth2"
	"oklavier-api/internal/auth"
	"oklavier-api/internal/db"
)

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

	// Generate state parameter (CSRF protection)
	_, stateToken, _ := auth.GenerateSessionToken()

	// Store state in a short-lived cookie
	c.Cookie(&fiber.Cookie{
		Name:     "oklavier_oidc_state",
		Value:    stateToken[:32], // Use first 32 chars
		Path:     "/",
		MaxAge:   300, // 5 minutes
		HTTPOnly: true,
		Secure:   h.SecureCookie,
		SameSite: "Lax",
	})

	authURL := cfg.OAuth2Config.AuthCodeURL(stateToken[:32])
	return c.Redirect(authURL, 302)
}

// GET /api/auth/oidc/:providerId/callback - handle OIDC callback
func (h *OIDCHandler) Callback(c *fiber.Ctx) error {
	providerID := c.Params("providerId")
	code := c.Query("code")
	state := c.Query("state")

	// Verify state (CSRF)
	storedState := c.Cookies("oklavier_oidc_state")
	if storedState == "" || storedState != state {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid state parameter"})
	}

	// Clear state cookie
	c.Cookie(&fiber.Cookie{
		Name:   "oklavier_oidc_state",
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

	// Redirect to frontend callback with tokens in URL fragment
	redirectURL := fmt.Sprintf("%s/auth/callback#access_token=%s&refresh_token=%s",
		h.FrontendURL, accessToken, refreshToken)
	return c.Redirect(redirectURL, 302)
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

	// Initialize OIDC provider
	ctx := context.Background()
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
	rand.Read(b)
	return hex.EncodeToString(b)
}
