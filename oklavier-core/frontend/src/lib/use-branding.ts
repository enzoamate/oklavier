"use client";

import useSWR from "swr";

export interface Branding {
  app_name: string;
  logo_url: string;
  favicon_url: string;
  creator: string;
  creator_url: string;
  primary_color: string;
  accent_color: string;
  login_bg: string;
  version: string;
}

const defaultBranding: Branding = {
  app_name: "Oklavier",
  logo_url: "",
  favicon_url: "",
  creator: "",
  creator_url: "",
  primary_color: "#4F46E5",
  accent_color: "#F97316",
  login_bg: "",
  version: "",
};

const fetcher = (url: string) => fetch(url).then(r => r.json()).catch(() => defaultBranding);

export function useBranding() {
  const { data, isLoading } = useSWR<Branding>("/api/branding", fetcher, {
    revalidateOnFocus: true,
    dedupingInterval: 5000,
    refreshInterval: 30000,
  });
  return { branding: data || defaultBranding, isLoading };
}
