import { NextResponse } from "next/server";
import { headers as nextHeaders, cookies as nextCookies } from "next/headers";

const API = process.env.OKLAVIER_API || "http://oklavier-api.oklavier.svc.cluster.local:8080";
const internalSecret = process.env.OKLAVIER_INTERNAL_SECRET || "";

// getForwardHeaders forwards the auth cookies (oklavier_access /
// oklavier_refresh) AND the Authorization header (legacy/automation) plus
// the Sec-Fetch-Site / Origin signals so the Go API's CSRF guard can
// recognize this as a same-site request.
async function getForwardHeaders(): Promise<Record<string, string>> {
  const headerStore = await nextHeaders();
  const cookieStore = await nextCookies();
  const h: Record<string, string> = { "Content-Type": "application/json" };

  if (internalSecret) {
    h["X-Oklavier-Internal"] = internalSecret;
  }

  const auth = headerStore.get("authorization");
  if (auth) h["Authorization"] = auth;

  // Forward auth cookies to the Go API.
  const access = cookieStore.get("oklavier_access")?.value;
  const refresh = cookieStore.get("oklavier_refresh")?.value;
  const cookieParts: string[] = [];
  if (access) cookieParts.push(`oklavier_access=${access}`);
  if (refresh) cookieParts.push(`oklavier_refresh=${refresh}`);
  if (cookieParts.length) h["Cookie"] = cookieParts.join("; ");

  // Mark as same-origin so the Go CSRF guard accepts mutating calls coming
  // through this BFF.
  const origin = headerStore.get("origin");
  if (origin) h["Origin"] = origin;
  h["Sec-Fetch-Site"] = "same-origin";

  return h;
}

// proxyResponse turns a fetch Response from the Go API into a NextResponse
// that preserves any Set-Cookie headers the Go API issued (login, refresh,
// logout). Without this, the auth cookies set by the Go backend would never
// reach the browser.
async function proxyResponse(res: Response): Promise<NextResponse> {
  const data = await res.json().catch(() => ({}));
  const out = NextResponse.json(data, { status: res.status });
  // Fetch's Headers iterator (Web standard) supports getSetCookie() in modern
  // Node. Fallback to the raw header otherwise.
  const setCookies: string[] | undefined = res.headers.getSetCookie?.();
  if (setCookies && setCookies.length) {
    for (const sc of setCookies) {
      out.headers.append("set-cookie", sc);
    }
  } else {
    const single = res.headers.get("set-cookie");
    if (single) out.headers.append("set-cookie", single);
  }
  return out;
}

export async function requireAdmin() {
  const h = await getForwardHeaders();
  const res = await fetch(`${API}/api/auth/me`, { headers: h });
  if (!res.ok) return null;
  const data = await res.json();
  if (data?.user?.role !== "admin") return null;
  return data.user;
}

export async function requireAuth() {
  const h = await getForwardHeaders();
  const res = await fetch(`${API}/api/auth/me`, { headers: h });
  if (!res.ok) return null;
  const data = await res.json();
  return data?.user || null;
}

export async function apiPost(path: string, body: unknown = {}) {
  const h = await getForwardHeaders();
  return fetch(`${API}${path}`, { method: "POST", headers: h, body: JSON.stringify(body) });
}

export async function apiGet(path: string) {
  const h = await getForwardHeaders();
  return fetch(`${API}${path}`, { method: "GET", headers: h });
}

export { API, NextResponse, proxyResponse };
