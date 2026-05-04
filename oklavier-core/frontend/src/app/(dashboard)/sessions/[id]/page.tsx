"use client";

import { useParams } from "next/navigation";
import { useState, useEffect } from "react";
import { Loader2, Clipboard, Printer, Download, Upload, Monitor, Mic, Maximize, Settings, Share2, ArrowLeft, Trash2, X, Volume2 } from "lucide-react";
import Link from "next/link";
import { useTranslation } from "@/lib/i18n";
import { ProxyViewer } from "@/components/proxy-viewer";
import { authFetch } from "@/lib/auth-fetch";

export default function SessionPage() {
  const params = useParams();
  const sessionId = params.id as string;
  const { t } = useTranslation();
  const [session, setSession] = useState<{ container_ip: string; operational_status: string; agent_vnc_url?: string; image?: { friendly_name: string; image_src: string } } | null>(null);
  const [loading, setLoading] = useState(true);
  const [panelOpen, setPanelOpen] = useState(false);
  const [qualityPanel, setQualityPanel] = useState(false);
  const [videoQuality, setVideoQuality] = useState(2);

  const qualityLevels = [
    { value: 0, label: "Auto", desc: "Adaptatif" },
    { value: 1, label: "Low", desc: "Faible bande passante" },
    { value: 2, label: "Medium", desc: "Équilibré" },
    { value: 3, label: "High", desc: "Haute qualité" },
    { value: 5, label: "Very High", desc: "Très haute qualité" },
    { value: 7, label: "Lossless", desc: "Sans perte" },
  ];
  const [proxyReady, setProxyReady] = useState(false);
  const [proxyConnecting, setProxyConnecting] = useState(false);

  // Start proxy engine when session is found (retry until connected)
  useEffect(() => {
    if (proxyReady) return;
    let cancelled = false;
    const tryConnect = async () => {
      for (let i = 0; i < 60 && !cancelled; i++) {
        try {
          const res = await authFetch(`/api/proxy/connect/${sessionId}`, { method: "POST" });
          const data = await res.json();
          if (data.ok && !cancelled) {
            setProxyReady(true);
            return;
          }
        } catch {}
        await new Promise(r => setTimeout(r, 2000)); // Retry every 2s
      }
    };
    tryConnect();
    return () => { cancelled = true; };
  }, [session, sessionId]);

  const closePanel = () => {
    setPanelOpen(false);
  };

  useEffect(() => {
    async function fetchSession() {
      try {
        const res = await authFetch("/api/sessions");
        const data = await res.json();
        const found = data.sessions?.find((s: { session_id: string }) =>
          s.session_id === sessionId || s.session_id.replace(/-/g, "") === sessionId
        );
        if (found) setSession(found);
      } catch {
        // Silently fail — session stays null, "not found" UI will show
      } finally {
        setLoading(false);
      }
    }
    fetchSession();
  }, [sessionId]);

  if (loading) {
    return (
      <div className="fixed inset-0 bg-[#1a1f36] flex items-center justify-center">
        <Loader2 className="size-8 animate-spin text-oklavier-blue" />
      </div>
    );
  }

  if (!session) {
    return (
      <div className="fixed inset-0 bg-[#1a1f36] flex flex-col items-center justify-center text-white gap-4">
        <p className="text-xl">{t("session.not_found")}</p>
        <Link href="/workspaces" className="text-oklavier-blue hover:underline">{t("session.back_to_spaces")}</Link>
      </div>
    );
  }

  // ProxyViewer handles the VNC/WebRTC connection

  const toggleFullscreen = () => {
    if (!document.fullscreenElement) {
      document.documentElement.requestFullscreen();
    } else {
      document.exitFullscreen();
    }
  };

  return (
    <div className="fixed inset-0 bg-black">
      {/* Side tab - hidden when panel is open */}
      {!panelOpen && (
        <button
          onClick={() => setPanelOpen(true)}
          className="fixed left-0 top-1/2 -translate-y-1/2 z-50 bg-black/50 hover:bg-black/70 border border-white/10 border-l-0 rounded-r-lg px-1 py-8 transition-all opacity-30 hover:opacity-100"
        >
          <svg viewBox="0 0 24 24" className="size-4 text-white/80" fill="none" stroke="currentColor" strokeWidth="2">
            <circle cx="12" cy="5" r="1"/><circle cx="12" cy="12" r="1"/><circle cx="12" cy="19" r="1"/>
          </svg>
        </button>
      )}

      {/* Overlay to close panel */}
      {panelOpen && (
        <div className="fixed inset-0 z-30" onClick={closePanel} />
      )}

      {/* Side panel */}
      <div
        className={`fixed left-0 top-0 bottom-0 z-40 w-64 bg-[#1a1f36]/95 backdrop-blur-xl border-r border-white/10 transform transition-transform duration-200 flex flex-col ${
          panelOpen ? "translate-x-0" : "-translate-x-full"
        }`}
      >
        <div className="flex items-center justify-between p-4 border-b border-white/10">
          <h2 className="text-white font-semibold text-sm">{t("session.control_panel")}</h2>
          <button onClick={closePanel} className="text-white/40 hover:text-white">
            <X className="size-4" />
          </button>
        </div>

        {/* Quick actions */}
        <div className="flex items-center justify-center gap-6 py-4 border-b border-white/10">
          <button className="flex flex-col items-center gap-1 text-white/60 hover:text-white">
            <div className="size-9 rounded-full bg-oklavier-blue/20 flex items-center justify-center">
              <Volume2 className="size-4 text-oklavier-blue" />
            </div>
            <span className="text-[10px]">{t("session.sound")}</span>
          </button>
          <button className="flex flex-col items-center gap-1 text-white/60 hover:text-white">
            <div className="size-9 rounded-full bg-white/10 flex items-center justify-center">
              <Mic className="size-4" />
            </div>
            <span className="text-[10px]">{t("session.microphone")}</span>
          </button>
          <button onClick={toggleFullscreen} className="flex flex-col items-center gap-1 text-white/60 hover:text-white">
            <div className="size-9 rounded-full bg-white/10 flex items-center justify-center">
              <Maximize className="size-4" />
            </div>
            <span className="text-[10px]">{t("session.fullscreen")}</span>
          </button>
        </div>

        {/* Menu items with real actions */}
        <div className="flex-1 overflow-y-auto">
          {[
            { icon: Clipboard, label: t("session.clipboard"), desc: t("session.clipboard_desc"), action: () => {
              prompt(t("session.clipboard_prompt"));
            }},
            { icon: Monitor, label: t("session.displays"), desc: t("session.displays_desc"), action: () => {} },
            { icon: Settings, label: t("session.streaming_quality"), desc: qualityLevels.find(q => q.value === videoQuality)?.label || "Medium", action: () => setQualityPanel(!qualityPanel) },
            { icon: Download, label: t("session.download"), desc: t("session.download_desc") },
            { icon: Upload, label: t("session.upload"), desc: t("session.upload_desc") },
            { icon: Settings, label: t("session.advanced"), desc: t("session.advanced") },
          ].map((item, i) => (
            <button
              key={i}
              onClick={item.action}
              className="w-full flex items-center gap-3 px-4 py-2.5 text-white/70 hover:bg-white/5 hover:text-white transition-colors"
            >
              <item.icon className="size-4 text-oklavier-blue shrink-0" />
              <div className="text-left min-w-0">
                <p className="text-sm truncate">{item.label}</p>
                <p className="text-[10px] text-white/40 truncate">{item.desc}</p>
              </div>
            </button>
          ))}

          {/* Quality slider inline */}
          {qualityPanel && (
            <div className="px-4 pb-3">
              <input
                type="range"
                min={0}
                max={5}
                value={qualityLevels.findIndex(q => q.value === videoQuality)}
                onChange={(e) => {
                  const level = qualityLevels[parseInt(e.target.value)];
                  setVideoQuality(level.value);
                }}
                className="w-full h-1 bg-white/10 rounded-full appearance-none cursor-pointer accent-oklavier-blue"
              />
            </div>
          )}
        </div>

        {/* Bottom */}
        <div className="border-t border-white/10">
          <Link href="/workspaces" className="w-full flex items-center gap-3 px-4 py-2.5 text-white/70 hover:bg-white/5 hover:text-white transition-colors">
            <ArrowLeft className="size-4 text-oklavier-blue shrink-0" />
            <div className="text-left">
              <p className="text-sm">{t("session.workspaces")}</p>
              <p className="text-[10px] text-white/40">{t("session.workspaces_desc")}</p>
            </div>
          </Link>
          <button
            onClick={async () => {
              try {
                const res = await authFetch("/api/sessions", {
                  method: "POST",
                  headers: { "Content-Type": "application/json" },
                  body: JSON.stringify({ action: "destroy", session_id: sessionId }),
                });
                if (!res.ok) throw new Error("Destroy failed");
                window.location.href = "/workspaces";
              } catch {
                // Stay on page if destroy fails
              }
            }}
            className="w-full flex items-center gap-3 px-4 py-2.5 text-red-400/70 hover:bg-red-500/10 hover:text-red-400 transition-colors"
          >
            <Trash2 className="size-4 shrink-0" />
            <div className="text-left">
              <p className="text-sm">{t("session.destroy")}</p>
              <p className="text-[10px] text-white/40">{t("session.destroy_desc")}</p>
            </div>
          </button>
        </div>
      </div>

      {/* Proxy Viewer — WebRTC native H.264 */}
      {proxyReady ? (
        <ProxyViewer
          sessionId={sessionId}
          className="w-full h-full"
          onConnected={() => {}}
          onDisconnected={() => {}}
        />
      ) : (
        <div className="fixed inset-0 z-20 bg-[#1a1f36] flex flex-col items-center justify-center gap-4">
          <div className="relative">
            <svg className="size-32 -rotate-90 animate-spin" style={{ animationDuration: "3s" }} viewBox="0 0 120 120">
              <circle cx="60" cy="60" r="54" fill="none" stroke="rgba(112,150,255,0.15)" strokeWidth="3" />
              <circle cx="60" cy="60" r="54" fill="none" stroke="#7096ff" strokeWidth="3"
                strokeDasharray="339" strokeDashoffset="250" strokeLinecap="round" />
            </svg>
          </div>
          <p className="text-white/60 text-sm">{t("session.connecting")}</p>
        </div>
      )}
    </div>
  );
}
