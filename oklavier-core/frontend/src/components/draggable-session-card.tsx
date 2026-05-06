"use client";

import { useEffect, useRef, useState } from "react";
import { Minus, Maximize2 as Restore, X, Play, Trash2, Maximize2, Loader2, Monitor } from "lucide-react";

type Position = { x: number; y: number };
type CardState = { pos: Position; minimized: boolean };

// SessionLike must match the parent's WorkspaceSession shape exactly,
// because handleConnect/handleDestroy are typed against WorkspaceSession
// and parameter types are contravariant: a callback accepting a wider
// shape can't be called with a narrower one.
interface SessionLike {
  session_id: string;
  image: { image_id: string; friendly_name: string; image_src: string };
  operational_status: string;
  start_date: string;
  expiration_date: string;
  container_ip: string;
  keepalive_date: string;
  agent_id?: string;
  agent_vnc_url?: string;
  session_type?: string;
  workspace_type?: string;
}

interface Props {
  session: SessionLike;
  index: number; // for stacked default position
  imgSrcResolver: (img: SessionLike["image"]) => string;
  timeAgo: (d: string) => string;
  timeLeft: (d: string) => string;
  onConnect: (s: SessionLike) => void;
  onDestroy: (id: string) => void;
  destroying: string | null;
}

const STORAGE_KEY = "oklavier-session-cards";
const CARD_WIDTH = 192; // w-48
const MARGIN = 12;
const SNAP_DISTANCE = 80;

function loadStates(): Record<string, CardState> {
  if (typeof window === "undefined") return {};
  try {
    return JSON.parse(localStorage.getItem(STORAGE_KEY) || "{}");
  } catch {
    return {};
  }
}

function saveState(id: string, state: CardState) {
  if (typeof window === "undefined") return;
  const all = loadStates();
  all[id] = state;
  localStorage.setItem(STORAGE_KEY, JSON.stringify(all));
}

