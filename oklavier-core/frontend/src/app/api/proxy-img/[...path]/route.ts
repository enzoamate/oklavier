import { NextRequest, NextResponse } from "next/server";

const OKLAVIER_API = process.env.OKLAVIER_API || "http://oklavier-api.oklavier.svc.cluster.local:8080";

export async function GET(
  request: NextRequest,
  { params }: { params: Promise<{ path: string[] }> }
) {
  const { path } = await params;
  const imgPath = path.join("/");

  try {
    const res = await fetch(`${OKLAVIER_API}/${imgPath}`, {
      headers: { Accept: "image/*,*/*" },
    });

    if (!res.ok) {
      return new NextResponse(null, { status: 404 });
    }

    const buffer = await res.arrayBuffer();
    const contentType = res.headers.get("content-type") || "image/png";

    return new NextResponse(buffer, {
      headers: {
        "Content-Type": contentType,
        "Cache-Control": "public, max-age=86400",
      },
    });
  } catch {
    return new NextResponse(null, { status: 500 });
  }
}
