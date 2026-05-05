import { NextResponse } from "next/server";
import { headers as nextHeaders, cookies as nextCookies } from "next/headers";

const API = process.env.OKLAVIER_API || "http://oklavier-api.oklavier.svc.cluster.local:8080";
const internalSecret = process.env.OKLAVIER_INTERNAL_SECRET || "";

export async function GET() {
  const headerStore = await nextHeaders();
  const cookieStore = await nextCookies();

  const fwd: Record<string, string> = {
    "Sec-Fetch-Site": "same-origin",
  };
  if (internalSecret) fwd["X-Oklavier-Internal"] = internalSecret;
  const auth = headerStore.get("authorization");
  if (auth) fwd["Authorization"] = auth;

  const access = cookieStore.get("oklavier_access")?.value;
  if (access) fwd["Cookie"] = `oklavier_access=${access}`;

  if (!access && !auth) {
    return NextResponse.json({ error: "Not authenticated" }, { status: 401 });
  }

  const res = await fetch(`${API}/api/auth/me`, { headers: fwd });
  const data = await res.json().catch(() => ({}));
  return NextResponse.json(data, { status: res.status });
}
