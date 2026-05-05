import { NextRequest, NextResponse } from "next/server";

const OKLAVIER_API = process.env.OKLAVIER_API || "http://oklavier-api.oklavier.svc.cluster.local:8080";

// SECURITY: previously this route concatenated user path segments to a fixed
// host with default redirect-following — meaning an open-redirect on the
// upstream API would have turned this into a real SSRF, and any path could
// be probed. We now:
//  - allowlist a small set of upstream prefixes (only branding/avatar paths),
//  - reject `..`, `?`, `#`, `\` and absolute paths,
//  - cap response size,
//  - disable redirect following.
const ALLOWED_PREFIXES = [
  "img/",
  "api/branding/",
  "api/avatars/",
  "api/proxy-img/",
];
const MAX_BYTES = 5 * 1024 * 1024;

export async function GET(
  request: NextRequest,
  { params }: { params: Promise<{ path: string[] }> }
) {
  const { path } = await params;
  if (!path?.length) {
    return new NextResponse(null, { status: 400 });
  }
  for (const seg of path) {
    if (
      !seg ||
      seg.includes("..") ||
      seg.includes("?") ||
      seg.includes("#") ||
      seg.includes("\\") ||
      seg.startsWith("/")
    ) {
      return new NextResponse(null, { status: 400 });
    }
  }
  const imgPath = path.join("/");
  if (!ALLOWED_PREFIXES.some(p => imgPath.startsWith(p))) {
    return new NextResponse(null, { status: 403 });
  }

  try {
    const res = await fetch(`${OKLAVIER_API}/${imgPath}`, {
      headers: { Accept: "image/*" },
      redirect: "manual",
    });

    if (res.status >= 300 && res.status < 400) {
      return new NextResponse(null, { status: 502 });
    }
    if (!res.ok) {
      return new NextResponse(null, { status: 404 });
    }

    const contentType = res.headers.get("content-type") || "";
    if (!contentType.startsWith("image/")) {
      return new NextResponse(null, { status: 415 });
    }
    const buffer = await res.arrayBuffer();
    if (buffer.byteLength > MAX_BYTES) {
      return new NextResponse(null, { status: 413 });
    }

    return new NextResponse(buffer, {
      headers: {
        "Content-Type": contentType,
        "Cache-Control": "public, max-age=86400",
        "X-Content-Type-Options": "nosniff",
      },
    });
  } catch {
    return new NextResponse(null, { status: 500 });
  }
}
