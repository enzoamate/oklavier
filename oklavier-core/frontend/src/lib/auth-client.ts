"use client";

import useSWR from "swr";
import { authFetch } from "./auth-fetch";

export async function signIn(email: string, password: string) {
  const res = await fetch("/api/auth/login", {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email, password }),
  });
  const data = await res.json();
  if (!res.ok) throw new Error(data.error || "Login failed");
  // Auth cookies are set by the backend Set-Cookie response.
  return data;
}

export async function signUp(name: string, email: string, password: string) {
  const res = await fetch("/api/auth/signup", {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name, email, password }),
  });
  const data = await res.json();
  if (!res.ok) throw new Error(data.error || "Signup failed");
  return data;
}

export async function signOut() {
  await authFetch("/api/auth/logout", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: "{}",
  });
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
