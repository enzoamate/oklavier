"use client";

import { useEffect, useRef, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import { ArrowLeft, Play, Pause, SkipBack, SkipForward, Loader2 } from "lucide-react";
import { authFetch } from "@/lib/auth-fetch";
import { useTranslation } from "@/lib/i18n";
import Script from "next/script";

declare global {
  interface Window {
    Guacamole: any;
    module: any;
  }
}

function formatTime(ms: number): string {
  const totalSec = Math.floor(ms / 1000);
  const h = Math.floor(totalSec / 3600);
  const m = Math.floor((totalSec % 3600) / 60);
  const s = totalSec % 60;
  const pad = (n: number) => n.toString().padStart(2, "0");
  return h > 0 ? `${h}:${pad(m)}:${pad(s)}` : `${pad(m)}:${pad(s)}`;
}

export default function RecordingReplayPage() {
  const { id } = useParams<{ id: string }>();
  const router = useRouter();
  const { t } = useTranslation();
  const displayRef = useRef<HTMLDivElement>(null);
  const recordingRef = useRef<any>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [playing, setPlaying] = useState(false);
  const [position, setPosition] = useState(0);
  const [duration, setDuration] = useState(0);
  const [speed, setSpeed] = useState(1);
  const [guacLoaded, setGuacLoaded] = useState(false);
  const [blobUrl, setBlobUrl] = useState<string | null>(null);

  // Fetch recording file
  useEffect(() => {
    if (!id) return;
    (async () => {
      try {
        const res = await authFetch(`/api/admin/recordings/${id}/download`);
        if (!res.ok) {
          setError("Failed to load recording");
          setLoading(false);
          return;
        }
        const contentType = res.headers.get("content-type") || "";
        if (contentType.includes("octet-stream")) {
          const blob = await res.blob();
          const url = URL.createObjectURL(blob);
          setBlobUrl(url);
        } else {
          // S3 — get JSON with URL
          const info = await res.json();
          if (info.endpoint && info.bucket && info.s3_key) {
            setBlobUrl(`${info.endpoint}/${info.bucket}/${info.s3_key}`);
          } else {
            setError("Recording not available");
          }
        }
      } catch {
        setError("Failed to load recording");
      }
      setLoading(false);
    })();
    return () => {
      if (blobUrl?.startsWith("blob:")) URL.revokeObjectURL(blobUrl);
    };
  }, [id]);

  // Init player when both guacamole lib and blob URL are ready
  useEffect(() => {
    if (!guacLoaded || !blobUrl || !displayRef.current) return;
    const Guacamole = window.Guacamole;
    if (!Guacamole?.SessionRecording) {
      setError("Guacamole.SessionRecording not available");
      return;
    }

    try {
      const tunnel = new Guacamole.StaticHTTPTunnel(blobUrl);
      const recording = new Guacamole.SessionRecording(tunnel);
      recordingRef.current = recording;

      const display = recording.getDisplay();
      const displayEl = displayRef.current;
      displayEl.innerHTML = "";
      displayEl.appendChild(display.getElement());

      recording.connect();

      recording.onplay = () => setPlaying(true);
      recording.onpause = () => setPlaying(false);
      recording.onseek = (pos: number) => setPosition(pos);
      recording.onprogress = (dur: number) => setDuration(dur);

      // Auto-scale display
      const resizeObserver = new ResizeObserver(() => {
        const dw = display.getWidth();
        const dh = display.getHeight();
        if (!dw || !dh) return;
        const cw = displayEl.clientWidth;
        const ch = displayEl.clientHeight;
        display.scale(Math.min(cw / dw, ch / dh));
      });
      resizeObserver.observe(displayEl);
      display.onresize = () => {
        const dw = display.getWidth();
        const dh = display.getHeight();
        if (!dw || !dh) return;
        const cw = displayEl.clientWidth;
        const ch = displayEl.clientHeight;
        display.scale(Math.min(cw / dw, ch / dh));
      };

      setLoading(false);

      return () => {
        resizeObserver.disconnect();
        recording.disconnect();
      };
    } catch (e: any) {
      setError("Failed to initialize player: " + e.message);
    }
  }, [guacLoaded, blobUrl]);

  function togglePlay() {
    const r = recordingRef.current;
    if (!r) return;
    if (playing) r.pause();
    else r.play();
  }

  function seek(e: React.ChangeEvent<HTMLInputElement>) {
    const r = recordingRef.current;
    if (!r) return;
    r.seek(parseInt(e.target.value));
  }

  function changeSpeed(newSpeed: number) {
    setSpeed(newSpeed);
    // Guacamole.SessionRecording doesn't have a speed API natively,
    // but we can pause and play to reset the playback
  }

  function skipBack() {
    const r = recordingRef.current;
    if (!r) return;
    r.seek(Math.max(0, position - 10000));
  }

  function skipForward() {
    const r = recordingRef.current;
    if (!r) return;
    r.seek(Math.min(duration, position + 10000));
  }

  return (
    <div className="flex flex-col h-[calc(100vh-3.5rem)]">
      {/* Module shim for guacamole CJS */}
      <Script
        id="guac-module-shim"
        strategy="beforeInteractive"
        dangerouslySetInnerHTML={{ __html: "var module = { exports: {} };" }}
      />
      <Script
        src="https://cdn.jsdelivr.net/npm/guacamole-common-js@1.5.0/dist/cjs/guacamole-common.js"
        integrity="sha384-puRyg8V7m9K0cq9y+ndybD9ZZ08eYGgFwTTnNe8NxMJBUy02vcil7/XWwKa0Rbcs"
        crossOrigin="anonymous"
        strategy="afterInteractive"
        onLoad={() => {
          window.Guacamole = window.module?.exports || (window as any).Guacamole;
          setGuacLoaded(true);
        }}
      />

      {/* Header */}
      <div className="flex items-center gap-4 p-4 border-b border-white/10 bg-[#1a1f36]/50">
        <button
          onClick={() => router.push("/admin/recordings")}
          className="flex items-center gap-2 text-white/60 hover:text-white transition-colors text-sm"
        >
          <ArrowLeft className="size-4" />
          {t("admin.recordings.title")}
        </button>
        <span className="text-white/30">|</span>
        <span className="text-white/50 text-sm font-mono">{id?.toString().slice(0, 8)}</span>
      </div>

      {/* Display area */}
      <div className="flex-1 bg-black overflow-hidden flex items-center justify-center relative">
        {(loading || !guacLoaded) && (
          <div className="absolute inset-0 flex items-center justify-center bg-[#0f1225] z-10">
            <div className="flex flex-col items-center gap-3">
              <Loader2 className="size-8 animate-spin text-oklavier-blue" />
              <p className="text-white/40 text-sm">{t("common.loading")}</p>
            </div>
          </div>
        )}
        {error && (
          <div className="absolute inset-0 flex items-center justify-center bg-[#0f1225] z-10">
            <p className="text-red-400 text-sm">{error}</p>
          </div>
        )}
        <div ref={displayRef} className="w-full h-full" />
      </div>

      {/* Playback controls */}
      <div className="bg-[#1a1f36] border-t border-white/10 px-6 py-3 flex items-center gap-4">
        <button onClick={skipBack} className="text-white/50 hover:text-white transition-colors" title="-10s">
          <SkipBack className="size-4" />
        </button>
        <button
          onClick={togglePlay}
          className="w-10 h-10 rounded-full bg-oklavier-blue/20 hover:bg-oklavier-blue/30 flex items-center justify-center text-white transition-colors"
        >
          {playing ? <Pause className="size-5" /> : <Play className="size-5 ml-0.5" />}
        </button>
        <button onClick={skipForward} className="text-white/50 hover:text-white transition-colors" title="+10s">
          <SkipForward className="size-4" />
        </button>

        <span className="text-white/50 text-xs font-mono min-w-[50px]">{formatTime(position)}</span>

        <input
          type="range"
          min={0}
          max={duration || 100}
          value={position}
          onChange={seek}
          className="flex-1 accent-oklavier-blue h-1"
        />

        <span className="text-white/50 text-xs font-mono min-w-[50px] text-right">{formatTime(duration)}</span>

        <select
          value={speed}
          onChange={(e) => changeSpeed(parseFloat(e.target.value))}
          className="bg-white/5 border border-white/10 rounded px-2 py-1 text-white/60 text-xs"
        >
          <option value={0.5}>0.5x</option>
          <option value={1}>1x</option>
          <option value={2}>2x</option>
          <option value={4}>4x</option>
        </select>
      </div>
    </div>
  );
}
