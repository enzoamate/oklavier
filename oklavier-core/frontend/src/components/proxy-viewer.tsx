"use client";

import { useRef, useCallback, useEffect, useState } from "react";
import { useProxyStream } from "@/lib/use-proxy-stream";
import { Maximize2, Minimize2, Signal, SignalZero } from "lucide-react";

interface ProxyViewerProps {
  sessionId: string;
  className?: string;
  onConnected?: () => void;
  onDisconnected?: () => void;
}

// Map browser key events to X11 keysyms
function keyToKeysym(e: KeyboardEvent): number {
  // Common mappings
  const map: Record<string, number> = {
    Backspace: 0xff08, Tab: 0xff09, Enter: 0xff0d, Escape: 0xff1b,
    Delete: 0xffff, Home: 0xff50, End: 0xff57, PageUp: 0xff55, PageDown: 0xff56,
    ArrowLeft: 0xff51, ArrowUp: 0xff52, ArrowRight: 0xff53, ArrowDown: 0xff54,
    ShiftLeft: 0xffe1, ShiftRight: 0xffe2, ControlLeft: 0xffe3, ControlRight: 0xffe4,
    AltLeft: 0xffe9, AltRight: 0xffea, MetaLeft: 0xffeb, MetaRight: 0xffec,
    F1: 0xffbe, F2: 0xffbf, F3: 0xffc0, F4: 0xffc1, F5: 0xffc2, F6: 0xffc3,
    F7: 0xffc4, F8: 0xffc5, F9: 0xffc6, F10: 0xffc7, F11: 0xffc8, F12: 0xffc9,
    Space: 0x20, Insert: 0xff63, CapsLock: 0xffe5, NumLock: 0xff7f,
  };
  if (map[e.code]) return map[e.code];
  // For printable characters, use the char code
  if (e.key.length === 1) return e.key.charCodeAt(0);
  return 0;
}

