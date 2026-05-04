import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";

const publicPaths = ["/login", "/auth/callback", "/api/auth/", "/api/lang", "/api/branding"];

export async function middleware(request: NextRequest) {
  const { pathname } = request.nextUrl;

  // Allow public paths
  if (publicPaths.some(p => pathname.startsWith(p))) {
    return NextResponse.next();
  }

  // Allow static files
  if (pathname.startsWith("/_next") || pathname.startsWith("/img") || pathname.startsWith("/api/proxy-img") || pathname.includes(".")) {
    return NextResponse.next();
  }

  // API routes: check for Authorization header presence
  if (pathname.startsWith("/api/")) {
    const auth = request.headers.get("authorization");
    if (!auth) {
      return NextResponse.json({ error: "Not authenticated" }, { status: 401 });
    }
    return NextResponse.next();
  }

  // Page routes: can't check localStorage from middleware
  // AuthGuard client component handles redirect to /login
  if (pathname === "/") {
    return NextResponse.redirect(new URL("/workspaces", request.url));
  }

  return NextResponse.next();
}

export const config = {
  matcher: ["/((?!_next/static|_next/image|favicon.ico).*)"],
};
