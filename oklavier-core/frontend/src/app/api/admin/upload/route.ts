import { NextRequest } from "next/server";
import { requireAdmin, NextResponse } from "@/lib/api-proxy";
import { headers } from "next/headers";

const API = process.env.OKLAVIER_API || "http://oklavier-api.oklavier.svc.cluster.local:8080";

export async function POST(request: NextRequest) {
  if (!await requireAdmin()) return NextResponse.json({ error: "Forbidden" }, { status: 403 });

  const headerStore = await headers();
  const formData = await request.formData();

  const fwdHeaders: Record<string, string> = {};
  const auth = headerStore.get("authorization");
  if (auth) {
    fwdHeaders["Authorization"] = auth;
  }
  const internalSecret = process.env.OKLAVIER_INTERNAL_SECRET || "";
  if (internalSecret) {
    fwdHeaders["X-Oklavier-Internal"] = internalSecret;
  }

  const res = await fetch(`${API}/api/admin/upload`, {
    method: "POST",
    headers: fwdHeaders,
    body: formData,
  });

  return NextResponse.json(await res.json(), { status: res.status });
}
