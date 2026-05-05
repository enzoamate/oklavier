"use client";

import { useState, useEffect } from "react";
import { useParams } from "next/navigation";
import { Loader2, Monitor, Lock, AlertCircle, ExternalLink } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { OklavierLogo } from "@/components/oklavier-logo";
import { LangSwitcher } from "@/components/lang-switcher";
import { useTranslation } from "@/lib/i18n";
import { useBranding } from "@/lib/use-branding";

function GuestPage() {
  const { t } = useTranslation();
  const { branding } = useBranding();
  const params = useParams();
  const token = params.token as string;

  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [info, setInfo] = useState<any>(null);
  const [password, setPassword] = useState("");
  const [serverUsername, setServerUsername] = useState("");
  const [serverPassword, setServerPassword] = useState("");
  const [serverDomain, setServerDomain] = useState("");
  const [launching, setLaunching] = useState(false);
  const [passwordError, setPasswordError] = useState(false);

  useEffect(() => {
    fetch(`/api/guest/${token}`)
      .then(async (res) => {
        const data = await res.json();
        if (!res.ok) {
          setError(data.error || "Link not found");
        } else {
          setInfo(data);
        }
        setLoading(false);
      })
      .catch(() => {
        setError("Failed to load");
        setLoading(false);
      });
  }, [token]);

  async function handleLaunch() {
    setLaunching(true);
    setPasswordError(false);
    try {
      const lang = document.cookie.match(/oklavier_lang=([^;]+)/)?.[1] || navigator.language?.split("-")[0] || "en";
      const res = await fetch(`/api/guest/${token}`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          password,
          lang,
          server_username: serverUsername,
          server_password: serverPassword,
          server_domain: serverDomain,
        }),
      });
      const data = await res.json();
      if (!res.ok) {
        if (res.status === 401) {
          setPasswordError(true);
        } else {
          setError(data.error || "Failed to create session");
        }
        setLaunching(false);
        return;
      }

      // Redirect to the agent viewer.
      // SECURITY: validate `agent_url` is a real https:// URL. Without this,
      // a server-side bug (or a malicious admin) returning `//evil.com` or
      // `https://evil.com#` would navigate the browser to attacker.com WITH
      // the live session_token in the URL, leaking the credential.
      if (data.agent_url && data.session_id) {
        let viewerUrl: string;
        try {
          const base = new URL(data.agent_url);
          if (base.protocol !== "https:") throw new Error("non-https agent_url");
          base.pathname = `${base.pathname.replace(/\/+$/, "")}/viewer/${encodeURIComponent(data.session_id)}`;
          base.search = `?token=${encodeURIComponent(data.session_token)}`;
          viewerUrl = base.toString();
        } catch {
          setError("Invalid agent URL");
          setLaunching(false);
          return;
        }
        window.location.href = viewerUrl;
      } else {
        setError("No agent available");
        setLaunching(false);
      }
    } catch {
      setError("Connection failed");
      setLaunching(false);
    }
  }

  if (loading) {
    return (
      <div className="min-h-screen bg-[#0f1225] flex items-center justify-center">
        <Loader2 className="size-8 animate-spin text-oklavier-blue" />
      </div>
    );
  }

  if (error) {
    return (
      <div className="min-h-screen bg-[#0f1225] flex items-center justify-center">
        <div className="text-center max-w-sm">
          <AlertCircle className="size-12 mx-auto text-red-400/50 mb-4" />
          <h1 className="text-xl font-semibold text-white mb-2">{t("guest.error_title")}</h1>
          <p className="text-white/50 text-sm">{error}</p>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-[#0f1225] flex flex-col">
      {/* Header */}
      <div className="h-14 border-b border-white/10 flex items-center justify-between px-6">
        <div className="flex items-center gap-3">
          {branding.logo_url ? (
            <img src={branding.logo_url} alt={branding.app_name} className="h-7" />
          ) : (
            <OklavierLogo className="size-7" gradient={{ id: "gl1", from: "#7096ff", to: "#65d5c5" }} />
          )}
          <span className="text-white font-semibold">{branding.app_name}</span>
        </div>
        <LangSwitcher variant="dark" />
      </div>

      {/* Content */}
      <div className="flex-1 flex items-center justify-center p-6">
        <div className="bg-[#1a1f36]/50 border border-white/10 rounded-xl p-8 w-full max-w-md">
          {/* Workspace info */}
          <div className="text-center mb-6">
            {info.workspace_image && (
              <img src={info.workspace_image} alt="" className="size-16 mx-auto mb-4 rounded-xl" />
            )}
            {!info.workspace_image && (
              <div className="size-16 mx-auto mb-4 rounded-xl bg-oklavier-blue/20 flex items-center justify-center">
                <Monitor className="size-8 text-oklavier-blue" />
              </div>
            )}
            <h1 className="text-xl font-bold text-white">{info.workspace_name}</h1>
            {info.workspace_description && (
              <p className="text-white/50 text-sm mt-1">{info.workspace_description}</p>
            )}
            {info.label && (
              <p className="text-white/30 text-xs mt-2">{info.label}</p>
            )}
          </div>

          <div className="space-y-4">
            {/* Password field */}
            {info.requires_password && (
              <div>
                <label className="block text-white/60 text-sm mb-1.5 flex items-center gap-1.5">
                  <Lock className="size-3.5" /> {t("guest.password")}
                </label>
                <Input
                  type="password"
                  value={password}
                  onChange={(e) => { setPassword(e.target.value); setPasswordError(false); }}
                  placeholder={t("guest.password_placeholder")}
                  className={`bg-white/5 border-white/10 text-white ${passwordError ? "border-red-500" : ""}`}
                />
                {passwordError && (
                  <p className="text-red-400 text-xs mt-1">{t("guest.wrong_password")}</p>
                )}
              </div>
            )}

            {/* Server credentials (for prompt-mode workspaces) */}
            {info.requires_credentials && (
              <>
                <div>
                  <label className="block text-white/60 text-sm mb-1.5">{t("workspace.username")}</label>
                  <Input value={serverUsername} onChange={(e) => setServerUsername(e.target.value)} className="bg-white/5 border-white/10 text-white" />
                </div>
                <div>
                  <label className="block text-white/60 text-sm mb-1.5">{t("workspace.password")}</label>
                  <Input type="password" value={serverPassword} onChange={(e) => setServerPassword(e.target.value)} className="bg-white/5 border-white/10 text-white" />
                </div>
                <div>
                  <label className="block text-white/60 text-sm mb-1.5">{t("workspace.domain")} <span className="text-white/30">({t("workspace.optional")})</span></label>
                  <Input value={serverDomain} onChange={(e) => setServerDomain(e.target.value)} className="bg-white/5 border-white/10 text-white" />
                </div>
              </>
            )}

            <Button
              onClick={handleLaunch}
              disabled={launching || (info.requires_credentials && !serverUsername)}
              className="w-full bg-oklavier-blue hover:bg-oklavier-blue/80 text-white h-11 gap-2"
            >
              {launching ? (
                <>
                  <Loader2 className="size-4 animate-spin" /> {t("guest.launching")}
                </>
              ) : (
                <>
                  <ExternalLink className="size-4" /> {t("guest.launch")}
                </>
              )}
            </Button>
          </div>

          <p className="text-white/20 text-xs text-center mt-6">
            {t("guest.powered_by")} {branding.app_name}
          </p>
        </div>
      </div>
    </div>
  );
}

export default GuestPage;
