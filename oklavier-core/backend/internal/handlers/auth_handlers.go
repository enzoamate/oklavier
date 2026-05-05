package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/smtp"
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"oklavier-api/internal/auth"
	"oklavier-api/internal/db"
	"oklavier-api/internal/middleware"
)

type AuthHandlers struct {
	DB          *db.DB
	RateLimiter *auth.RateLimiter
	Blacklist   *auth.TokenBlacklist
	// FrontendURL is the canonical base URL of the frontend (e.g.
	// "https://oklavier.example.com"). Used for outbound links such as
	// password-reset emails. MUST come from server-side configuration —
	// previously, password-reset URLs were built from the request Origin /
	// Referer header, allowing an attacker to phish reset tokens.
	FrontendURL string
}

// POST /api/auth/signup
func (h *AuthHandlers) Signup(c *fiber.Ctx) error {
	var req struct {
		Name     string `json:"name" validate:"required,min=1,max=100"`
		Email    string `json:"email" validate:"required,email"`
		Password string `json:"password" validate:"required,min=8"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}
	if err := middleware.Validate.Struct(req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.Name = strings.TrimSpace(req.Name)

	if err := auth.ValidateEmail(req.Email); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	if err := auth.ValidatePassword(req.Password); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	if len(req.Name) < 1 || len(req.Name) > 100 {
		return c.Status(400).JSON(fiber.Map{"error": "Name is required (max 100 chars)"})
	}

	// Check if user already exists
	var exists int
	h.DB.Get(&exists, `SELECT COUNT(*) FROM "user" WHERE email = $1`, req.Email)
	if exists > 0 {
		return c.Status(409).JSON(fiber.Map{"error": "An account with this email already exists"})
	}

	// Hash password
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		log.Printf("Signup hash error: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Internal error"})
	}

	// Create user
	userID := generateAuthID()
	accountID := generateAuthID()
	now := time.Now()

	tx, err := h.DB.Beginx()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Internal error"})
	}
	defer tx.Rollback()

	_, err = tx.Exec(`INSERT INTO "user" (id, name, email, "emailVerified", "createdAt", "updatedAt", role) VALUES ($1, $2, $3, false, $4, $4, 'user')`,
		userID, req.Name, req.Email, now)
	if err != nil {
		log.Printf("Signup user insert error: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Internal error"})
	}

	_, err = tx.Exec(`INSERT INTO "account" (id, "accountId", "providerId", "userId", password, "createdAt", "updatedAt") VALUES ($1, $2, 'credential', $3, $4, $5, $5)`,
		accountID, userID, userID, hash, now)
	if err != nil {
		log.Printf("Signup account insert error: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Internal error"})
	}

	// Add user to default group
	tx.Exec(`INSERT INTO user_group (user_id, group_id) SELECT $1, id FROM oklavier_group WHERE is_default = true`, userID)

	if err := tx.Commit(); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Internal error"})
	}

	// Audit
	h.DB.LogAudit(userID, req.Email, "signup", "user", userID, "", c.IP())

	// Auto-login: create tokens
	return h.createTokens(c, userID, req.Email, "user", req.Name)
}

// POST /api/auth/login
func (h *AuthHandlers) Login(c *fiber.Ctx) error {
	// Rate limiting
	if !h.RateLimiter.Allow(c.IP()) {
		remaining := h.RateLimiter.Remaining(c.IP())
		c.Set("Retry-After", "60")
		c.Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
		h.DB.LogAudit("", "", "login_rate_limited", "auth", "", c.IP(), c.IP())
		return c.Status(429).JSON(fiber.Map{"error": "Too many login attempts. Please try again later."})
	}

	var req struct {
		Email    string `json:"email" validate:"required,email"`
		Password string `json:"password" validate:"required"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}
	if err := middleware.Validate.Struct(req); err != nil {
		time.Sleep(200 * time.Millisecond)
		return c.Status(401).JSON(fiber.Map{"error": "Invalid credentials"})
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	// Validate input format (prevent injection, but generic error)
	if auth.ValidateEmail(req.Email) != nil || len(req.Password) == 0 {
		time.Sleep(200 * time.Millisecond)
		return c.Status(401).JSON(fiber.Map{"error": "Invalid credentials"})
	}

	// Get user + password hash
	var userID, name, email, role, passwordHash string
	var locked bool
	var failedAttempts int
	var lockedUntil *time.Time

	err := h.DB.QueryRow(`
		SELECT u.id, u.name, u.email, COALESCE(u.role, 'user'),
			a.password,
			COALESCE(u.banned, false),
			COALESCE(u.failed_login_attempts, 0),
			u.locked_until
		FROM "user" u
		JOIN "account" a ON a."userId" = u.id AND a."providerId" = 'credential'
		WHERE u.email = $1
	`, req.Email).Scan(&userID, &name, &email, &role, &passwordHash, &locked, &failedAttempts, &lockedUntil)

	if err != nil {
		time.Sleep(200 * time.Millisecond)
		h.DB.LogAudit("", req.Email, "login_failed", "auth", "", "user not found", c.IP())
		return c.Status(401).JSON(fiber.Map{"error": "Invalid credentials"})
	}

	if locked {
		h.DB.LogAudit(userID, email, "login_banned", "auth", userID, "", c.IP())
		return c.Status(403).JSON(fiber.Map{"error": "Account is suspended"})
	}

	if lockedUntil != nil && time.Now().Before(*lockedUntil) {
		h.DB.LogAudit(userID, email, "login_locked", "auth", userID, "", c.IP())
		return c.Status(423).JSON(fiber.Map{"error": "Account temporarily locked. Try again later."})
	}

	if !auth.VerifyPassword(passwordHash, req.Password) {
		newAttempts := failedAttempts + 1
		if newAttempts >= auth.MaxLoginAttempts {
			lockUntil := time.Now().Add(auth.LockoutDuration)
			h.DB.Exec(`UPDATE "user" SET failed_login_attempts = $1, locked_until = $2 WHERE id = $3`, newAttempts, lockUntil, userID)
			h.DB.LogAudit(userID, email, "account_locked", "auth", userID, fmt.Sprintf("attempts=%d", newAttempts), c.IP())
		} else {
			h.DB.Exec(`UPDATE "user" SET failed_login_attempts = $1 WHERE id = $2`, newAttempts, userID)
		}
		h.DB.LogAudit(userID, email, "login_failed", "auth", userID, fmt.Sprintf("attempt=%d/%d", newAttempts, auth.MaxLoginAttempts), c.IP())
		return c.Status(401).JSON(fiber.Map{"error": "Invalid credentials"})
	}

	// Success: reset failed attempts + update last login
	h.DB.Exec(`UPDATE "user" SET failed_login_attempts = 0, locked_until = NULL, last_login = NOW() WHERE id = $1`, userID)

	h.DB.LogAudit(userID, email, "login_success", "auth", userID, "", c.IP())

	return h.createTokens(c, userID, email, role, name)
}

// POST /api/auth/logout
func (h *AuthHandlers) Logout(c *fiber.Ctx) error {
	// Extract access token from Authorization header
	authHeader := c.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		claims, err := auth.ValidateAccessToken(tokenStr)
		if err == nil && claims.ID != "" {
			// Blacklist the access token for its remaining lifetime
			ttl := time.Until(claims.ExpiresAt.Time)
			h.Blacklist.Blacklist(claims.ID, ttl)
		}
	}

	// Delete refresh token if provided
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	c.BodyParser(&req)
	if req.RefreshToken != "" {
		refreshClaims, err := auth.ValidateRefreshToken(req.RefreshToken)
		if err == nil {
			h.DB.Exec(`DELETE FROM refresh_token WHERE jti = $1`, refreshClaims.ID)
		}
	}

	return c.JSON(fiber.Map{"ok": true})
}

