package middleware

import (
	"net/url"
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// CSRFGuard rejects state-changing browser requests that don't come from
// the configured frontend origin. With auth carried via SameSite=Lax cookies,
// a same-site form-POST attack (or a leaked session in a third-party iframe)
// could otherwise mutate state. The check happens on POST/PUT/PATCH/DELETE
// only — safe methods are not blocked.
//
// Strategy:
//  1. If Sec-Fetch-Site is present and is "same-origin" or "none" — allow.
//     (Modern browsers; "none" means user typed/bookmarked the URL.)
//  2. Otherwise check the Origin header against the allowlist (FRONTEND_URL
//     and any extras configured via OKLAVIER_ALLOWED_ORIGINS=comma,sep).
//  3. Requests with no Origin AND no Sec-Fetch-Site are allowed only for
//     non-browser clients (no Cookie header AND a Bearer Authorization).
//  4. Optional bypass via the internal-secret header (Next.js BFF) — that
//     one already authenticates itself with InternalOnly upstream.
func CSRFGuard(allowedOrigins []string) fiber.Handler {
	internalSecret := os.Getenv("OKLAVIER_INTERNAL_SECRET")
	allow := map[string]struct{}{}
	for _, o := range allowedOrigins {
		o = strings.TrimRight(strings.TrimSpace(o), "/")
		if o != "" {
			allow[o] = struct{}{}
		}
	}
	if extra := os.Getenv("OKLAVIER_ALLOWED_ORIGINS"); extra != "" {
		for _, o := range strings.Split(extra, ",") {
			o = strings.TrimRight(strings.TrimSpace(o), "/")
			if o != "" {
				allow[o] = struct{}{}
			}
		}
	}

	return func(c *fiber.Ctx) error {
		method := c.Method()
		if method == "GET" || method == "HEAD" || method == "OPTIONS" {
			return c.Next()
		}

		// Trust the Next.js BFF when InternalOnly already validated it.
		if internalSecret != "" && c.Get("X-Oklavier-Internal") == internalSecret {
			return c.Next()
		}

		// Sec-Fetch-Site: modern browser hint, can't be set by JS cross-site.
		switch c.Get("Sec-Fetch-Site") {
		case "same-origin", "same-site", "none":
			return c.Next()
		case "cross-site":
			return c.Status(403).JSON(fiber.Map{"error": "Cross-site request denied"})
		}

		origin := strings.TrimRight(c.Get("Origin"), "/")
		if origin == "" {
			origin = strings.TrimRight(refererOrigin(c.Get("Referer")), "/")
		}
		if origin != "" {
			if _, ok := allow[origin]; ok {
				return c.Next()
			}
			return c.Status(403).JSON(fiber.Map{"error": "Origin not allowed"})
		}

		// No browser signals. Allow only if it looks like an automation client:
		// Bearer Authorization AND no Cookie header.
		if strings.HasPrefix(c.Get("Authorization"), "Bearer ") && c.Get("Cookie") == "" {
			return c.Next()
		}
		return c.Status(403).JSON(fiber.Map{"error": "Origin required"})
	}
}

func refererOrigin(referer string) string {
	if referer == "" {
		return ""
	}
	u, err := url.Parse(referer)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host
}
