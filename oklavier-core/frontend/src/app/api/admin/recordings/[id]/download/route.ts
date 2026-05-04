import { NextRequest } from "next/server";
import { requireAdmin, apiGet, NextResponse } from "@/lib/api-proxy";

export async function GET(request: NextRequest, { params }: { params: Promise<{ id: string }> }) {
  if (!await requireAdmin()) return NextResponse.json({ error: "Forbidden" }, { status: 403 });
  const { id } = await params;
  const res = await apiGet(`/api/admin/recordings/${id}/download`);

  const contentType = res.headers.get("content-type") || "";
  if (contentType.includes("application/octet-stream")) {
    // Local storage — proxy binary stream directly
    const data = await res.arrayBuffer();
    return new NextResponse(data, {
      headers: {
        "Content-Type": "application/octet-stream",
        "Content-Disposition": res.headers.get("content-disposition") || `attachment; filename="${id}.guac"`,
      },
    });
  }

  // S3 storage — return JSON info
  return NextResponse.json(await res.json());
}
