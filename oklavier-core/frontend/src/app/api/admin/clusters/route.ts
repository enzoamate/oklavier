import { NextRequest } from "next/server";
import { requireAdmin, apiPost, NextResponse } from "@/lib/api-proxy";

export async function GET() {
  if (!await requireAdmin()) return NextResponse.json({ error: "Forbidden" }, { status: 403 });
  const res = await apiPost("/api/admin/clusters");
  return NextResponse.json(await res.json());
}

export async function POST(request: NextRequest) {
  if (!await requireAdmin()) return NextResponse.json({ error: "Forbidden" }, { status: 403 });
  const body = await request.json();
  const res = await apiPost("/api/admin/clusters/create", body);
  return NextResponse.json(await res.json());
}

export async function PUT(request: NextRequest) {
  if (!await requireAdmin()) return NextResponse.json({ error: "Forbidden" }, { status: 403 });
  const body = await request.json();
  const res = await apiPost("/api/admin/clusters/update", body);
  return NextResponse.json(await res.json());
}

export async function DELETE(request: NextRequest) {
  if (!await requireAdmin()) return NextResponse.json({ error: "Forbidden" }, { status: 403 });
  const body = await request.json();
  const res = await apiPost("/api/admin/clusters/delete", body);
  return NextResponse.json(await res.json());
}

export async function PATCH(request: NextRequest) {
  if (!await requireAdmin()) return NextResponse.json({ error: "Forbidden" }, { status: 403 });
  const body = await request.json();
  const res = await apiPost("/api/admin/clusters/toggle-default", body);
  return NextResponse.json(await res.json());
}
