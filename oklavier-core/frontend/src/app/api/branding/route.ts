import { NextResponse } from "next/server";

const API = process.env.OKLAVIER_API || "http://oklavier-api.oklavier.svc.cluster.local:8080";

export async function GET() {
  try {
    const res = await fetch(`${API}/api/branding`);
    return NextResponse.json(await res.json());
  } catch {
    return NextResponse.json({ app_name: "Oklavier", primary_color: "#4F46E5", accent_color: "#F97316" });
  }
}
