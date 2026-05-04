import { NextRequest } from "next/server";
import { requireAuth, apiPost, NextResponse } from "@/lib/api-proxy";

export async function POST(request: NextRequest, { params }: { params: Promise<{ sessionId: string }> }) {
  if (!await requireAuth()) return NextResponse.json({ error: "Not authenticated" }, { status: 401 });
  const { sessionId } = await params;
  const body = await request.json();
  const res = await apiPost(`/api/proxy/webrtc/ice/${sessionId}`, body);
  return NextResponse.json(await res.json(), { status: res.status });
}
