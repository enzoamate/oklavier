import { NextResponse } from "next/server";

const OKLAVIER_API = process.env.OKLAVIER_API || "http://oklavier-api.oklavier.svc.cluster.local:8080";

export async function GET() {
  try {
    const res = await fetch(`${OKLAVIER_API}/api/login_settings`, {
      method: "GET",
      headers: { "Content-Type": "application/json" },
      // @ts-expect-error - Node fetch supports this
      rejectUnauthorized: false,
    });
    const data = await res.json();
    return NextResponse.json(data);
  } catch (error) {
    return NextResponse.json(
      { error: "Failed to fetch login settings" },
      { status: 500 }
    );
  }
}
