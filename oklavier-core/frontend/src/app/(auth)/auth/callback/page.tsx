"use client";

import { useEffect } from "react";
import { useRouter, useSearchParams } from "next/navigation";

// OIDC callback. Tokens are now set by the Go backend as httpOnly cookies
// during /api/auth/oidc/<id>/callback (server-side redirect). This page
// just navigates to the dashboard once.
export default function OIDCCallbackPage() {
  const router = useRouter();
  const params = useSearchParams();

  useEffect(() => {
    if (params.get("oidc") === "ok") {
      router.replace("/workspaces");
    } else {
      router.replace("/login?error=oidc_failed");
    }
  }, [router, params]);

  return (
    <div style={{ display: "flex", alignItems: "center", justifyContent: "center", height: "100vh", background: "#0f1225", color: "rgba(255,255,255,0.4)", fontFamily: "Inter, system-ui, sans-serif" }}>
      Authenticating...
    </div>
  );
}
