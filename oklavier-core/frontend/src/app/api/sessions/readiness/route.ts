import { NextRequest } from "next/server";
import { requireAuth, apiPost, NextResponse } from "@/lib/api-proxy";

export async function POST(request: NextRequest) {
  if (!await requireAuth()) return NextResponse.json({ error: "Not authenticated" }, { status: 401 });

  const { session_id } = await request.json();

  const res = await apiPost("/api/session_readiness", { session_id });

  return NextResponse.json(await res.json());
}
