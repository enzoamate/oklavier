"use client";

// authFetch wraps fetch with the cookie-auth flow:
//  - `credentials: "include"` so the browser attaches the auth cookies.
//  - On 401, automatically POST /api/auth/refresh once; if it succeeds,
//    retry the original request.

let refreshPromise: Promise<boolean> | null = null;

async function doRefresh(): Promise<boolean> {
  try {
    const res = await fetch("/api/auth/refresh", {
      method: "POST",
      credentials: "include",
      headers: { "Content-Type": "application/json" },
      body: "{}",
    });
    return res.ok;
  } catch {
    return false;
  }
}

export async function authFetch(url: string, init?: RequestInit): Promise<Response> {
  let res = await fetch(url, { ...init, credentials: "include" });

  if (res.status === 401) {
    if (!refreshPromise) {
      refreshPromise = doRefresh().finally(() => { refreshPromise = null; });
    }
    const ok = await refreshPromise;
    if (ok) {
      res = await fetch(url, { ...init, credentials: "include" });
    } else if (typeof window !== "undefined" && !window.location.pathname.startsWith("/login")) {
      window.location.href = "/login";
    }
  }
  return res;
}
