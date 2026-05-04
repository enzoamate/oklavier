import { NextResponse } from "next/server";

// BetterAuth catch-all route removed - auth is now handled by the Go API
export async function GET() {
  return NextResponse.json({ error: "Not found" }, { status: 404 });
}

export async function POST() {
  return NextResponse.json({ error: "Not found" }, { status: 404 });
}
