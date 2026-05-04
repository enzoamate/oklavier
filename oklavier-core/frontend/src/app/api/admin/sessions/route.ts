import { NextRequest } from "next/server";
import { requireAdmin, apiPost, NextResponse } from "@/lib/api-proxy";

export async function GET(request: NextRequest) {
  if (!await requireAdmin()) return NextResponse.json({ error: "Forbidden" }, { status: 403 });

  const { searchParams } = new URL(request.url);
  const body = {
    page: parseInt(searchParams.get("page") || "1"),
    per_page: parseInt(searchParams.get("per_page") || "1000"),
    search: searchParams.get("search") || "",
  };
  const res = await apiPost("/api/admin/sessions", body);
  return NextResponse.json(await res.json());
}
