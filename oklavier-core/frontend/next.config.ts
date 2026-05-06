import type { NextConfig } from "next";

// SECURITY: ship a strict baseline of security headers.
// - CSP defaults to same-origin and blocks framing entirely (clickjacking).
// - X-Content-Type-Options stops MIME sniffing of uploaded assets.
// - Referrer-Policy avoids leaking full URLs (which used to carry tokens
//   in the OIDC callback fragment).
// - Permissions-Policy disables features the app does not need.
//
// `unsafe-inline` is enabled for `style-src` because Next.js / Tailwind
// emit inline styles. `script-src` allows the Guacamole CDN bundle that
// the recordings player loads — narrow this further with SRI when possible.
const cspDirectives = [
  "default-src 'self'",
  "base-uri 'self'",
  "frame-ancestors 'none'",
  "form-action 'self'",
  "object-src 'none'",
  "img-src 'self' data: blob: https:",
  "media-src 'self' blob:",
  "font-src 'self' data:",
  "style-src 'self' 'unsafe-inline'",
  "script-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net",
  "connect-src 'self' wss: https:",
  "worker-src 'self' blob:",
];

const securityHeaders = [
  { key: "Content-Security-Policy", value: cspDirectives.join("; ") },
  { key: "X-Frame-Options", value: "DENY" },
  { key: "X-Content-Type-Options", value: "nosniff" },
  { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
  { key: "Permissions-Policy", value: "camera=(), microphone=(), geolocation=(), payment=()" },
  { key: "Strict-Transport-Security", value: "max-age=31536000; includeSubDomains" },
  // Cross-Origin-* isolation (browser-side defence-in-depth, ZAP baseline asks for these).
  { key: "Cross-Origin-Opener-Policy", value: "same-origin" },
  { key: "Cross-Origin-Resource-Policy", value: "same-origin" },
  // COEP omitted on purpose: `require-corp` would break the recording player
  // that pulls guacamole-common-js from cdn.jsdelivr.net unless we set CORP
  // headers there too. Re-enable when all third-party assets are
  // self-hosted or carry CORP.
];

const nextConfig: NextConfig = {
  output: "standalone",
  // Don't advertise the runtime — ZAP info finding 10037.
  poweredByHeader: false,
  async headers() {
    return [{ source: "/(.*)", headers: securityHeaders }];
  },
  async rewrites() {
    const apiUrl = process.env.NEXT_PUBLIC_OKLAVIER_API || "http://oklavier-api:8080";
    const oklavierApi = process.env.OKLAVIER_API || "http://oklavier-api:8080";
    return [
      {
        source: "/api/auth/oidc/:path*",
        destination: `${oklavierApi}/api/auth/oidc/:path*`,
      },
      {
        source: "/api/vnc-proxy/:path*",
        destination: `${apiUrl}/vnc/:path*`,
      },
      {
        source: "/api/vnc-ws/:path*",
        destination: `${apiUrl}/vnc-ws/:path*`,
      },
      {
        source: "/api/agent/:path*",
        destination: `${apiUrl}/api/agent/:path*`,
      },
      {
        source: "/proxy/ws/:path*",
        destination: `${oklavierApi}/proxy/ws/:path*`,
      },
    ];
  },
};

export default nextConfig;
