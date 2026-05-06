"use client";

import { useState } from "react";
import { Monitor, Play, Trash2, Copy, Maximize2, Loader2, Search, Globe, X, CheckCircle, Star } from "lucide-react";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Button } from "@/components/ui/button";
import { useSession, signOut } from "@/lib/auth-client";
import { useTranslation } from "@/lib/i18n";
import { useToast } from "@/components/toast";
import { LangSwitcher } from "@/components/lang-switcher";
import { useAPI, usePostAPI, apiAction, invalidate } from "@/lib/api";
import { authFetch } from "@/lib/auth-fetch";
import { useBranding } from "@/lib/use-branding";
import { OklavierLogo } from "@/components/oklavier-logo";

interface WorkspaceImage {
  image_id: string;
  friendly_name: string;
  name: string;
  description: string;
  image_src: string;
  cores: number;
  memory: number;
  categories: string[];
  default_category?: string;
  workspace_type?: string;
  server_auth_mode?: string;
  server_protocol?: string;
  server_allow_remember?: boolean;
}

interface WorkspaceSession {
  session_id: string;
  operational_status: string;
  start_date: string;
  expiration_date: string;
  container_ip: string;
  image: {
    image_id: string;
    friendly_name: string;
    image_src: string;
  };
  keepalive_date: string;
  session_type?: string;
  workspace_type?: string;
  agent_vnc_url?: string;
}

function timeAgo(date: string) {
  const diff = Date.now() - new Date(date).getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 60) return `${String(mins).padStart(2, "0")}m`;
  return `${Math.floor(mins / 60)}h${String(mins % 60).padStart(2, "0")}`;
}

function timeLeft(date: string) {
  const diff = new Date(date).getTime() - Date.now();
  if (diff <= 0) return "00m";
  const mins = Math.floor(diff / 60000);
  if (mins < 60) return `${String(mins).padStart(2, "0")}m`;
  return `${Math.floor(mins / 60)}h${String(mins % 60).padStart(2, "0")}`;
}

type LaunchPhase = "idle" | "modal" | "credentials" | "loading" | "ready" | "resume";

