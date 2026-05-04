import useSWR, { mutate as globalMutate } from "swr";
import { authFetch } from "./auth-fetch";

const fetcher = async (url: string) => {
  const res = await authFetch(url, { cache: "no-store" });
  if (!res.ok) throw new Error(`${res.status}`);
  return res.json();
};

const postFetcher = async (url: string) => {
  const res = await authFetch(url, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: "{}",
    cache: "no-store",
  });
  if (!res.ok) throw new Error(`${res.status}`);
  return res.json();
};

/** GET-based SWR hook */
export function useAPI<T = any>(path: string | null, opts?: { refreshInterval?: number; revalidateOnFocus?: boolean }) {
  return useSWR<T>(path, fetcher, {
    revalidateOnFocus: false,
    dedupingInterval: 2000,
    keepPreviousData: true,
    ...opts,
  });
}

/** POST-based SWR hook (for APIs that use POST for reads) */
export function usePostAPI<T = any>(path: string | null, opts?: { refreshInterval?: number }) {
  return useSWR<T>(path ? `POST:${path}` : null, () => postFetcher(path!), {
    revalidateOnFocus: false,
    dedupingInterval: 2000,
    ...opts,
  });
}

/** Mutate/invalidate a key to trigger refetch */
export function invalidate(path: string) {
  globalMutate(path);
  globalMutate(`POST:${path}`);
}

/** POST action (create/update/delete) with error handling */
export async function apiAction(
  path: string,
  method: string,
  body?: Record<string, unknown>
): Promise<{ ok: boolean; data?: any; error?: string }> {
  try {
    const res = await authFetch(path, {
      method,
      headers: { "Content-Type": "application/json" },
      body: body ? JSON.stringify(body) : undefined,
    });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) return { ok: false, error: data?.error || `HTTP ${res.status}` };
    return { ok: true, data };
  } catch (e) {
    return { ok: false, error: (e as Error).message };
  }
}