// GET /api/auth/me
func (h *AuthHandlers) Me(c *fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(string)
	if userID == "" {
		return c.Status(401).JSON(fiber.Map{"error": "Not authenticated"})
	}

	var user struct {
		ID        string    `db:"id" json:"id"`
		Name      string    `db:"name" json:"name"`
		Email     string    `db:"email" json:"email"`
		Role      string    `db:"role" json:"role"`
		Image     *string   `db:"image" json:"image"`
		CreatedAt time.Time `db:"createdAt" json:"createdAt"`
	}
	err := h.DB.Get(&user, `SELECT id, name, email, COALESCE(role,'user') as role, image, "createdAt" FROM "user" WHERE id = $1`, userID)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "User not found"})
	}

	return c.JSON(fiber.Map{
		"user": user,
		"session": fiber.Map{
			"userId": userID,
		},
	})
}

// POST /api/auth/refresh
func (h *AuthHandlers) Refresh(c *fiber.Ctx) error {
	// Rate limit refresh attempts
	if !h.RateLimiter.Allow(c.IP()) {
		return c.Status(429).JSON(fiber.Map{"error": "Too many refresh attempts. Please try again later."})
	}

	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := c.BodyParser(&req); err != nil || req.RefreshToken == "" {
		return c.Status(400).JSON(fiber.Map{"error": "refresh_token is required"})
	}

	// Validate the refresh token
	claims, err := auth.ValidateRefreshToken(req.RefreshToken)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{"error": "Invalid or expired refresh token"})
	}

	// Atomic rotation: DELETE + check in one query (prevents race condition)
	var userID string
	err = h.DB.QueryRow(`DELETE FROM refresh_token WHERE jti = $1 AND expires_at > NOW() RETURNING user_id`, claims.ID).Scan(&userID)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{"error": "Refresh token revoked or expired"})
	}

	// Get current user info (role may have changed)
	var email, role string
	err = h.DB.QueryRow(`SELECT email, COALESCE(role, 'user') FROM "user" WHERE id = $1`, userID).Scan(&email, &role)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{"error": "User not found"})
	}

	// Generate new tokens
	accessToken, _, err := auth.GenerateAccessToken(userID, email, role)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Internal error"})
	}

	newRefreshToken, newRefreshJTI, err := auth.GenerateRefreshToken(userID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Internal error"})
	}

	// Store new refresh token
	h.DB.Exec(`INSERT INTO refresh_token (jti, user_id, expires_at, ip_address, user_agent) VALUES ($1, $2, $3, $4, $5)`,
		newRefreshJTI, userID, time.Now().Add(auth.RefreshTokenTTL), c.IP(), c.Get("User-Agent"))

	return c.JSON(fiber.Map{
		"access_token":  accessToken,
		"refresh_token": newRefreshToken,
		"expires_in":    int(auth.AccessTokenTTL.Seconds()),
	})
}

