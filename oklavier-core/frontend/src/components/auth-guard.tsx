"use client";

import { useSession } from "@/lib/auth-client";
import { useRouter } from "next/navigation";
import { useEffect } from "react";

export function AuthGuard({ children }: { children: React.ReactNode }) {
  const { data, isPending } = useSession();
  const router = useRouter();

  useEffect(() => {
    if (!isPending && !data) {
      router.replace("/login");
    }
  }, [data, isPending, router]);

  if (isPending || !data) return null;
  return <>{children}</>;
}
