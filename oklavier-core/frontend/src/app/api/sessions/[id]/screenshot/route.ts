import { NextResponse } from "next/server";
import { headers as nextHeaders, cookies as nextCookies } from "next/headers";

const API = process.env.OKLAVIER_API || "http://oklavier-api.oklavier.svc.cluster.local:8080";
const internalSecret = process.env.OKLAVIER_INTERNAL_SECRET || "";

// GET /api/sessions/:id/screenshot — proxy the dashboard thumbnail.
// Auth is the user cookie (oklavier_access). The Go core checks ownership,
// mints a short-lived bearer for the agent, fetches the PNG, and streams
// it back. The browser never holds an agent bearer.
export async function GET(
  _request: Request,
  { params }: { params: Promise<{ id: string }> }
) {
  const { id } = await params;
  if (!/^[A-Za-z0-9-]+$/.test(id)) {
    return new NextResponse(null, { status: 400 });
  }
  const headerStore = await nextHeaders();
  const cookieStore = await nextCookies();

  const fwd: Record<string, string> = { "Sec-Fetch-Site": "same-origin" };
  if (internalSecret) fwd["X-Oklavier-Internal"] = internalSecret;
  const auth = headerStore.get("authorization");
  if (auth) fwd["Authorization"] = auth;
  const access = cookieStore.get("oklavier_access")?.value;
  if (access) fwd["Cookie"] = `oklavier_access=${access}`;

  const res = await fetch(`${API}/api/sessions/${encodeURIComponent(id)}/screenshot`, { headers: fwd });
  const buf = await res.arrayBuffer();
  return new NextResponse(buf, {
    status: res.status,
    headers: {
      "Content-Type": "image/png",
      "Cache-Control": "no-store",
      "X-Content-Type-Options": "nosniff",
    },
  });
}
