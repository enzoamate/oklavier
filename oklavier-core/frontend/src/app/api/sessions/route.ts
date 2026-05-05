import { NextRequest } from "next/server";
import { requireAuth, apiPost, NextResponse } from "@/lib/api-proxy";

export async function GET() {
  const user = await requireAuth();
  if (!user) return NextResponse.json({ error: "Not authenticated" }, { status: 401 });

  try {
    const res = await apiPost(`/api/get_user_sessions?user_id=${user.id}`, {});
    const data = await res.json();
    return NextResponse.json(data);
  } catch {
    return NextResponse.json({ sessions: [] });
  }
}

export async function POST(request: NextRequest) {
  const user = await requireAuth();
  if (!user) return NextResponse.json({ error: "Not authenticated" }, { status: 401 });

  const { action, image_id, session_id, lang, server_username, server_password, server_domain } = await request.json();

  let endpoint = "";
  const body: Record<string, unknown> = {};

  if (action === "create") {
    endpoint = "request_session";
    body.image_id = image_id;
    body.user_id = user.id;
    if (lang) body.lang = lang;
    if (server_username) body.server_username = server_username;
    if (server_password) body.server_password = server_password;
    if (server_domain) body.server_domain = server_domain;
  } else if (action === "destroy") {
    endpoint = "destroy_session";
    body.session_id = session_id;
  } else if (action === "connect") {
    endpoint = "session/connect";
    body.session_id = session_id;
  } else {
    return NextResponse.json({ error: "Invalid action" }, { status: 400 });
  }

  try {
    const res = await apiPost(`/api/${endpoint}`, body);
    const data = await res.json();
    return NextResponse.json(data);
  } catch {
    return NextResponse.json({ error: "Failed" }, { status: 500 });
  }
}