export function ProxyViewer({ sessionId, className = "", onConnected, onDisconnected }: ProxyViewerProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [fullscreen, setFullscreen] = useState(false);
  const [showStats, setShowStats] = useState(false);

  const { videoRef, connected, stats, sendInput, sendClipboard, sendResize } = useProxyStream({
    sessionId,
    onConnected,
    onDisconnected,
  });

  // Keyboard events
  const handleKeyDown = useCallback((e: KeyboardEvent) => {
    e.preventDefault();
    const keysym = keyToKeysym(e);
    if (keysym) sendInput({ type: "key", key: keysym, down: true });
  }, [sendInput]);

  const handleKeyUp = useCallback((e: KeyboardEvent) => {
    e.preventDefault();
    const keysym = keyToKeysym(e);
    if (keysym) sendInput({ type: "key", key: keysym, down: false });
  }, [sendInput]);

  // Mouse events on video element
  const getMousePos = useCallback((e: React.MouseEvent) => {
    const video = videoRef.current;
    if (!video) return { x: 0, y: 0 };
    const rect = video.getBoundingClientRect();
    const scaleX = video.videoWidth / rect.width;
    const scaleY = video.videoHeight / rect.height;
    return {
      x: Math.round((e.clientX - rect.left) * scaleX),
      y: Math.round((e.clientY - rect.top) * scaleY),
    };
  }, []);

  const handleMouseMove = useCallback((e: React.MouseEvent) => {
    const pos = getMousePos(e);
    sendInput({ type: "mouse", x: pos.x, y: pos.y, buttons: e.buttons });
  }, [getMousePos, sendInput]);

  const handleMouseDown = useCallback((e: React.MouseEvent) => {
    e.preventDefault();
    const pos = getMousePos(e);
    // VNC button mask: 1=left, 2=middle, 4=right
    let buttons = 0;
    if (e.button === 0) buttons = 1;
    if (e.button === 1) buttons = 2;
    if (e.button === 2) buttons = 4;
    sendInput({ type: "mouse", x: pos.x, y: pos.y, buttons });
  }, [getMousePos, sendInput]);

  const handleMouseUp = useCallback((e: React.MouseEvent) => {
    const pos = getMousePos(e);
    sendInput({ type: "mouse", x: pos.x, y: pos.y, buttons: 0 });
  }, [getMousePos, sendInput]);

  const handleWheel = useCallback((e: React.WheelEvent) => {
    e.preventDefault();
    const pos = getMousePos(e);
    // VNC scroll: button 4 = scroll up, button 5 = scroll down
    const button = e.deltaY < 0 ? 8 : 16;
    sendInput({ type: "mouse", x: pos.x, y: pos.y, buttons: button });
    // Release immediately
    setTimeout(() => sendInput({ type: "mouse", x: pos.x, y: pos.y, buttons: 0 }), 50);
  }, [getMousePos, sendInput]);

  // Prevent context menu
  const handleContextMenu = useCallback((e: React.MouseEvent) => {
    e.preventDefault();
  }, []);

  // Keyboard listener on container
  useEffect(() => {
    const el = containerRef.current;
    if (!el) return;
    el.addEventListener("keydown", handleKeyDown);
    el.addEventListener("keyup", handleKeyUp);
    return () => {
      el.removeEventListener("keydown", handleKeyDown);
      el.removeEventListener("keyup", handleKeyUp);
    };
  }, [handleKeyDown, handleKeyUp]);

  // Fullscreen toggle
  const toggleFullscreen = useCallback(() => {
    if (!containerRef.current) return;
    if (!document.fullscreenElement) {
      containerRef.current.requestFullscreen();
      setFullscreen(true);
    } else {
      document.exitFullscreen();
      setFullscreen(false);
    }
  }, []);

  // Clipboard paste
  useEffect(() => {
    const handlePaste = (e: ClipboardEvent) => {
      const text = e.clipboardData?.getData("text");
      if (text) sendClipboard(text);
    };
    window.addEventListener("paste", handlePaste);
    return () => window.removeEventListener("paste", handlePaste);
  }, [sendClipboard]);

  // Auto-resize: observe the container and tell the agent to match.
  // Without this, the source video stays at whatever resolution it booted at
  // and the browser only letterboxes via object-contain, so the content looks
  // blurry/zoomed until you wiggle the window enough to trigger a re-layout.
  useEffect(() => {
    if (!connected) return;
    const el = containerRef.current;
    if (!el) return;

    let timer: ReturnType<typeof setTimeout> | null = null;
    let lastW = 0, lastH = 0;
    const fire = () => {
      const w = Math.round(el.clientWidth);
      const h = Math.round(el.clientHeight);
      if (w === lastW && h === lastH) return;
      if (w < 100 || h < 100) return; // ignore degenerate sizes
      lastW = w; lastH = h;
      sendResize(w, h);
    };

    // Initial sync on connect
    fire();

    const schedule = () => {
      if (timer) clearTimeout(timer);
      timer = setTimeout(fire, 200);
    };

    const ro = new ResizeObserver(schedule);
    ro.observe(el);
    window.addEventListener("resize", schedule);
    return () => {
      ro.disconnect();
      window.removeEventListener("resize", schedule);
      if (timer) clearTimeout(timer);
    };
  }, [connected, sendResize]);

  return (
    <div
      ref={containerRef}
      tabIndex={0}
      className={`relative bg-black focus:outline-none ${className}`}
      onContextMenu={handleContextMenu}
    >
      {/* Video element — native H.264 decode */}
      <video
        ref={videoRef}
        autoPlay
        playsInline
        muted
        onMouseMove={handleMouseMove}
        onMouseDown={handleMouseDown}
        onMouseUp={handleMouseUp}
        onWheel={handleWheel}
        className="w-full h-full object-contain cursor-none"
        style={{ imageRendering: "auto" }}
      />

      {/* Connection status overlay */}
      {!connected && (
        <div className="absolute inset-0 flex items-center justify-center bg-black/80">
          <div className="text-center text-white">
            <div className="size-10 border-2 border-white/20 border-t-white rounded-full animate-spin mx-auto mb-4" />
            <p className="text-sm opacity-60">Connecting to workspace...</p>
          </div>
        </div>
      )}

      {/* Controls overlay */}
      <div className="absolute top-2 right-2 flex items-center gap-2 opacity-0 hover:opacity-100 transition-opacity">
        {/* Stats toggle */}
        <button
          onClick={() => setShowStats(!showStats)}
          className="p-1.5 rounded bg-black/50 text-white/60 hover:text-white"
        >
          {connected ? <Signal className="size-4" /> : <SignalZero className="size-4" />}
        </button>

        {/* Fullscreen */}
        <button
          onClick={toggleFullscreen}
          className="p-1.5 rounded bg-black/50 text-white/60 hover:text-white"
        >
          {fullscreen ? <Minimize2 className="size-4" /> : <Maximize2 className="size-4" />}
        </button>
      </div>

      {/* Stats panel */}
      {showStats && connected && (
        <div className="absolute top-12 right-2 bg-black/80 rounded-lg p-3 text-xs text-white/60 font-mono space-y-1">
          <p>FPS: {stats.fps}</p>
          <p>Bitrate: {stats.bitrate} kbps</p>
          <p>Resolution: {stats.resolution}</p>
          <p>Codec: {stats.codec}</p>
        </div>
      )}
    </div>
  );
}
