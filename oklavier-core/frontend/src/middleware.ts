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
