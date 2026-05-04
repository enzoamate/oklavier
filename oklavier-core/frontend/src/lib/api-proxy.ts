import { NextResponse } from "next/server";
import { headers } from "next/headers";

const API = process.env.OKLAVIER_API || "http://oklavier-api.oklavier.svc.cluster.local:8080";
const internalSecret = process.env.OKLAVIER_INTERNAL_SECRET || "";

// Forward Authorization header from incoming request to Go API
async function getForwardHeaders(): Promise<Record<string, string>> {
  const headerStore = await headers();
  const h: Record<string, string> = { "Content-Type": "application/json" };

  if (internalSecret) {
    h["X-Oklavier-Internal"] = internalSecret;
  }

  const auth = headerStore.get("authorization");
  if (auth) {
    h["Authorization"] = auth;
  }
  return h;
}

export async function requireAdmin() {
  const h = await getForwardHeaders();
  const res = await fetch(`${API}/api/auth/me`, { headers: h });
  if (!res.ok) return null;
  const data = await res.json();
  if (data?.user?.role !== "admin") return null;
  return data.user;
}

export async function requireAuth() {
  const h = await getForwardHeaders();
  const res = await fetch(`${API}/api/auth/me`, { headers: h });
  if (!res.ok) return null;
  const data = await res.json();
  return data?.user || null;
}

export async function apiPost(path: string, body: any = {}) {
  const h = await getForwardHeaders();
  return fetch(`${API}${path}`, { method: "POST", headers: h, body: JSON.stringify(body) });
}

export async function apiGet(path: string) {
  const h = await getForwardHeaders();
  return fetch(`${API}${path}`, { method: "GET", headers: h });
}

export { API, NextResponse };
