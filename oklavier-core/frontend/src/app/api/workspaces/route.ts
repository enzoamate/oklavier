import { requireAuth, apiPost, NextResponse } from "@/lib/api-proxy";

export async function GET() {
  const user = await requireAuth();
  if (!user) return NextResponse.json({ error: "Not authenticated" }, { status: 401 });

  const isAdmin = (user as any).role === "admin";

  try {
    const res = await apiPost(`/api/get_user_images?user_id=${user.id}&is_admin=${isAdmin}`, {});
    const data = await res.json();
    return NextResponse.json(data);
  } catch {
    return NextResponse.json({ error: "Failed to fetch workspaces" }, { status: 500 });
  }
}
