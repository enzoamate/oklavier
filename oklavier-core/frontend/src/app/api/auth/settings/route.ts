import { NextResponse } from "next/server";

const OKLAVIER_API = process.env.OKLAVIER_API || "http://oklavier-api.oklavier.svc.cluster.local:8080";

export async function GET() {
  try {
    // Cluster-internal call to the Go API. We do NOT disable certificate
    // validation; Node's default fetch validates the server cert. The
    // previous `rejectUnauthorized: false` was a no-op on Node fetch
    // anyway and tripped CodeQL's js/disabling-certificate-validation rule.
    const res = await fetch(`${OKLAVIER_API}/api/login_settings`, {
      method: "GET",
      headers: { "Content-Type": "application/json" },
    });
    const data = await res.json();
    return NextResponse.json(data);
  } catch {
    return NextResponse.json(
      { error: "Failed to fetch login settings" },
      { status: 500 }
    );
  }
}