// Internal: create access + refresh tokens and return them
func (h *AuthHandlers) createTokens(c *fiber.Ctx, userID, email, role, name string) error {
	accessToken, _, err := auth.GenerateAccessToken(userID, email, role)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Internal error"})
	}

	refreshToken, refreshJTI, err := auth.GenerateRefreshToken(userID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Internal error"})
	}

	// Store refresh token in DB
	_, err = h.DB.Exec(`INSERT INTO refresh_token (jti, user_id, expires_at, ip_address, user_agent) VALUES ($1, $2, $3, $4, $5)`,
		refreshJTI, userID, time.Now().Add(auth.RefreshTokenTTL), c.IP(), c.Get("User-Agent"))
	if err != nil {
		log.Printf("Refresh token insert error: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Internal error"})
	}

	return c.JSON(fiber.Map{
		"user": fiber.Map{
			"id":    userID,
			"name":  name,
			"email": email,
			"role":  role,
		},
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"expires_in":    int(auth.AccessTokenTTL.Seconds()),
	})
}

// POST /api/admin/revoke-sessions
func (h *AuthHandlers) RevokeSessions(c *fiber.Ctx) error {
	var req struct {
		UserID string `json:"user_id" validate:"required"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}
	if err := middleware.Validate.Struct(req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "user_id required"})
	}

	result, err := h.DB.Exec(`DELETE FROM refresh_token WHERE user_id = $1`, req.UserID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to revoke sessions"})
	}
	rows, _ := result.RowsAffected()

	h.DB.LogAudit(
		c.Locals("user_id").(string),
		c.Locals("user_email").(string),
		"revoke_sessions", "user", req.UserID,
		fmt.Sprintf("revoked %d refresh tokens", rows),
		c.IP(),
	)

	return c.JSON(fiber.Map{"ok": true, "revoked": rows})
}

// POST /api/auth/change-password
func (h *AuthHandlers) ChangePassword(c *fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(string)
	if userID == "" {
		return c.Status(401).JSON(fiber.Map{"error": "Not authenticated"})
	}

	var req struct {
		CurrentPassword string `json:"current_password" validate:"required"`
		NewPassword     string `json:"new_password" validate:"required,min=8"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}
	if err := middleware.Validate.Struct(req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	if err := auth.ValidatePassword(req.NewPassword); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	var currentHash string
	err := h.DB.QueryRow(`SELECT password FROM "account" WHERE "userId" = $1 AND "providerId" = 'credential'`, userID).Scan(&currentHash)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "No password-based account found"})
	}

	if !auth.VerifyPassword(currentHash, req.CurrentPassword) {
		return c.Status(401).JSON(fiber.Map{"error": "Current password is incorrect"})
	}

	newHash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Internal error"})
	}

	_, err = h.DB.Exec(`UPDATE "account" SET password = $1, "updatedAt" = NOW() WHERE "userId" = $2 AND "providerId" = 'credential'`, newHash, userID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to update password"})
	}

	email, _ := c.Locals("user_email").(string)
	h.DB.LogAudit(userID, email, "change_password", "user", userID, "", c.IP())

	return c.JSON(fiber.Map{"ok": true})
}

