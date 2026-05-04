import { requireAuth, apiPost, NextResponse } from "@/lib/api-proxy";

export async function POST(req: Request) {
  const user = await requireAuth();
  if (!user) return NextResponse.json({ error: "Not authenticated" }, { status: 401 });

  try {
    const body = await req.json();
    const res = await apiPost("/api/favorites/toggle", body);
    return NextResponse.json(await res.json());
  } catch {
    return NextResponse.json({ error: "Failed to toggle favorite" }, { status: 500 });
  }
}
