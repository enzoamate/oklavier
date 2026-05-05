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
  const auth = headerStore.get("authorization");
  if (auth) fwd["Authorization"] = auth;

  const access = cookieStore.get("oklavier_access")?.value;
  const refresh = cookieStore.get("oklavier_refresh")?.value;
  const cookieParts: string[] = [];
  if (access) cookieParts.push(`oklavier_access=${access}`);
  if (refresh) cookieParts.push(`oklavier_refresh=${refresh}`);
  if (cookieParts.length) fwd["Cookie"] = cookieParts.join("; ");

  const res = await fetch(`${API}/api/auth/logout`, {
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
