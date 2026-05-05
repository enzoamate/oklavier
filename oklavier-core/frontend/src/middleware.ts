import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";

const publicPaths = ["/login", "/auth/callback", "/api/auth/", "/api/lang", "/api/branding"];

// SECURITY: tightly-scoped static-file extensions. Previously, the gate was
// `pathname.includes(".")` which let ANY URL containing a dot bypass the
// /api/ Authorization check (e.g. /api/admin/users.json reached the route
// handler with no edge auth).
const STATIC_EXT_RE = /\.(png|jpg|jpeg|gif|svg|ico|webp|avif|css|js|map|woff|woff2|ttf|otf|txt|xml)$/i;

export async function middleware(request: NextRequest) {
  const { pathname } = request.nextUrl;

  // Allow public paths
  if (publicPaths.some(p => pathname.startsWith(p))) {
    return NextResponse.next();
  }

  // Allow Next.js build assets and the controlled image proxy.
  if (pathname.startsWith("/_next") || pathname.startsWith("/img") || pathname.startsWith("/api/proxy-img")) {
    return NextResponse.next();
  }

  // Allow real static files only (extension allowlist, not "any dot").
  if (STATIC_EXT_RE.test(pathname)) {
    return NextResponse.next();
  }

  // API routes: require Authorization header at the edge.
  if (pathname.startsWith("/api/")) {
    const auth = request.headers.get("authorization");
    if (!auth) {
      return NextResponse.json({ error: "Not authenticated" }, { status: 401 });
    }
    return NextResponse.next();
  }

  // Page routes: AuthGuard client component handles redirect to /login.
  if (pathname === "/") {
    return NextResponse.redirect(new URL("/workspaces", request.url));
  }

  return NextResponse.next();
}

export const config = {
  matcher: ["/((?!_next/static|_next/image|favicon.ico).*)"],
};