export default function WorkspacesPage() {
  const { data: imagesData } = useAPI<{ images: WorkspaceImage[] }>("/api/workspaces", { refreshInterval: 10000 });
  const { data: sessionsData } = useAPI<{ sessions: WorkspaceSession[] }>("/api/sessions", { refreshInterval: 10000 });
  const { data: favData } = usePostAPI<{ workspace_ids: string[] }>("/api/favorites", { refreshInterval: 30000 });
  const favoriteIds = new Set(favData?.workspace_ids ?? []);
  const rawImages = imagesData?.images ?? [];
  const images = [...rawImages].sort((a, b) => {
    const aFav = favoriteIds.has(a.image_id) ? 0 : 1;
    const bFav = favoriteIds.has(b.image_id) ? 0 : 1;
    if (aFav !== bFav) return aFav - bFav;
    return a.friendly_name.localeCompare(b.friendly_name);
  });
  const sessions = sessionsData?.sessions ?? [];

  const [selectedImage, setSelectedImage] = useState<WorkspaceImage | null>(null);
  const [openIn, setOpenIn] = useState("current");
  const [launchPhase, setLaunchPhase] = useState<LaunchPhase>("idle");
  const [creds, setCreds] = useState({ username: "", password: "", domain: "" });
  const [rememberCreds, setRememberCreds] = useState(false);
  const [progress, setProgress] = useState(0);
  const [launchMessage, setLaunchMessage] = useState("");
  const [destroying, setDestroying] = useState<string | null>(null);
  const [toast, setToast] = useState<string | null>(null);
  const [langOpen, setLangOpen] = useState(false);
  const [lang, setLang] = useState(() => {
    if (typeof document !== "undefined") {
      const match = document.cookie.match(/oklavier_lang=(\w+)/);
      return match ? match[1] : "en";
    }
    return "en";
  });
  const [userMenuOpen, setUserMenuOpen] = useState(false);
  const { data: session } = useSession();
  const username = session?.user?.email || session?.user?.name || "";
  const isAdmin = session?.user?.role === "admin";

  const languages = [
    { code: "fr", label: "Français", flag: "🇫🇷" },
    { code: "en", label: "English", flag: "🇬🇧" },
    { code: "es", label: "Español", flag: "🇪🇸" },
    { code: "de", label: "Deutsch", flag: "🇩🇪" },
  ];

  const { t } = useTranslation();
  const { branding } = useBranding();
  const toastCtx = useToast();

  const imgSrc = (image: { image_src: string }) => {
    const src = image.image_src;
    if (src.startsWith("http")) return src;
    return `/api/proxy-img/${src}`;
  };


  async function handleToggleFavorite(e: React.MouseEvent, imageId: string) {
    e.stopPropagation();
    try {
      await authFetch("/api/favorites/toggle", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ workspace_id: imageId }),
      });
      invalidate("/api/favorites");
    } catch { /* ignore */ }
  }

  function handleAppClick(image: WorkspaceImage) {
    setSelectedImage(image);
    setOpenIn("current");
    // Pre-fill saved credentials if available
    if (image.server_auth_mode === "prompt" && image.server_allow_remember) {
      try {
        const saved = JSON.parse(localStorage.getItem(`oklavier-creds-${image.image_id}`) || "{}");
        if (saved.username) setCreds(saved);
        else setCreds({ username: "", password: "", domain: "" });
      } catch { setCreds({ username: "", password: "", domain: "" }); }
    } else {
      setCreds({ username: "", password: "", domain: "" });
    }
    setLaunchPhase("modal");
  }

  async function handleLaunch() {
    if (!selectedImage) return;
    // If prompt mode and no credentials yet, show credentials modal
    if (selectedImage.server_auth_mode === "prompt" && !creds.username) {
      setLaunchPhase("credentials");
      return;
    }
    setLaunchPhase("loading");
    setProgress(0);
    setLaunchMessage(t("workspace.creating_pod"));

    try {
      const lang = document.cookie.match(/oklavier_lang=(\w+)/)?.[1] || navigator.language?.split("-")[0] || "en";
      const body: Record<string, unknown> = { action: "create", image_id: selectedImage.image_id, lang };
      if (selectedImage.server_auth_mode === "prompt") {
        body.server_username = creds.username;
        body.server_password = creds.password;
        body.server_domain = creds.domain;
        // Save credentials if remember is enabled
        if (rememberCreds && selectedImage.server_allow_remember) {
          localStorage.setItem(`oklavier-creds-${selectedImage.image_id}`, JSON.stringify(creds));
        }
      }
      // Step 1: Create the session
      const res = await authFetch("/api/sessions", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      });
      const data = await res.json();

      if (!data.session_id) throw new Error("No session ID");

      // Step 2: Wait for session to be ready (poll readiness for container sessions)
      if (data.session_type === "container") {
        setProgress(30);
        setLaunchMessage(t("workspace.starting_pod"));
        for (let i = 0; i < 120; i++) {
          await new Promise(r => setTimeout(r, 2000));
          try {
            const readyRes = await authFetch("/api/sessions/readiness", {
              method: "POST",
              headers: { "Content-Type": "application/json" },
              body: JSON.stringify({ session_id: data.session_id }),
            });
            const readyData = await readyRes.json();
            if (readyData.progress) setProgress(readyData.progress);
            if (readyData.message) setLaunchMessage(readyData.message);
            if (readyData.phase === "ready") break;
            if (readyData.phase === "error") throw new Error(readyData.message || "Session failed");
          } catch (e: any) {
            if (e.message === "Session failed") throw e;
          }
        }
      }

      setProgress(100);
      setLaunchMessage(t("workspace.session_created"));
      setToast(t("workspace.session_created"));
      setTimeout(() => setToast(null), 4000);

      invalidate("/api/workspaces");
      invalidate("/api/sessions");

      // Redirect to agent domain with auth token in URL fragment (#token=).
      // SECURITY: previously concatenated as `https://${agentUrl}/...` without
      // any check — a server response of `agentUrl: "evil.com"` or with
      // a leading `//` would land the browser on attacker.com and the
      // fragment would still travel through history/extensions.
      const agentUrl: string | undefined = data.agent_url;
      const sessionPath = "sessions";
      const tokenFragment = data.session_token ? `#token=${encodeURIComponent(data.session_token)}` : "";
      let url: string;
      if (agentUrl) {
        const safeHost = /^[a-z0-9.\-]+(:\d{1,5})?$/i.test(agentUrl) ? agentUrl : null;
        if (!safeHost) {
          setLaunchPhase("idle");
          setSelectedImage(null);
          setToast(t("workspace.session_error"));
          setTimeout(() => setToast(null), 4000);
          return;
        }
        url = `https://${safeHost}/${sessionPath}/${encodeURIComponent(data.session_id)}${tokenFragment}`;
      } else {
        url = `/${sessionPath}/${encodeURIComponent(data.session_id)}${tokenFragment}`;
      }
      setTimeout(() => {
        if (openIn === "new-tab") {
          window.open(url, "_blank");
        } else if (openIn === "new-window") {
          window.open(url, "_blank", "width=1920,height=1080");
        } else {
          window.location.href = url;
        }
        setLaunchPhase("idle");
        setSelectedImage(null);
      }, 500);
    } catch (e) {
      setLaunchPhase("idle");
      setSelectedImage(null);
      setToast(e instanceof Error ? e.message : t("workspace.session_error"));
      setTimeout(() => setToast(null), 4000);
    }
  }

  async function handleDestroy(sessionId: string) {
    setDestroying(sessionId);
    try {
      const { ok, error } = await apiAction("/api/sessions", "POST", { action: "destroy", session_id: sessionId });
      if (!ok) throw new Error(error || t("workspace.session_error"));
      toastCtx.success(t("workspace.session_destroyed"));
      invalidate("/api/workspaces");
      invalidate("/api/sessions");
    } catch (e) {
      toastCtx.error(e instanceof Error ? e.message : t("workspace.session_error"));
    } finally {
      setDestroying(null);
    }
  }

  const [resumeSession, setResumeSession] = useState<WorkspaceSession | null>(null);

  function handleConnect(session: WorkspaceSession) {
    setResumeSession(session);
    setOpenIn("current");
    setLaunchPhase("resume");
  }

  async function handleResumeConfirm() {
    if (!resumeSession) return;
    // Mint a fresh single-purpose bearer right before opening the viewer.
    let bearer = "";
    let agentUrl = resumeSession.agent_vnc_url;
    try {
      const r = await authFetch("/api/sessions", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ action: "connect", session_id: resumeSession.session_id }),
      });
      if (!r.ok) throw new Error(`connect ${r.status}`);
      const j = await r.json();
      bearer = j.session_token || "";
      agentUrl = j.agent_url || agentUrl;
    } catch {
      setToast(t("workspace.session_error"));
      setTimeout(() => setToast(null), 4000);
      setLaunchPhase("idle");
      setResumeSession(null);
      return;
    }
    const sessionPath = "sessions";
    const tokenFragment = bearer ? `#token=${encodeURIComponent(bearer)}` : "";
    const safeHost = agentUrl && /^[a-z0-9.\-]+(:\d{1,5})?$/i.test(agentUrl) ? agentUrl : null;
    const url = safeHost
      ? `https://${safeHost}/${sessionPath}/${encodeURIComponent(resumeSession.session_id)}${tokenFragment}`
      : `/${sessionPath}/${encodeURIComponent(resumeSession.session_id)}${tokenFragment}`;
    if (openIn === "new-tab") {
      window.open(url, "_blank");
    } else if (openIn === "new-window") {
      window.open(url, "_blank", "width=1920,height=1080");
    } else {
      window.location.href = url;
    }
    setLaunchPhase("idle");
    setResumeSession(null);
  }

  return (
    <div
      className="fixed inset-0 bg-cover bg-center bg-no-repeat"
      style={{ backgroundImage: "url('https://images.unsplash.com/photo-1506905925346-21bda4d32df4?w=2560&q=80')" }}
    >
      {/* Toast notification */}
      {toast && (
        <div className="absolute top-4 right-4 z-50 flex items-center gap-2 bg-emerald-600 text-white px-4 py-3 rounded-lg shadow-2xl animate-in slide-in-from-right">
          <CheckCircle className="size-5" />
          <div>
            <p className="font-medium text-sm">{t("workspace.create_session")}</p>
            <p className="text-xs text-white/80">{toast}</p>
          </div>
        </div>
      )}

      {/* Top bar */}
      <div className="absolute top-0 left-0 right-0 flex items-center justify-between px-5 py-3 z-10">
        <div className="flex items-center gap-4">
          {branding.logo_url ? (
            <img src={branding.logo_url} alt={branding.app_name} className="h-8 drop-shadow-lg" />
          ) : (
            <OklavierLogo className="size-8 drop-shadow-lg" gradient={{ id: "wp1", from: "#7096ff", to: "#65d5c5" }} />
          )}
          {isAdmin && (
            <a href="/admin" className="text-white/80 hover:text-white text-sm backdrop-blur-sm bg-white/10 px-3 py-1.5 rounded-full transition-colors">
              {t("workspace.switch_to_admin")}
            </a>
          )}
        </div>
        <div className="flex items-center gap-2">
          <button className="size-9 rounded-full backdrop-blur-sm bg-white/10 flex items-center justify-center text-white/80 hover:text-white hover:bg-white/20 transition-colors">
            <Search className="size-4" />
          </button>
          <LangSwitcher variant="dark" />
          <div className="relative">
            <button
              onClick={() => setUserMenuOpen(!userMenuOpen)}
              className="size-9 rounded-full backdrop-blur-sm bg-oklavier-blue/80 flex items-center justify-center text-white text-xs font-semibold hover:bg-oklavier-blue transition-colors"
            >
              {username ? username.split("@")[0].slice(0, 2).toUpperCase() : "?"}
            </button>
            {userMenuOpen && (
              <>
                <div className="fixed inset-0 z-40" onClick={() => setUserMenuOpen(false)} />
                <div className="absolute right-0 top-12 z-50 w-52 backdrop-blur-xl bg-[#1a1f36]/95 border border-white/10 rounded-xl shadow-2xl overflow-hidden">
                  <div className="px-4 py-3 border-b border-white/10">
                    <p className="text-white text-sm font-medium truncate">{username}</p>
                    {isAdmin && <p className="text-oklavier-blue text-xs">{t("auth.administrator")}</p>}
                  </div>
                  <a
                    href="/profile"
                    className="w-full block text-left px-4 py-2.5 text-sm text-white/70 hover:bg-white/5 transition-colors"
                  >
                    {t("profile.title")}
                  </a>
                  <button
                    onClick={async () => {
                      await signOut();
                      window.location.href = "/login";
                    }}
                    className="w-full text-left px-4 py-2.5 text-sm text-red-400 hover:bg-white/5 transition-colors"
                  >
                    {t("auth.logout")}
                  </button>
                </div>
              </>
            )}
          </div>
        </div>
      </div>

      {/* Active sessions - floating cards on the left */}
      {sessions.length > 0 && launchPhase === "idle" && (
        <div className="absolute top-20 left-5 z-10 space-y-3 max-h-[calc(100vh-6rem)] overflow-y-auto">
          {sessions.map((session) => (
            <div key={session.session_id} className="w-48 backdrop-blur-xl bg-black/40 border border-white/10 rounded-xl overflow-hidden shadow-2xl">
              <div className="flex items-center gap-2 p-3">
                {session.image.image_src && (
                  <img src={imgSrc(session.image)} alt="" className="size-8 rounded" />
                )}
                <div className="flex-1 min-w-0">
                  <p className="text-white text-sm font-medium truncate">{session.image.friendly_name}</p>
                  <div className="flex items-center gap-2 text-xs text-white/60">
                    <span className="text-green-400">{timeAgo(session.start_date)}</span>
                    <span className="text-red-400">{timeLeft(session.expiration_date)}</span>
                  </div>
                </div>
                <button onClick={() => handleDestroy(session.session_id)} className="text-white/40 hover:text-white/80">
                  {destroying === session.session_id ? <Loader2 className="size-3.5 animate-spin" /> : <X className="size-3.5" />}
                </button>
              </div>
              <div className="mx-2 mb-2 rounded-lg bg-white/5 aspect-video flex items-center justify-center relative overflow-hidden">
                <img
                  src={session.agent_vnc_url ? `https://${session.agent_vnc_url}/api/screenshot/${session.session_id}?width=400&height=225${(session as { session_token?: string }).session_token ? `&ticket=${encodeURIComponent((session as { session_token?: string }).session_token!)}` : ``}` : ``}
                  alt=""
                  className="w-full h-full object-cover"
                  onError={(e) => { (e.target as HTMLImageElement).style.display = "none"; }}
                />
                <Monitor className="size-6 text-white/20 absolute" />
                <div className={`absolute top-1.5 right-1.5 size-2.5 rounded-full ${session.operational_status === "running" ? "bg-green-500 shadow-[0_0_6px_rgba(34,197,94,0.5)]" : "bg-yellow-500 shadow-[0_0_6px_rgba(234,179,8,0.5)]"}`} />
              </div>
              <div className="flex border-t border-white/10">
                <button onClick={() => handleConnect(session)} className="flex-1 flex items-center justify-center py-2.5 text-oklavier-blue hover:bg-white/5 transition-colors"><Play className="size-4" /></button>
                <button onClick={() => handleDestroy(session.session_id)} className="flex-1 flex items-center justify-center py-2.5 text-white/50 hover:bg-white/5 transition-colors"><Trash2 className="size-4" /></button>
                <button onClick={() => { window.location.href = `/sessions/${session.session_id}`; }} className="flex-1 flex items-center justify-center py-2.5 text-white/50 hover:bg-white/5 transition-colors"><Maximize2 className="size-4" /></button>
                <button className="flex-1 flex items-center justify-center py-2.5 text-white/50 hover:bg-white/5 transition-colors"><Copy className="size-4" /></button>
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Launch Modal */}
      {launchPhase === "modal" && selectedImage && (
        <div className="fixed inset-0 z-40 flex items-center justify-center">
          <div className="absolute inset-0 bg-black/40 backdrop-blur-sm" onClick={() => { setLaunchPhase("idle"); setSelectedImage(null); }} />
          <div className="relative z-50 w-full max-w-md backdrop-blur-xl bg-[#1a1f36]/90 border border-white/10 rounded-2xl p-6 shadow-2xl">
            <button
              onClick={() => { setLaunchPhase("idle"); setSelectedImage(null); }}
              className="absolute top-4 right-4 text-white/40 hover:text-white/80"
            >
              <X className="size-5" />
            </button>

            <div className="flex items-center gap-4 mb-6">
              {selectedImage.image_src && (
                <img src={imgSrc(selectedImage)} alt="" className="size-14 rounded-lg" />
              )}
              <div>
                <h2 className="text-white text-xl font-semibold">{t("workspace.launch", { name: selectedImage.friendly_name })}</h2>
                <p className="text-white/50 text-sm">{selectedImage.description}</p>
              </div>
            </div>

            <div className="mb-6">
              <label className="text-white/60 text-sm mb-2 block">{t("workspace.open_in")}</label>
              <Select value={openIn} onValueChange={(v) => v && setOpenIn(v)}>
                <SelectTrigger className="w-full bg-[#2e3862]/80 border-white/10 text-white">
                  <SelectValue placeholder="Onglet actuel">
                    {openIn === "current" ? t("workspace.current_tab") : openIn === "new-tab" ? t("workspace.new_tab") : t("workspace.new_window")}
                  </SelectValue>
                </SelectTrigger>
                <SelectContent className="bg-[#1a1f36] border-white/10 min-w-[var(--radix-select-trigger-width)]">
                  <SelectItem value="current" className="text-white focus:bg-white/10 focus:text-white">{t("workspace.current_tab")}</SelectItem>
                  <SelectItem value="new-tab" className="text-white focus:bg-white/10 focus:text-white">{t("workspace.new_tab")}</SelectItem>
                  <SelectItem value="new-window" className="text-white focus:bg-white/10 focus:text-white">{t("workspace.new_window")}</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <Button
              onClick={handleLaunch}
              className="w-full bg-oklavier-blue hover:bg-oklavier-purple text-white py-6 text-base font-medium"
            >
              {t("workspace.launch_session")}
            </Button>
          </div>
        </div>
      )}

      {/* Credentials Modal (TSE/RDS) */}
      {launchPhase === "credentials" && selectedImage && (
        <div className="fixed inset-0 z-40 flex items-center justify-center">
          <div className="absolute inset-0 bg-black/40 backdrop-blur-sm" onClick={() => { setLaunchPhase("idle"); setCreds({ username: "", password: "", domain: "" }); }} />
          <div className="relative z-50 w-full max-w-md backdrop-blur-xl bg-[#1a1f36]/90 border border-white/10 rounded-2xl p-6 shadow-2xl">
            <button onClick={() => { setLaunchPhase("idle"); setCreds({ username: "", password: "", domain: "" }); }} className="absolute top-4 right-4 text-white/30 hover:text-white/60 text-lg">&times;</button>
            <div className="flex items-center gap-3 mb-5">
              {selectedImage.image_src && (
                <img src={imgSrc(selectedImage)} className="size-10 rounded-lg" alt="" />
              )}
              <div>
                <h3 className="text-white font-semibold">{selectedImage.friendly_name}</h3>
                <p className="text-white/30 text-xs">{t("workspace.enter_credentials")}</p>
              </div>
            </div>
            <div className="space-y-3 mb-5">
              <input
                type="text" placeholder={t("workspace.username")} autoFocus
                className="w-full bg-[#2e3862]/80 border border-white/10 text-white rounded-lg px-4 py-3 text-sm focus:outline-none focus:ring-2 focus:ring-oklavier-blue/50"
                value={creds.username} onChange={(e) => setCreds({ ...creds, username: e.target.value })}
                onKeyDown={(e) => { if (e.key === "Enter") handleLaunch(); }}
              />
              <input
                type="password" placeholder={t("workspace.password")}
                className="w-full bg-[#2e3862]/80 border border-white/10 text-white rounded-lg px-4 py-3 text-sm focus:outline-none focus:ring-2 focus:ring-oklavier-blue/50"
                value={creds.password} onChange={(e) => setCreds({ ...creds, password: e.target.value })}
                onKeyDown={(e) => { if (e.key === "Enter") handleLaunch(); }}
              />
              {selectedImage.server_protocol === "rdp" && (
                <input
                  type="text" placeholder={t("workspace.domain") + " (" + t("workspace.optional") + ")"}
                  className="w-full bg-[#2e3862]/80 border border-white/10 text-white rounded-lg px-4 py-3 text-sm focus:outline-none focus:ring-2 focus:ring-oklavier-blue/50"
                  value={creds.domain} onChange={(e) => setCreds({ ...creds, domain: e.target.value })}
                  onKeyDown={(e) => { if (e.key === "Enter") handleLaunch(); }}
                />
              )}
            </div>
            {selectedImage.server_allow_remember && (
              <label className="flex items-center gap-2 mb-4 cursor-pointer">
                <input type="checkbox" checked={rememberCreds} onChange={(e) => setRememberCreds(e.target.checked)} className="accent-[#7096ff] w-4 h-4" />
                <span className="text-white/50 text-sm">{t("workspace.remember_credentials")}</span>
              </label>
            )}
            <button
              onClick={handleLaunch}
              disabled={!creds.username || !creds.password}
              className="w-full py-3 rounded-xl bg-gradient-to-r from-oklavier-blue to-oklavier-teal text-white font-semibold text-sm disabled:opacity-30 disabled:cursor-not-allowed hover:opacity-90 transition-opacity"
            >
              {t("workspace.connect")}
            </button>
          </div>
        </div>
      )}

      {/* Resume Modal */}
      {launchPhase === "resume" && resumeSession && (
        <div className="fixed inset-0 z-40 flex items-center justify-center">
          <div className="absolute inset-0 bg-black/40 backdrop-blur-sm" onClick={() => { setLaunchPhase("idle"); setResumeSession(null); }} />
          <div className="relative z-50 w-full max-w-md backdrop-blur-xl bg-[#1a1f36]/90 border border-white/10 rounded-2xl p-6 shadow-2xl">
            <button
              onClick={() => { setLaunchPhase("idle"); setResumeSession(null); }}
              className="absolute top-4 right-4 text-white/40 hover:text-white/80"
            >
              <X className="size-5" />
            </button>

            <div className="flex items-center gap-4 mb-6">
              {resumeSession.image.image_src && (
                <img src={imgSrc(resumeSession.image)} alt="" className="size-14 rounded-lg" />
              )}
              <div>
                <h2 className="text-white text-xl font-semibold">{t("workspace.resume", { name: resumeSession.image.friendly_name })}</h2>
                <p className="text-white/50 text-sm">Session en cours depuis {timeAgo(resumeSession.start_date)}</p>
              </div>
            </div>

            <div className="mb-6">
              <label className="text-white/60 text-sm mb-2 block">{t("workspace.open_in")}</label>
              <Select value={openIn} onValueChange={(v) => v && setOpenIn(v)}>
                <SelectTrigger className="w-full bg-[#2e3862]/80 border-white/10 text-white">
                  <SelectValue placeholder="Onglet actuel">
                    {openIn === "current" ? t("workspace.current_tab") : openIn === "new-tab" ? t("workspace.new_tab") : t("workspace.new_window")}
                  </SelectValue>
                </SelectTrigger>
                <SelectContent className="bg-[#1a1f36] border-white/10 min-w-[var(--radix-select-trigger-width)]">
                  <SelectItem value="current" className="text-white focus:bg-white/10 focus:text-white">{t("workspace.current_tab")}</SelectItem>
                  <SelectItem value="new-tab" className="text-white focus:bg-white/10 focus:text-white">{t("workspace.new_tab")}</SelectItem>
                  <SelectItem value="new-window" className="text-white focus:bg-white/10 focus:text-white">{t("workspace.new_window")}</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <Button
              onClick={handleResumeConfirm}
              className="w-full bg-emerald-600 hover:bg-emerald-700 text-white py-6 text-base font-medium"
            >
              {t("workspace.resume_session")}
            </Button>
          </div>
        </div>
      )}

      {/* Loading Overlay */}
      {launchPhase === "loading" && selectedImage && (
        <div className="fixed inset-0 z-40 flex flex-col items-center justify-center">
          <div className="relative">
            {/* Outer spinning ring */}
            <div className="size-52 rounded-full border-4 border-oklavier-blue/30 absolute inset-0 animate-spin" style={{ animationDuration: "3s" }}>
              <div className="absolute top-0 left-1/2 -translate-x-1/2 -translate-y-1/2 size-3 rounded-full bg-oklavier-blue" />
            </div>
            {/* Progress ring */}
            <svg className="size-52 -rotate-90" viewBox="0 0 208 208">
              <circle cx="104" cy="104" r="100" fill="none" stroke="rgba(112,150,255,0.15)" strokeWidth="4" />
              <circle
                cx="104" cy="104" r="100" fill="none" stroke="#7096ff" strokeWidth="4"
                strokeDasharray={`${2 * Math.PI * 100}`}
                strokeDashoffset={`${2 * Math.PI * 100 * (1 - progress / 100)}`}
                strokeLinecap="round"
                className="transition-all duration-300"
              />
            </svg>
            {/* Center content */}
            <div className="absolute inset-0 flex flex-col items-center justify-center">
              <div className="backdrop-blur-xl bg-black/40 size-40 rounded-full flex flex-col items-center justify-center gap-2">
                <p className="text-white/60 text-xs">{t("common.loading")}</p>
                <p className="text-white font-semibold">{selectedImage.friendly_name}</p>
                {selectedImage.image_src && (
                  <img src={imgSrc(selectedImage)} alt="" className="size-12" />
                )}
                <p className="text-white/50 text-xs text-center px-4">{launchMessage}</p>
              </div>
            </div>
          </div>
          <div className="mt-8 text-center">
            <p className="text-white text-2xl font-bold">
              <span>{Math.round(progress)}%</span>
              <span className="text-white/60 font-normal ml-2">{t("workspace.complete")}</span>
            </p>
          </div>
        </div>
      )}

      {/* Ready overlay (brief) */}
      {launchPhase === "ready" && selectedImage && (
        <div className="fixed inset-0 z-40 flex flex-col items-center justify-center">
          <div className="relative">
            <svg className="size-52 -rotate-90" viewBox="0 0 208 208">
              <circle cx="104" cy="104" r="100" fill="none" stroke="#7096ff" strokeWidth="4" />
            </svg>
            <div className="absolute inset-0 flex flex-col items-center justify-center">
              <div className="backdrop-blur-xl bg-black/40 size-40 rounded-full flex flex-col items-center justify-center gap-2">
                <p className="text-white/60 text-xs">{t("common.loading")}</p>
                <p className="text-white font-semibold">{selectedImage.friendly_name}</p>
                {selectedImage.image_src && (
                  <img src={imgSrc(selectedImage)} alt="" className="size-12" />
                )}
                <p className="text-white/50 text-xs text-center px-4">{launchMessage}</p>
              </div>
            </div>
          </div>
          <div className="mt-8 text-center">
            <p className="text-white text-2xl font-bold">
              100% <span className="text-white/60 font-normal">{t("workspace.complete")}</span>
            </p>
          </div>
        </div>
      )}

      {/* App dock - centered (hidden during loading) */}
      {(launchPhase === "idle" || launchPhase === "modal" || launchPhase === "credentials" || launchPhase === "resume") && (
        <div className="absolute left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2 z-10">
          {images.length === 0 ? (
            <div className="backdrop-blur-xl bg-black/30 border border-white/10 rounded-2xl px-6 py-4 shadow-2xl flex items-center gap-3 text-white/60">
              {!imagesData ? (
                <>
                  <Loader2 className="size-5 animate-spin" />
                  <span>{t("workspace.loading_spaces")}</span>
                </>
              ) : (
                <>
                  <Monitor className="size-5" />
                  <span>{t("workspace.no_spaces")}</span>
                </>
              )}
            </div>
          ) : (
            <div className="flex flex-wrap justify-center gap-4 max-w-4xl">
              {images.map((image) => (
                <button
                  key={image.image_id}
                  onClick={() => handleAppClick(image)}
                  className="relative flex items-center gap-3 px-5 py-3.5 rounded-2xl backdrop-blur-xl bg-black/30 border border-white/10 shadow-2xl hover:bg-white/15 hover:scale-105 transition-all duration-200 group min-w-[180px]"
                >
                  <span
                    onClick={(e) => handleToggleFavorite(e, image.image_id)}
                    className="absolute top-2 right-2 p-0.5 rounded-full hover:bg-white/10 transition-colors"
                  >
                    <Star className={`size-3.5 transition-colors ${favoriteIds.has(image.image_id) ? "fill-yellow-400 text-yellow-400" : "text-white/30 hover:text-white/60"}`} />
                  </span>
                  {image.image_src && (
                    <img src={imgSrc(image)} alt={image.friendly_name} className="size-10 rounded group-hover:scale-110 transition-transform" />
                  )}
                  <div className="text-left">
                    <p className="text-white text-sm font-medium">{image.friendly_name}</p>
                    <p className="text-white/50 text-xs">{image.default_category || image.categories?.[0] || ""}</p>
                  </div>
                </button>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
