"use client";

import { useEffect, useRef, useState, useCallback } from "react";
import { authFetch } from "./auth-fetch";
import { getAccessToken } from "./token-store";

interface ProxyStreamOptions {
  sessionId: string;
  onConnected?: () => void;
  onDisconnected?: () => void;
  onError?: (error: string) => void;
}

interface InputEvent {
  type: "key" | "mouse";
  key?: number;
  down?: boolean;
  x?: number;
  y?: number;
  buttons?: number;
}

export function useProxyStream({ sessionId, onConnected, onDisconnected, onError }: ProxyStreamOptions) {
  const videoRef = useRef<HTMLVideoElement>(null);
  const pcRef = useRef<RTCPeerConnection | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const [connected, setConnected] = useState(false);
  const [stats, setStats] = useState({ fps: 0, bitrate: 0, codec: "", resolution: "" });

  // Connect WebRTC for video + WebSocket for control
  const connect = useCallback(async () => {
    try {
      // 1. Create WebRTC PeerConnection
      const pc = new RTCPeerConnection({
        iceServers: [{ urls: "stun:stun.l.google.com:19302" }],
      });
      pcRef.current = pc;

      // Handle incoming video track
      pc.ontrack = (event) => {

        if (videoRef.current && event.streams[0]) {
          videoRef.current.srcObject = event.streams[0];
          videoRef.current.play().catch(() => {});
        }
      };

      pc.oniceconnectionstatechange = () => {

        if (pc.iceConnectionState === "connected" || pc.iceConnectionState === "completed") {
          setConnected(true);
          onConnected?.();
        }
        if (pc.iceConnectionState === "disconnected" || pc.iceConnectionState === "failed") {
          setConnected(false);
          onDisconnected?.();
        }
      };

      // We need to add a transceiver to receive video
      pc.addTransceiver("video", { direction: "recvonly" });

      // 2. Create offer
      const offer = await pc.createOffer();
      await pc.setLocalDescription(offer);

      // 3. Send offer to agent via API (using authFetch for Bearer token)
      const offerRes = await authFetch(`/api/proxy/webrtc/offer/${sessionId}`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ sdp: offer.sdp, type: offer.type }),
      });

      if (!offerRes.ok) {
        throw new Error(`WebRTC offer failed: ${offerRes.status}`);
      }

      const answer = await offerRes.json();

      // 4. Set remote description (agent's answer)
      await pc.setRemoteDescription(new RTCSessionDescription({
        type: "answer",
        sdp: answer.sdp,
      }));

      // 5. Handle ICE candidates (trickle ICE)
      pc.onicecandidate = async (event) => {
        if (event.candidate) {
          await authFetch(`/api/proxy/webrtc/ice/${sessionId}`, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify(event.candidate.toJSON()),
          }).catch(() => {});
        }
      };

      // 6. Connect WebSocket for control channel (pass token as query param)
      const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
      const token = getAccessToken();
      const wsUrl = `${protocol}//${window.location.host}/proxy/ws/${sessionId}${token ? `?token=${encodeURIComponent(token)}` : ""}`;
      const ws = new WebSocket(wsUrl);
      wsRef.current = ws;

      ws.onopen = () => {

      };

      ws.onmessage = (e) => {
        try {
          const msg = JSON.parse(e.data);
          if (msg.ch === "control" && msg.type === "pong") {
            // Latency measurement
          }
          if (msg.ch === "clipboard" && msg.type === "text") {
            // Incoming clipboard from remote
            navigator.clipboard.writeText(msg.data?.text || "").catch(() => {});
          }
        } catch {}
      };

      ws.onclose = () => {

      };

      // 7. Start stats polling
      const statsInterval = setInterval(async () => {
        if (pc.connectionState !== "connected") return;
        const rtcStats = await pc.getStats();
        rtcStats.forEach((report) => {
          if (report.type === "inbound-rtp" && report.kind === "video") {
            setStats({
              fps: report.framesPerSecond || 0,
              bitrate: Math.round((report.bytesReceived || 0) * 8 / 1000),
              codec: report.codecId || "",
              resolution: `${report.frameWidth || 0}x${report.frameHeight || 0}`,
            });
          }
        });
      }, 2000);

      return () => clearInterval(statsInterval);

    } catch (err: any) {

      onError?.(err.message || "Connection failed");
    }
  }, [sessionId]);

  // Send input events via WebSocket
  const sendInput = useCallback((event: InputEvent) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({
        ch: "input",
        type: event.type,
        data: event,
      }));
    }
  }, []);

  // Send clipboard
  const sendClipboard = useCallback((text: string) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({
        ch: "clipboard",
        type: "set",
        data: { text },
      }));
    }
  }, []);

  // Resize
  const sendResize = useCallback((width: number, height: number) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({
        ch: "control",
        type: "resize",
        data: { Width: width, Height: height },
      }));
    }
  }, []);

  // Disconnect
  const disconnect = useCallback(() => {
    if (wsRef.current) {
      wsRef.current.close();
      wsRef.current = null;
    }
    if (pcRef.current) {
      pcRef.current.close();
      pcRef.current = null;
    }
    setConnected(false);
  }, []);

  // Auto-connect on mount
  useEffect(() => {
    connect();
    return () => disconnect();
  }, [sessionId]);

  return {
    videoRef,
    connected,
    stats,
    sendInput,
    sendClipboard,
    sendResize,
    disconnect,
  };
}
