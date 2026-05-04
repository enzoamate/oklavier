import { NextRequest, NextResponse } from "next/server";
import { requireAuth, apiPost } from "@/lib/api-proxy";

export async function POST(request: NextRequest) {
  const user = await requireAuth();
  if (!user) return NextResponse.json({ error: "Not authenticated" }, { status: 401 });
  const body = await request.json();
  const res = await apiPost("/api/auth/update-profile", body);
  const data = await res.json().catch(() => ({}));
  return NextResponse.json(data, { status: res.status });
}
