"use client";

import { useState, useEffect, useRef } from "react";
import { RefreshCw, Loader2, Terminal, ChevronDown } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useTranslation } from "@/lib/i18n";
import { useAPI, invalidate } from "@/lib/api";

interface LogEntry {
  timestamp: string;
  source: string;
  level: string;
  message: string;
}

interface LogSource {
  id: string;
  name: string;
}

export default function LogsPage() {
  const { t } = useTranslation();
  const [selectedSource, setSelectedSource] = useState("all");
  const [autoRefresh, setAutoRefresh] = useState(true);
  const [showSourceMenu, setShowSourceMenu] = useState(false);
  const logsEndRef = useRef<HTMLDivElement>(null);

  const apiPath = `/api/admin/logs?source=${selectedSource}&limit=200`;
  const { data: logsData, isLoading: loading } = useAPI<{ logs: LogEntry[]; sources: LogSource[] }>(
    autoRefresh ? apiPath : null,
    { refreshInterval: autoRefresh ? 5000 : 0 }
  );
  // Also fetch once when paused (no polling) so we have data to display
  const { data: staticData } = useAPI<{ logs: LogEntry[]; sources: LogSource[] }>(
    !autoRefresh ? apiPath : null
  );

  const currentData = autoRefresh ? logsData : staticData;
  const logs = currentData?.logs || [];
  const sources = currentData?.sources || [];

  useEffect(() => {
    if (autoRefresh) {
      logsEndRef.current?.scrollIntoView({ behavior: "smooth" });
    }
  }, [logs, autoRefresh]);

  function formatTime(ts: string) {
    try {
      return new Date(ts).toLocaleTimeString();
    } catch {
      return ts;
    }
  }

  function getSourceColor(source: string) {
    if (source === "oklavier-api") return "text-oklavier-blue";
    const colors = ["text-green-400", "text-purple-400", "text-orange-400", "text-pink-400", "text-cyan-400"];
    const idx = sources.findIndex(s => s.name === source);
    return colors[idx % colors.length] || "text-white/50";
  }

  // Only show full-page loader on initial load (no data yet)
  if (!currentData && loading) return <div className="flex items-center gap-2 text-white/60"><Loader2 className="size-5 animate-spin" /> {t("common.loading")}</div>;

  return (
    <div className="flex flex-col h-[calc(100vh-8rem)]">
      {/* Header */}
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-3">
          <Terminal className="size-6 text-white/40" />
          <div>
            <h1 className="text-2xl font-bold text-white">{t("admin.logs.title")}</h1>
            <p className="text-white/50 text-sm">{t("admin.logs.subtitle", { count: logs.length })}</p>
          </div>
        </div>
        <div className="flex items-center gap-2">
          {/* Source filter */}
          <div className="relative">
            <button
              onClick={() => setShowSourceMenu(!showSourceMenu)}
              className="flex items-center gap-2 bg-[#1a1f36] border border-white/10 rounded-lg px-3 py-1.5 text-sm text-white/70 hover:text-white transition-colors"
            >
              {selectedSource === "all" ? t("admin.logs.all_sources") : sources.find(s => s.id === selectedSource)?.name || selectedSource}
              <ChevronDown className="size-3.5" />
            </button>
            {showSourceMenu && (
              <>
                <div className="fixed inset-0 z-40" onClick={() => setShowSourceMenu(false)} />
                <div className="absolute right-0 top-10 z-50 w-48 bg-[#1a1f36] border border-white/10 rounded-xl shadow-2xl overflow-hidden">
                  <button onClick={() => { setSelectedSource("all"); setShowSourceMenu(false); }}
                    className={`w-full text-left px-4 py-2 text-sm transition-colors ${selectedSource === "all" ? "bg-oklavier-blue/20 text-white" : "text-white/60 hover:bg-white/5"}`}>
                    {t("admin.logs.all_sources")}
                  </button>
                  {sources.map(s => (
                    <button key={s.id} onClick={() => { setSelectedSource(s.id); setShowSourceMenu(false); }}
                      className={`w-full text-left px-4 py-2 text-sm transition-colors ${selectedSource === s.id ? "bg-oklavier-blue/20 text-white" : "text-white/60 hover:bg-white/5"}`}>
                      {s.name}
                    </button>
                  ))}
                </div>
              </>
            )}
          </div>

          {/* Auto-refresh toggle */}
          <button
            onClick={() => setAutoRefresh(!autoRefresh)}
            className={`px-3 py-1.5 rounded-lg text-sm transition-colors ${autoRefresh ? "bg-green-500/20 text-green-400" : "bg-white/5 text-white/40"}`}
          >
            {autoRefresh ? t("admin.logs.live") : t("admin.logs.paused")}
          </button>

          <Button onClick={() => invalidate(apiPath)} className="bg-oklavier-blue hover:bg-oklavier-purple">
            <RefreshCw className="size-4 mr-2" /> {t("common.refresh")}
          </Button>
        </div>
      </div>

      {/* Log viewer */}
      <div className="flex-1 bg-[#0a0d1a] border border-white/10 rounded-xl overflow-auto font-mono text-xs">
        {logs.length === 0 ? (
          <div className="flex items-center justify-center h-full text-white/20">
            <Terminal className="size-8 mr-3" /> {t("admin.logs.no_logs")}
          </div>
        ) : (
          <div className="p-3 space-y-0.5">
            {logs.map((entry, i) => (
              <div key={i} className="flex gap-2 hover:bg-white/5 px-2 py-0.5 rounded">
                <span className="text-white/25 shrink-0 w-20">{formatTime(entry.timestamp)}</span>
                <span className={`shrink-0 w-28 truncate ${getSourceColor(entry.source)}`}>{entry.source}</span>
                <span className="text-white/70 whitespace-pre-wrap break-all">{entry.message.trim()}</span>
              </div>
            ))}
            <div ref={logsEndRef} />
          </div>
        )}
      </div>
    </div>
  );
}
