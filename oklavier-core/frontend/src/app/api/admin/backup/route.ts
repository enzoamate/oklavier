import { NextRequest } from "next/server";
import { requireAdmin, apiPost, NextResponse } from "@/lib/api-proxy";

export async function POST(request: NextRequest) {
  if (!await requireAdmin()) return NextResponse.json({ error: "Forbidden" }, { status: 403 });
  const body = await request.json();
  const action = body._action;
  delete body._action;

  let path = "/api/admin/backup/export";
  if (action === "import") path = "/api/admin/backup/import";

  const res = await apiPost(path, body);
  return NextResponse.json(await res.json());
}
