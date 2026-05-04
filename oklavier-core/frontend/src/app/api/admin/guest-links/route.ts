import { NextRequest } from "next/server";
import { requireAdmin, apiPost, NextResponse } from "@/lib/api-proxy";

export async function POST(request: NextRequest) {
  if (!await requireAdmin()) return NextResponse.json({ error: "Forbidden" }, { status: 403 });
  const body = await request.json();
  const action = body._action;
  delete body._action;

  let path = "/api/admin/guest-links";
  if (action === "create") path = "/api/admin/guest-links/create";
  else if (action === "delete") path = "/api/admin/guest-links/delete";

  const res = await apiPost(path, body);
  return NextResponse.json(await res.json());
}