export function DraggableSessionCard({
  session,
  index,
  imgSrcResolver,
  timeAgo,
  timeLeft,
  onConnect,
  onDestroy,
  destroying,
}: Props) {
  const cardRef = useRef<HTMLDivElement>(null);
  const dragStart = useRef<{ mx: number; my: number; cx: number; cy: number } | null>(null);

  // Default position: stacked top-left, 80px from top, with 12px gap per card
  const defaultPos: Position = { x: MARGIN + 8, y: 80 + index * 220 };
  const [state, setState] = useState<CardState>(() => {
    const stored = loadStates()[session.session_id];
    return stored ?? { pos: defaultPos, minimized: false };
  });

  // Persist whenever the state changes
  useEffect(() => {
    saveState(session.session_id, state);
  }, [session.session_id, state]);

  // Snap helper — once drag ends, snap to nearest vertical edge.
  function snapToEdge(p: Position): Position {
    if (typeof window === "undefined") return p;
    const w = window.innerWidth;
    const h = window.innerHeight;
    const cardW = cardRef.current?.offsetWidth ?? CARD_WIDTH;
    const cardH = cardRef.current?.offsetHeight ?? 200;

    // Clamp inside the viewport
    let x = Math.max(MARGIN, Math.min(w - cardW - MARGIN, p.x));
    const y = Math.max(MARGIN, Math.min(h - cardH - MARGIN, p.y));

    // Snap to nearest vertical edge if close
    const distLeft = x;
    const distRight = w - cardW - x;
    if (distLeft < SNAP_DISTANCE && distLeft <= distRight) {
      x = MARGIN;
    } else if (distRight < SNAP_DISTANCE) {
      x = w - cardW - MARGIN;
    }
    return { x, y };
  }

  function onPointerDown(e: React.PointerEvent) {
    // Only drag from the header (data-drag-handle parent), not from buttons.
    const target = e.target as HTMLElement;
    if (target.closest("button")) return;
    // Stop the browser from claiming this gesture (text selection, native
    // drag-and-drop preview, horizontal swipe-to-navigate on trackpads).
    e.preventDefault();
    dragStart.current = {
      mx: e.clientX,
      my: e.clientY,
      cx: state.pos.x,
      cy: state.pos.y,
    };
    try {
      (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
    } catch {
      // setPointerCapture can throw if the pointer was already released
      // (e.g. user lifted finger between down and React batching). Safe to
      // ignore — onPointerMove will still get the bubbled events.
    }
  }

  function onPointerMove(e: React.PointerEvent) {
    if (!dragStart.current) return;
    const dx = e.clientX - dragStart.current.mx;
    const dy = e.clientY - dragStart.current.my;
    // Clamp during drag so the card can never escape into territory where the
    // browser steals the gesture (e.g. negative x triggers swipe-back on some
    // trackpads). snapToEdge clamps to viewport on release; we apply a looser
    // bound here so the user still gets natural feedback while dragging.
    const w = typeof window !== "undefined" ? window.innerWidth : 9999;
    const h = typeof window !== "undefined" ? window.innerHeight : 9999;
    const cardW = cardRef.current?.offsetWidth ?? CARD_WIDTH;
    const cardH = cardRef.current?.offsetHeight ?? 200;
    const rawX = dragStart.current.cx + dx;
    const rawY = dragStart.current.cy + dy;
    const x = Math.max(0, Math.min(w - cardW, rawX));
    const y = Math.max(0, Math.min(h - cardH, rawY));
    setState((s) => ({ ...s, pos: { x, y } }));
  }

  function endDrag(e: React.PointerEvent, snap: boolean) {
    if (!dragStart.current) return;
    dragStart.current = null;
    try {
      (e.currentTarget as HTMLElement).releasePointerCapture(e.pointerId);
    } catch {
      // Pointer capture may already have been released by the browser
      // (e.g. window blur, gesture cancel) — ignore.
    }
    if (snap) setState((s) => ({ ...s, pos: snapToEdge(s.pos) }));
  }

  function onPointerUp(e: React.PointerEvent) { endDrag(e, true); }
  function onPointerCancel(e: React.PointerEvent) { endDrag(e, true); }
  function onLostPointerCapture(e: React.PointerEvent) { endDrag(e, false); }

  // Re-snap on window resize
  useEffect(() => {
    function onResize() { setState((s) => ({ ...s, pos: snapToEdge(s.pos) })); }
    window.addEventListener("resize", onResize);
    return () => window.removeEventListener("resize", onResize);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <div
      ref={cardRef}
      className="fixed z-30 w-48 backdrop-blur-xl bg-black/40 border border-white/10 rounded-xl overflow-hidden shadow-2xl select-none"
      style={{ left: state.pos.x, top: state.pos.y, cursor: dragStart.current ? "grabbing" : "default" }}
    >
      {/* Header — drag handle. touch-action:none prevents the browser from
          claiming the gesture for swipe-back / pull-to-refresh / scroll. */}
      <div
        className="flex items-center gap-2 p-3 cursor-grab active:cursor-grabbing"
        style={{ touchAction: "none", overscrollBehavior: "contain" }}
        onPointerDown={onPointerDown}
        onPointerMove={onPointerMove}
        onPointerUp={onPointerUp}
        onPointerCancel={onPointerCancel}
        onLostPointerCapture={onLostPointerCapture}
      >
        {session.image.image_src && (
          <img src={imgSrcResolver(session.image)} alt="" className="size-8 rounded pointer-events-none" />
        )}
        <div className="flex-1 min-w-0">
          <p className="text-white text-sm font-medium truncate">{session.image.friendly_name}</p>
          {!state.minimized && (
            <div className="flex items-center gap-2 text-xs text-white/60">
              <span className="text-green-400">{timeAgo(session.start_date)}</span>
              <span className="text-red-400">{timeLeft(session.expiration_date)}</span>
            </div>
          )}
        </div>
        <button
          onClick={() => setState((s) => ({ ...s, minimized: !s.minimized }))}
          className="text-white/40 hover:text-white/80"
          title={state.minimized ? "Restore" : "Minimize"}
        >
          {state.minimized ? <Restore className="size-3.5" /> : <Minus className="size-3.5" />}
        </button>
        <button onClick={() => onDestroy(session.session_id)} className="text-white/40 hover:text-white/80" title="Close">
          {destroying === session.session_id ? <Loader2 className="size-3.5 animate-spin" /> : <X className="size-3.5" />}
        </button>
      </div>

      {!state.minimized && (
        <>
          <div className="mx-2 mb-2 rounded-lg bg-white/5 aspect-video flex items-center justify-center relative overflow-hidden">
            <img
              src={`/api/sessions/${encodeURIComponent(session.session_id)}/screenshot`}
              alt=""
              className="w-full h-full object-cover pointer-events-none"
              onError={(e) => { (e.target as HTMLImageElement).style.display = "none"; }}
            />
            <Monitor className="size-6 text-white/20 absolute" />
            <div className={`absolute top-1.5 right-1.5 size-2.5 rounded-full ${session.operational_status === "running" ? "bg-green-500 shadow-[0_0_6px_rgba(34,197,94,0.5)]" : "bg-yellow-500 shadow-[0_0_6px_rgba(234,179,8,0.5)]"}`} />
          </div>
          <div className="flex border-t border-white/10">
            <button onClick={() => onConnect(session)} className="flex-1 flex items-center justify-center py-2.5 text-oklavier-blue hover:bg-white/5 transition-colors" title="Open"><Play className="size-4" /></button>
            <button onClick={() => onDestroy(session.session_id)} className="flex-1 flex items-center justify-center py-2.5 text-white/50 hover:bg-white/5 transition-colors" title="Destroy"><Trash2 className="size-4" /></button>
            <button onClick={() => { window.location.href = `/sessions/${session.session_id}`; }} className="flex-1 flex items-center justify-center py-2.5 text-white/50 hover:bg-white/5 transition-colors" title="Fullscreen"><Maximize2 className="size-4" /></button>
          </div>
        </>
      )}
    </div>
  );
}

// Wrapper that maps an array of sessions to draggable cards.
export function ActiveSessionStack(props: {
  sessions: SessionLike[];
  imgSrcResolver: Props["imgSrcResolver"];
  timeAgo: Props["timeAgo"];
  timeLeft: Props["timeLeft"];
  onConnect: Props["onConnect"];
  onDestroy: Props["onDestroy"];
  destroying: string | null;
}) {
  const { sessions, ...rest } = props;
  return (
    <>
      {sessions.map((s, i) => (
        <DraggableSessionCard key={s.session_id} session={s} index={i} {...rest} />
      ))}
    </>
  );
}

export type { SessionLike };