// POST /api/auth/forgot-password (public)
func (h *AuthHandlers) ForgotPassword(c *fiber.Ctx) error {
	var req struct {
		Email string `json:"email" validate:"required,email"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}
	if err := middleware.Validate.Struct(req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	defer func() {
		time.Sleep(200 * time.Millisecond)
	}()

	var userID string
	err := h.DB.QueryRow(`SELECT id FROM "user" WHERE email = $1`, req.Email).Scan(&userID)
	if err != nil {
		return c.JSON(fiber.Map{"ok": true, "message": "If this email exists, a reset link has been sent"})
	}

	raw, hashed, err := auth.GenerateSessionToken()
	if err != nil {
		return c.JSON(fiber.Map{"ok": true, "message": "If this email exists, a reset link has been sent"})
	}

	h.DB.Exec(`INSERT INTO "verification" (id, identifier, value, "expiresAt") VALUES ($1, $2, $3, NOW() + INTERVAL '1 hour')`,
		generateAuthID(), req.Email, hashed)

	smtpHost := h.DB.GetSetting("smtp.host")
	if smtpHost == "" {
		smtpHost = os.Getenv("SMTP_HOST")
	}
	if smtpHost != "" {
		smtpConfig := map[string]string{
			"host":     smtpHost,
			"port":     h.DB.GetSetting("smtp.port"),
			"user":     h.DB.GetSetting("smtp.user"),
			"password": h.DB.GetSetting("smtp.password"),
			"from":     h.DB.GetSetting("smtp.from"),
		}
		if smtpConfig["port"] == "" {
			smtpConfig["port"] = os.Getenv("SMTP_PORT")
		}
		if smtpConfig["from"] == "" {
			smtpConfig["from"] = os.Getenv("SMTP_FROM")
		}
		if smtpConfig["from"] == "" {
			smtpConfig["from"] = "noreply@oklavier.local"
		}

		// SECURITY: build the reset URL from server-side config, never from
		// request headers. Trusting Origin/Referer let an attacker submit
		// `Origin: https://evil.com` and receive (via the victim's email) a
		// reset link that POSTs the raw reset token to their server.
		base := h.FrontendURL
		if base == "" {
			base = os.Getenv("FRONTEND_URL")
		}
		if base == "" {
			base = "http://localhost:3000"
		}
		resetURL := fmt.Sprintf("%s/login?reset=%s", strings.TrimRight(base, "/"), raw)

		go sendResetEmail(req.Email, resetURL, smtpConfig)
	}

	h.DB.LogAudit(userID, req.Email, "forgot_password", "auth", userID, "", c.IP())

	return c.JSON(fiber.Map{"ok": true, "message": "If this email exists, a reset link has been sent"})
}

