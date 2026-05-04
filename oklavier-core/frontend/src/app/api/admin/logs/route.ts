import { NextRequest } from "next/server";
import { requireAdmin, apiGet, NextResponse } from "@/lib/api-proxy";

export async function GET(request: NextRequest) {
  if (!await requireAdmin()) return NextResponse.json({ error: "Forbidden" }, { status: 403 });

  const source = request.nextUrl.searchParams.get("source") || "all";
  const limit = request.nextUrl.searchParams.get("limit") || "100";

  const res = await apiGet(`/api/admin/logs?source=${source}&limit=${limit}`);
  return NextResponse.json(await res.json());
}
