import { NextResponse } from "next/server";
import { headers as nextHeaders } from "next/headers";

const API = process.env.OKLAVIER_API || "http://oklavier-api.oklavier.svc.cluster.local:8080";
const internalSecret = process.env.OKLAVIER_INTERNAL_SECRET || "";

export async function POST(request: Request) {
  const body = await request.json();
  const headerStore = await nextHeaders();

  const fwd: Record<string, string> = {
    "Content-Type": "application/json",
    "Sec-Fetch-Site": "same-origin",
  };
  if (internalSecret) fwd["X-Oklavier-Internal"] = internalSecret;
  const origin = headerStore.get("origin");
  if (origin) fwd["Origin"] = origin;

  const res = await fetch(`${API}/api/auth/login`, {
    method: "POST",
    headers: fwd,
    body: JSON.stringify(body),
  });
  const data = await res.json().catch(() => ({}));
  const out = NextResponse.json(data, { status: res.status });

  // Forward Set-Cookie from Go API to the browser (httpOnly auth cookies).
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
