import { NextRequest } from "next/server";
import { requireAdmin, apiPost, NextResponse } from "@/lib/api-proxy";

export async function POST(request: NextRequest) {
  if (!await requireAdmin()) return NextResponse.json({ error: "Forbidden" }, { status: 403 });

  const body = await request.json();

  // Step 1: Create agent in DB (get token)
  const createRes = await apiPost("/api/admin/agents/create", {
    name: body.name,
    region: body.region,
    namespace: body.namespace,
  });
  const agentData = await createRes.json();

  if (!agentData.token) {
    return NextResponse.json({ error: "Failed to create agent" }, { status: 500 });
  }

  // Step 2: Deploy agent via Go API
  const deployRes = await apiPost("/api/admin/agents/deploy", {
    agent_id: agentData.id,
    token: agentData.token,
    name: body.name,
    region: body.region,
    namespace: body.namespace,
    mode: body.mode,
    kubeconfig: body.kubeconfig || "",
    control_plane: body.control_plane || process.env.NEXT_PUBLIC_OKLAVIER_PUBLIC_URL || "",
  });
  const deployData = await deployRes.json();

  if (deployData.error) {
    return NextResponse.json({ error: deployData.error }, { status: 500 });
  }

  return NextResponse.json({ ok: true, agent_id: agentData.id });
}
