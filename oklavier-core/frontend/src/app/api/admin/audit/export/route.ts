import { NextRequest } from "next/server";
import { requireAdmin, apiPost, NextResponse } from "@/lib/api-proxy";

export async function POST(request: NextRequest) {
  if (!await requireAdmin()) return NextResponse.json({ error: "Forbidden" }, { status: 403 });
  const body = await request.json();
  const res = await apiPost("/api/admin/audit/export", body);

  const csv = await res.text();
  return new NextResponse(csv, {
    status: 200,
    headers: {
      "Content-Type": "text/csv",
      "Content-Disposition": "attachment; filename=audit-log.csv",
    },
  });
}
