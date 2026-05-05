"use client";

import { Suspense, useEffect } from "react";
import { useRouter, useSearchParams } from "next/navigation";

// Disable prerender — this page reads query params at runtime.
export const dynamic = "force-dynamic";

function CallbackInner() {
  const router = useRouter();
  const params = useSearchParams();

  useEffect(() => {
    if (params.get("oidc") === "ok") {
      router.replace("/workspaces");
    } else {
      router.replace("/login?error=oidc_failed");
    }
  }, [router, params]);

  return null;
}

export default function OIDCCallbackPage() {
  return (
    <Suspense fallback={null}>
      <div style={{ display: "flex", alignItems: "center", justifyContent: "center", height: "100vh", background: "#0f1225", color: "rgba(255,255,255,0.4)", fontFamily: "Inter, system-ui, sans-serif" }}>
        Authenticating...
      </div>
      <CallbackInner />
    </Suspense>
  );
}
