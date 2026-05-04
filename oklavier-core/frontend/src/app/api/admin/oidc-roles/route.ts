import { requireAdmin, apiPost, NextResponse } from "@/lib/api-proxy";

export async function GET() {
  if (!await requireAdmin()) return NextResponse.json({ error: "Forbidden" }, { status: 403 });
  const res = await apiPost("/api/admin/oidc-roles", {});
  return NextResponse.json(await res.json());
}
