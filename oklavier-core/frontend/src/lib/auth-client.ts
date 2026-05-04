"use client";

import useSWR from "swr";
import { authFetch } from "./auth-fetch";
import { setTokens, clearTokens, getAccessToken, getRefreshToken } from "./token-store";

export async function signIn(email: string, password: string) {
  const res = await fetch("/api/auth/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email, password }),
  });
  const data = await res.json();
  if (!res.ok) throw new Error(data.error || "Login failed");
  if (data.access_token && data.refresh_token) {
    setTokens(data.access_token, data.refresh_token);
  }
  return data;
}

export async function signUp(name: string, email: string, password: string) {
  const res = await authFetch("/api/auth/signup", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name, email, password }),
  });
  const data = await res.json();
  if (!res.ok) throw new Error(data.error || "Signup failed");
  return data;
}

export async function signOut() {
  const refreshToken = getRefreshToken();
  await authFetch("/api/auth/logout", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ refresh_token: refreshToken }),
  });
  clearTokens();
}

const fetcher = async (url: string) => {
  const res = await authFetch(url);
  if (!res.ok) return null;
  return res.json();
};

export function useSession() {
  const { data, error, isLoading, mutate } = useSWR("/api/auth/me", fetcher, {
    revalidateOnFocus: true,
    revalidateOnReconnect: true,
    dedupingInterval: 5000,
  });

  return {
    data: data ? { user: data.user, session: data.session } : null,
    isPending: isLoading,
    error,
    mutate,
  };
}
