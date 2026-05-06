import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";

const publicPaths = ["/login", "/auth/callback", "/api/auth/", "/api/lang", "/api/branding"];

const STATIC_EXT_RE = /\.(png|jpg|jpeg|gif|svg|ico|webp|avif|css|js|map|woff|woff2|ttf|otf|txt|xml)$/i;

const ACCESS_COOKIE = "oklavier_access";

export async function middleware(request: NextRequest) {
  const { pathname } = request.nextUrl;

  if (publicPaths.some(p => pathname.startsWith(p))) {
    return NextResponse.next();
  }
  if (pathname.startsWith("/_next") || pathname.startsWith("/img") || pathname.startsWith("/api/proxy-img")) {
    return NextResponse.next();
  }
  if (STATIC_EXT_RE.test(pathname)) {
    return NextResponse.next();
  }

  // API routes: require an auth cookie (or a Bearer header for automation).
  if (pathname.startsWith("/api/")) {
    // CSRF: reject cross-site state-changing requests. The Go API trusts the
    // BFF via X-Oklavier-Internal, so the BFF MUST be the gate that checks
    // Origin / Sec-Fetch-Site for browser-driven mutations.
    const m = request.method;
    if (m === "POST" || m === "PUT" || m === "PATCH" || m === "DELETE") {
      const fetchSite = request.headers.get("sec-fetch-site");
      const origin = request.headers.get("origin");
      // Compute the expected origin from the inbound `Host` header (what the
      // client actually saw) — `request.nextUrl.host` may show the pod's
      // internal port (3000) behind a service / port-forward / ingress.
      const hostHeader = request.headers.get("host");
      const proto = request.headers.get("x-forwarded-proto") || request.nextUrl.protocol.replace(":", "");
      const expectedFromHost = hostHeader ? `${proto}://${hostHeader}` : null;
      // Also accept FRONTEND_URL from env (set in helm values) and any extras.
      const allowed = new Set<string>();
      if (expectedFromHost) allowed.add(expectedFromHost);
      const cfg = process.env.FRONTEND_URL;
      if (cfg) allowed.add(cfg.replace(/\/$/, ""));
      const extra = process.env.OKLAVIER_ALLOWED_ORIGINS;
      if (extra) for (const o of extra.split(",")) allowed.add(o.trim().replace(/\/$/, ""));

      const isAutomation = !request.cookies.get(ACCESS_COOKIE)?.value
        && request.headers.get("authorization")?.startsWith("Bearer ");
      if (!isAutomation) {
        if (fetchSite === "cross-site") {
          return NextResponse.json({ error: "Cross-site request denied" }, { status: 403 });
        }
        if (origin && !allowed.has(origin.replace(/\/$/, ""))) {
          return NextResponse.json({ error: "Origin not allowed" }, { status: 403 });
        }
      }
    }

    const hasCookie = request.cookies.get(ACCESS_COOKIE)?.value;
    const hasBearer = request.headers.get("authorization")?.startsWith("Bearer ");
    if (!hasCookie && !hasBearer) {
      return NextResponse.json({ error: "Not authenticated" }, { status: 401 });
    }
    return NextResponse.next();
  }

  if (pathname === "/") {
    return NextResponse.redirect(new URL("/workspaces", request.url));
  }

  return NextResponse.next();
}

export const config = {
  matcher: ["/((?!_next/static|_next/image|favicon.ico).*)"],
};
