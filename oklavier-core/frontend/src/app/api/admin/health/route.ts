import { requireAdmin, apiPost, NextResponse } from "@/lib/api-proxy";

export async function POST() {
  if (!await requireAdmin()) return NextResponse.json({ error: "Forbidden" }, { status: 403 });
  const res = await apiPost("/api/admin/health", {});
  return NextResponse.json(await res.json());
}
