import { requireAuth, apiPost, NextResponse } from "@/lib/api-proxy";

export async function POST() {
  const user = await requireAuth();
  if (!user) return NextResponse.json({ error: "Not authenticated" }, { status: 401 });

  try {
    const res = await apiPost("/api/favorites", {});
    return NextResponse.json(await res.json());
  } catch {
    return NextResponse.json({ error: "Failed to fetch favorites" }, { status: 500 });
  }
}
