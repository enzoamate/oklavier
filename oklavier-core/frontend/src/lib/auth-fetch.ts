"use client";

import { getAccessToken, getRefreshToken, setTokens, clearTokens } from "./token-store";

let refreshPromise: Promise<boolean> | null = null;

async function doRefresh(): Promise<boolean> {
  const refresh = getRefreshToken();
  if (!refresh) return false;
  try {
    const res = await fetch("/api/auth/refresh", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ refresh_token: refresh }),
    });
    if (!res.ok) {
      clearTokens();
      return false;
    }
    const data = await res.json();
    setTokens(data.access_token, data.refresh_token);
    return true;
  } catch {
    clearTokens();
    return false;
  }
}

export async function authFetch(url: string, init?: RequestInit): Promise<Response> {
  const token = getAccessToken();
  const headers = new Headers(init?.headers);
  if (token) headers.set("Authorization", `Bearer ${token}`);

  let res = await fetch(url, { ...init, headers });

  if (res.status === 401 && getRefreshToken()) {
    if (!refreshPromise) {
      refreshPromise = doRefresh().finally(() => { refreshPromise = null; });
    }
    const ok = await refreshPromise;
    if (ok) {
      const newHeaders = new Headers(init?.headers);
      newHeaders.set("Authorization", `Bearer ${getAccessToken()}`);
      res = await fetch(url, { ...init, headers: newHeaders });
    } else if (typeof window !== "undefined") {
      window.location.href = "/login";
    }
  }
  return res;
}
