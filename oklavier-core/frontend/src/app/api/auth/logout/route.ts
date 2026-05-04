import { NextResponse } from "next/server";

const API = process.env.OKLAVIER_API || "http://oklavier-api.oklavier.svc.cluster.local:8080";

export async function POST(request: Request) {
  const auth = request.headers.get("authorization") || "";
  const body = await request.json().catch(() => ({}));
  const res = await fetch(`${API}/api/auth/logout`, {
    method: "POST",
    headers: { "Content-Type": "application/json", "Authorization": auth },
    body: JSON.stringify(body),
  });
  const data = await res.json().catch(() => ({}));
  return NextResponse.json(data, { status: res.status });
}
