"use client";

import { useEffect } from "react";
import { setTokens } from "@/lib/token-store";
import { useRouter } from "next/navigation";

export default function OIDCCallbackPage() {
  const router = useRouter();

  useEffect(() => {
    const hash = window.location.hash.substring(1);
    const params = new URLSearchParams(hash);
    const access = params.get("access_token");
    const refresh = params.get("refresh_token");
    if (access && refresh) {
      setTokens(access, refresh);
      history.replaceState(null, "", window.location.pathname);
      router.replace("/workspaces");
    } else {
      router.replace("/login?error=oidc_failed");
    }
  }, [router]);

  return (
    <div style={{ display: "flex", alignItems: "center", justifyContent: "center", height: "100vh", background: "#0f1225", color: "rgba(255,255,255,0.4)", fontFamily: "Inter, system-ui, sans-serif" }}>
      Authenticating...
    </div>
  );
}
