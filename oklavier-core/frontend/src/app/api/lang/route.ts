import { NextRequest, NextResponse } from "next/server";

export async function POST(request: NextRequest) {
  const { lang } = await request.json();
  const validLangs = ["fr", "en", "es", "de"];
  if (!validLangs.includes(lang)) {
    return NextResponse.json({ error: "Invalid language" }, { status: 400 });
  }

  const response = NextResponse.json({ ok: true, lang });
  response.cookies.set("oklavier_lang", lang, {
    path: "/",
    maxAge: 60 * 60 * 24 * 365,
    sameSite: "lax",
  });
  return response;
}
