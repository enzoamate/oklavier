import { NextRequest } from "next/server";
import { requireAdmin, apiPost, NextResponse } from "@/lib/api-proxy";

export async function GET() {
  if (!await requireAdmin()) return NextResponse.json({ error: "Forbidden" }, { status: 403 });
  const res = await apiPost("/api/admin/auth-methods");
  return NextResponse.json(await res.json());
}

export async function POST(request: NextRequest) {
  if (!await requireAdmin()) return NextResponse.json({ error: "Forbidden" }, { status: 403 });
  const body = await request.json();
  const res = await apiPost("/api/admin/auth-methods/create", body);
  return NextResponse.json(await res.json());
}

export async function PUT(request: NextRequest) {
  if (!await requireAdmin()) return NextResponse.json({ error: "Forbidden" }, { status: 403 });
  const body = await request.json();
  const res = await apiPost("/api/admin/auth-methods/update", body);
  return NextResponse.json(await res.json());
}

export async function DELETE(request: NextRequest) {
  if (!await requireAdmin()) return NextResponse.json({ error: "Forbidden" }, { status: 403 });
  const body = await request.json();
  const res = await apiPost("/api/admin/auth-methods/delete", body);
  return NextResponse.json(await res.json());
}

export async function PATCH(request: NextRequest) {
  if (!await requireAdmin()) return NextResponse.json({ error: "Forbidden" }, { status: 403 });
  const body = await request.json();
  const res = await apiPost("/api/admin/auth-methods/toggle", body);
  return NextResponse.json(await res.json());
}
