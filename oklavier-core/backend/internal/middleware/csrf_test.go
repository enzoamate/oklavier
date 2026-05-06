package middleware

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

// runCSRF builds a tiny app with CSRFGuard guarding POST /m, returns the response status.
func runCSRF(t *testing.T, allowed []string, method, origin, fetchSite, authHeader, cookie string) int {
	t.Helper()
	app := fiber.New()
	app.Use(CSRFGuard(allowed))
	app.Add(method, "/m", func(c *fiber.Ctx) error { return c.SendString("ok") })

	req := httptest.NewRequest(method, "/m", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	if fetchSite != "" {
		req.Header.Set("Sec-Fetch-Site", fetchSite)
	}
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	return resp.StatusCode
}

func TestCSRFGuard(t *testing.T) {
	allowed := []string{"https://oklavier.k0be.io"}

	cases := []struct {
		name   string
		method string
		origin string
		site   string
		auth   string
		cookie string
		want   int
	}{
		// Safe methods are never blocked.
		{"GET no signals", "GET", "", "", "", "", 200},
		{"HEAD no signals", "HEAD", "", "", "", "", 200},

		// Same-origin → allow.
		{"POST same-origin (Sec-Fetch-Site)", "POST", "", "same-origin", "", "", 200},
		{"POST allowed Origin", "POST", "https://oklavier.k0be.io", "", "", "", 200},
		{"POST same-site Sec-Fetch", "POST", "", "same-site", "", "", 200},
		{"POST none Sec-Fetch (typed URL)", "POST", "", "none", "", "", 200},

		// Cross-site → reject.
		{"POST attacker Origin", "POST", "https://evil.com", "", "", "", 403},
		{"POST cross-site Sec-Fetch", "POST", "https://evil.com", "cross-site", "", "", 403},

		// Bearer-only automation: no cookie + Authorization → allow.
		{"POST automation Bearer no Origin", "POST", "", "", "Bearer xyz", "", 200},
		// Same automation but with a cookie → must not auto-allow.
		{"POST cookie + Bearer + no Origin", "POST", "", "", "Bearer xyz", "oklavier_access=yyy", 403},

		// No signals at all on a mutating method (curl with no Origin, no Sec-Fetch,
		// no automation marker, no cookie) → reject.
		{"POST curl-style empty", "POST", "", "", "", "", 403},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := runCSRF(t, allowed, tc.method, tc.origin, tc.site, tc.auth, tc.cookie)
			if got != tc.want {
				t.Fatalf("got %d, want %d", got, tc.want)
			}
		})
	}
}

func TestCSRFGuard_TrailingSlash(t *testing.T) {
	// Trailing slashes in OKLAVIER_ALLOWED_ORIGINS / Origin must not break the match.
	allowed := []string{"https://oklavier.k0be.io/"}
	if got := runCSRF(t, allowed, "POST", "https://oklavier.k0be.io", "", "", ""); got != 200 {
		t.Fatalf("trailing slash in allowlist: got %d", got)
	}
	if got := runCSRF(t, allowed, "POST", "https://oklavier.k0be.io/", "", "", ""); got != 200 {
		t.Fatalf("trailing slash in Origin: got %d", got)
	}
}
