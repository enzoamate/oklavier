import { NextResponse } from "next/server";

const API = process.env.OKLAVIER_API || "http://oklavier-api.oklavier.svc.cluster.local:8080";
const internalSecret = process.env.OKLAVIER_INTERNAL_SECRET || "";

export async function GET() {
  try {
    const headers: Record<string, string> = {};
    if (internalSecret) {
      headers["X-Oklavier-Internal"] = internalSecret;
    }
    const res = await fetch(`${API}/api/auth/providers`, { headers });
    if (!res.ok) {
      return NextResponse.json({ providers: [] });
    }
    const data = await res.json();
    return NextResponse.json(data);
  } catch {
    return NextResponse.json({ providers: [] });
  }
}
