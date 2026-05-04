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
  const res = await apiPost("/api/admin/users", body);
  return NextResponse.json(await res.json(), { status: res.status });
}

export async function POST(request: NextRequest) {
  if (!await requireAdmin()) return NextResponse.json({ error: "Forbidden" }, { status: 403 });
  const body = await request.json();
  const res = await apiPost("/api/admin/signup", body);
  return NextResponse.json(await res.json(), { status: res.status });
}

export async function DELETE(request: NextRequest) {
  if (!await requireAdmin()) return NextResponse.json({ error: "Forbidden" }, { status: 403 });
  const body = await request.json();
  const res = await apiPost("/api/admin/users/delete", body);
  return NextResponse.json(await res.json(), { status: res.status });
}

export async function PATCH(request: NextRequest) {
  if (!await requireAdmin()) return NextResponse.json({ error: "Forbidden" }, { status: 403 });
  const body = await request.json();
  const endpoint = body.action === "reset-password" ? "/api/admin/users/reset-password" : "/api/admin/users/update";
  const res = await apiPost(endpoint, body);
  return NextResponse.json(await res.json(), { status: res.status });
}
