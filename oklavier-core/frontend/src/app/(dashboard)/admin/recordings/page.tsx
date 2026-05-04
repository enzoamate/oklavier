"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { Loader2, Download, ChevronLeft, ChevronRight, Video, Play } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useTranslation } from "@/lib/i18n";
import { usePostAPI } from "@/lib/api";
import { authFetch } from "@/lib/auth-fetch";

interface Recording {
  id: string;
  session_id: string;
  user_id: string;
  user_email: string;
  workspace_name: string;
  s3_key: string;
  file_size: number;
  duration_seconds: number;
  created_at: string;
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + " " + sizes[i];
}

function formatDuration(seconds: number): string {
  if (seconds === 0) return "-";
  const h = Math.floor(seconds / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  const s = seconds % 60;
  if (h > 0) return `${h}h ${m}m ${s}s`;
  if (m > 0) return `${m}m ${s}s`;
  return `${s}s`;
}

function formatDate(dateStr: string): string {
  try {
    return new Date(dateStr).toLocaleString();
  } catch {
    return dateStr;
  }
}

export default function AdminRecordingsPage() {
  const { t } = useTranslation();
  const router = useRouter();
  const [page, setPage] = useState(1);
  const perPage = 25;

  const { data, isLoading } = usePostAPI<{ recordings: Recording[]; total: number }>(
    `/api/admin/recordings`
  );

  const recordings = data?.recordings || [];
  const total = data?.total || 0;
  const totalPages = Math.max(1, Math.ceil(total / perPage));

  async function handleDownload(id: string, sessionId: string) {
    try {
      const res = await authFetch(`/api/admin/recordings/${id}/download`);
      if (!res.ok) return;

      const contentType = res.headers.get("content-type") || "";
      if (contentType.includes("application/octet-stream")) {
        // Local storage — backend proxied the file directly
        const blob = await res.blob();
        const url = URL.createObjectURL(blob);
        const a = document.createElement("a");
        a.href = url;
        a.download = `${sessionId}.guac`;
        a.click();
        URL.revokeObjectURL(url);
      } else {
        // S3 storage — backend returned JSON with S3 info
        const info = await res.json();
        if (info.endpoint && info.bucket && info.s3_key) {
          const url = `${info.endpoint}/${info.bucket}/${info.s3_key}`;
          window.open(url, "_blank");
        }
      }
    } catch {}
  }

  if (isLoading) {
    return (
      <div className="flex items-center gap-2 text-white/60">
        <Loader2 className="size-5 animate-spin" /> {t("common.loading")}
      </div>
    );
  }

  return (
    <div>
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-white">{t("admin.recordings.title")}</h1>
        <p className="text-white/50 text-sm">{t("admin.recordings.subtitle")}</p>
      </div>

      {recordings.length === 0 ? (
        <div className="bg-[#1a1f36]/50 border border-white/10 rounded-xl p-12 text-center">
          <Video className="size-12 mx-auto text-white/10 mb-4" />
          <p className="text-white/40 text-sm">{t("admin.recordings.empty")}</p>
          <p className="text-white/20 text-xs mt-1">{t("admin.recordings.empty_hint")}</p>
        </div>
      ) : (
        <div className="bg-[#1a1f36]/50 border border-white/10 rounded-xl overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-white/10">
                <th className="text-left text-white/40 font-medium px-4 py-3">{t("admin.recordings.date")}</th>
                <th className="text-left text-white/40 font-medium px-4 py-3">{t("admin.recordings.workspace")}</th>
                <th className="text-left text-white/40 font-medium px-4 py-3">{t("admin.recordings.user")}</th>
                <th className="text-left text-white/40 font-medium px-4 py-3">{t("admin.recordings.size")}</th>
                <th className="text-left text-white/40 font-medium px-4 py-3">{t("admin.recordings.duration")}</th>
                <th className="text-right text-white/40 font-medium px-4 py-3"></th>
              </tr>
            </thead>
            <tbody>
              {recordings.map((r) => (
                <tr key={r.id} className="border-b border-white/5 hover:bg-white/5 transition-colors">
                  <td className="px-4 py-3 text-white/70">{formatDate(r.created_at)}</td>
                  <td className="px-4 py-3 text-white">{r.workspace_name || "-"}</td>
                  <td className="px-4 py-3 text-white/60">{r.user_email || r.user_id || "-"}</td>
                  <td className="px-4 py-3 text-white/50">{formatBytes(r.file_size)}</td>
                  <td className="px-4 py-3 text-white/50">{formatDuration(r.duration_seconds)}</td>
                  <td className="px-4 py-3 text-right flex gap-1 justify-end">
                    <button
                      onClick={() => router.push(`/admin/recordings/${r.id}`)}
                      className="p-2 rounded-lg text-oklavier-blue/60 hover:text-oklavier-blue hover:bg-oklavier-blue/10 transition-colors"
                      title={t("admin.recordings.replay")}
                    >
                      <Play className="size-4" />
                    </button>
                    <button
                      onClick={() => handleDownload(r.id, r.session_id)}
                      className="p-2 rounded-lg text-white/40 hover:text-white hover:bg-white/10 transition-colors"
                      title={t("admin.recordings.download")}
                    >
                      <Download className="size-4" />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>

          {/* Pagination */}
          {totalPages > 1 && (
            <div className="flex items-center justify-between px-4 py-3 border-t border-white/10">
              <p className="text-white/30 text-xs">
                {t("common.showing")} {(page - 1) * perPage + 1}-{Math.min(page * perPage, total)} / {total}
              </p>
              <div className="flex gap-1">
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => setPage(Math.max(1, page - 1))}
                  disabled={page <= 1}
                  className="text-white/40 hover:text-white"
                >
                  <ChevronLeft className="size-4" />
                </Button>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => setPage(Math.min(totalPages, page + 1))}
                  disabled={page >= totalPages}
                  className="text-white/40 hover:text-white"
                >
                  <ChevronRight className="size-4" />
                </Button>
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
