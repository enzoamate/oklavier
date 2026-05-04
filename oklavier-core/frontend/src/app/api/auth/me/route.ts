import { NextResponse } from "next/server";

const API = process.env.OKLAVIER_API || "http://oklavier-api.oklavier.svc.cluster.local:8080";

export async function GET(request: Request) {
  const auth = request.headers.get("authorization");
  if (!auth) return NextResponse.json({ error: "Not authenticated" }, { status: 401 });

  const res = await fetch(`${API}/api/auth/me`, {
    headers: { "Authorization": auth },
  });
  const data = await res.json().catch(() => ({}));
  return NextResponse.json(data, { status: res.status });
}