// POST /api/auth/reset-password (public)
func (h *AuthHandlers) ResetPassword(c *fiber.Ctx) error {
	var req struct {
		Token       string `json:"token" validate:"required"`
		NewPassword string `json:"new_password" validate:"required,min=8"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}
	if err := middleware.Validate.Struct(req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	if err := auth.ValidatePassword(req.NewPassword); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	hashed := auth.HashToken(req.Token)
	var email string
	err := h.DB.QueryRow(`DELETE FROM "verification" WHERE value = $1 AND "expiresAt" > NOW() RETURNING identifier`, hashed).Scan(&email)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid or expired reset token"})
	}

	var userID string
	err = h.DB.QueryRow(`SELECT id FROM "user" WHERE email = $1`, email).Scan(&userID)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "User not found"})
	}

	newHash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Internal error"})
	}

	h.DB.Exec(`UPDATE "account" SET password = $1, "updatedAt" = NOW() WHERE "userId" = $2 AND "providerId" = 'credential'`, newHash, userID)

	// Invalidate all refresh tokens for this user (force re-login)
	h.DB.Exec(`DELETE FROM refresh_token WHERE user_id = $1`, userID)

	h.DB.LogAudit(userID, email, "reset_password", "auth", userID, "", c.IP())

	return c.JSON(fiber.Map{"ok": true})
}

func sendResetEmail(to, resetURL string, config map[string]string) {
	from := config["from"]
	if from == "" {
		from = "noreply@oklavier.local"
	}

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: Oklavier - Password Reset\r\nContent-Type: text/html\r\n\r\n"+
		"<h2>Password Reset</h2><p>Click the link below to reset your password:</p>"+
		"<p><a href=\"%s\">Reset Password</a></p>"+
		"<p>This link expires in 1 hour.</p>"+
		"<p>If you did not request this, ignore this email.</p>",
		from, to, resetURL)

	addr := fmt.Sprintf("%s:%s", config["host"], config["port"])
	var authSMTP smtp.Auth
	if config["user"] != "" {
		authSMTP = smtp.PlainAuth("", config["user"], config["password"], config["host"])
	}

	if err := smtp.SendMail(addr, authSMTP, from, []string{to}, []byte(msg)); err != nil {
		log.Printf("Failed to send reset email to %s: %v", to, err)
	} else {
		log.Printf("Reset email sent to %s", to)
	}
}

// POST /api/auth/update-profile
func (h *AuthHandlers) UpdateProfile(c *fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(string)
	if userID == "" {
		return c.Status(401).JSON(fiber.Map{"error": "Not authenticated"})
	}

	var req struct {
		Name string `json:"name" validate:"required,min=1,max=100"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}
	if err := middleware.Validate.Struct(req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Name must be 1-100 characters"})
	}

	req.Name = strings.TrimSpace(req.Name)
	if len(req.Name) < 1 || len(req.Name) > 100 {
		return c.Status(400).JSON(fiber.Map{"error": "Name must be 1-100 characters"})
	}

	_, err := h.DB.Exec(`UPDATE "user" SET name = $1, "updatedAt" = NOW() WHERE id = $2`, req.Name, userID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to update profile"})
	}

	email, _ := c.Locals("user_email").(string)
	h.DB.LogAudit(userID, email, "update_profile", "user", userID, req.Name, c.IP())

	return c.JSON(fiber.Map{"ok": true, "name": req.Name})
}

func generateAuthID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
