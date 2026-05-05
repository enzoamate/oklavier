import { NextResponse } from "next/server";
import { headers as nextHeaders, cookies as nextCookies } from "next/headers";

const API = process.env.OKLAVIER_API || "http://oklavier-api.oklavier.svc.cluster.local:8080";
const internalSecret = process.env.OKLAVIER_INTERNAL_SECRET || "";

export async function POST(request: Request) {
  const body = await request.json().catch(() => ({}));
  const headerStore = await nextHeaders();
  const cookieStore = await nextCookies();

  const fwd: Record<string, string> = {
    "Content-Type": "application/json",
    "Sec-Fetch-Site": "same-origin",
  };
  if (internalSecret) fwd["X-Oklavier-Internal"] = internalSecret;
  const origin = headerStore.get("origin");
  if (origin) fwd["Origin"] = origin;

  // Forward refresh cookie to Go (the source of truth for refresh now).
  const refresh = cookieStore.get("oklavier_refresh")?.value;
  if (refresh) fwd["Cookie"] = `oklavier_refresh=${refresh}`;

  const res = await fetch(`${API}/api/auth/refresh`, {
    method: "POST",
    headers: fwd,
    body: JSON.stringify(body),
  });
  const data = await res.json().catch(() => ({}));
  const out = NextResponse.json(data, { status: res.status });
  // @ts-expect-error: getSetCookie is non-standard but Node 18+/undici implements it
  const setCookies: string[] | undefined = res.headers.getSetCookie?.();
  if (setCookies?.length) {
    for (const sc of setCookies) out.headers.append("set-cookie", sc);
  } else {
    const single = res.headers.get("set-cookie");
    if (single) out.headers.append("set-cookie", single);
  }
  return out;
}
