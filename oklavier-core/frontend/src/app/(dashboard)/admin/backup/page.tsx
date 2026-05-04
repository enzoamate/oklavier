"use client";

import { useState, useRef } from "react";
import { Download, Upload, Loader2, CheckCircle, AlertCircle } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useTranslation } from "@/lib/i18n";
import { authFetch } from "@/lib/auth-fetch";
import { useToast } from "@/components/toast";

interface ImportSummary {
  [key: string]: any;
}

export default function BackupPage() {
  const { t } = useTranslation();
  const { toast } = useToast();
  const fileRef = useRef<HTMLInputElement>(null);
  const [exporting, setExporting] = useState(false);
  const [importing, setImporting] = useState(false);
  const [preview, setPreview] = useState<any>(null);
  const [importResult, setImportResult] = useState<ImportSummary | null>(null);

  async function handleExport() {
    setExporting(true);
    try {
      const res = await authFetch("/api/admin/backup", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ _action: "export" }),
      });
      const data = await res.json();
      const blob = new Blob([JSON.stringify(data, null, 2)], { type: "application/json" });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `oklavier-backup-${new Date().toISOString().split("T")[0]}.json`;
      a.click();
      URL.revokeObjectURL(url);
      toast("success", t("admin.backup.exported"));
    } catch {
      toast("error", t("toast.error"));
    }
    setExporting(false);
  }

  function handleFileSelect(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;
    const reader = new FileReader();
    reader.onload = (ev) => {
      try {
        const json = JSON.parse(ev.target?.result as string);
        setPreview(json);
        setImportResult(null);
      } catch {
        toast("error", t("admin.backup.invalid_file"));
      }
    };
    reader.readAsText(file);
  }

  async function handleImport() {
    if (!preview) return;
    setImporting(true);
    try {
      const res = await authFetch("/api/admin/backup", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ _action: "import", ...preview }),
      });
      const data = await res.json();
      if (data.ok) {
        setImportResult(data.summary);
        toast("success", t("admin.backup.imported"));
      } else {
        toast("error", data.error || t("toast.error"));
      }
    } catch {
      toast("error", t("toast.error"));
    }
    setImporting(false);
  }

  function renderPreviewStats() {
    if (!preview) return null;
    return (
      <div className="grid grid-cols-2 sm:grid-cols-3 gap-3 mt-4">
        {preview.workspaces && (
          <div className="bg-white/5 rounded-lg p-3">
            <p className="text-white/40 text-xs">{t("admin.workspaces.title")}</p>
            <p className="text-white text-lg font-semibold">{preview.workspaces.length}</p>
          </div>
        )}
        {preview.groups && (
          <div className="bg-white/5 rounded-lg p-3">
            <p className="text-white/40 text-xs">{t("admin.groups.title")}</p>
            <p className="text-white text-lg font-semibold">{preview.groups.length}</p>
          </div>
        )}
        {preview.auth_methods && (
          <div className="bg-white/5 rounded-lg p-3">
            <p className="text-white/40 text-xs">{t("admin.auth_methods.title")}</p>
            <p className="text-white text-lg font-semibold">{preview.auth_methods.length}</p>
          </div>
        )}
        {preview.settings && (
          <div className="bg-white/5 rounded-lg p-3">
            <p className="text-white/40 text-xs">{t("admin.backup.settings")}</p>
            <p className="text-white text-lg font-semibold">{Object.keys(preview.settings).length}</p>
          </div>
        )}
        {preview.branding && (
          <div className="bg-white/5 rounded-lg p-3">
            <p className="text-white/40 text-xs">{t("admin.branding.title")}</p>
            <p className="text-white text-lg font-semibold">{preview.branding.app_name || "-"}</p>
          </div>
        )}
        {preview.version && (
          <div className="bg-white/5 rounded-lg p-3">
            <p className="text-white/40 text-xs">{t("admin.backup.version")}</p>
            <p className="text-white text-lg font-semibold">v{preview.version}</p>
          </div>
        )}
      </div>
    );
  }

  function renderImportResult() {
    if (!importResult) return null;
    return (
      <div className="mt-4 bg-green-500/10 border border-green-500/20 rounded-xl p-4">
        <div className="flex items-center gap-2 mb-3">
          <CheckCircle className="size-5 text-green-400" />
          <p className="text-green-400 font-medium">{t("admin.backup.import_success")}</p>
        </div>
        <div className="space-y-1 text-sm">
          {Object.entries(importResult).map(([key, val]) => (
            <div key={key} className="flex items-center gap-2 text-white/60">
              <span className="capitalize">{key.replace("_", " ")}:</span>
              <span className="text-white">{typeof val === "object" ? JSON.stringify(val) : String(val)}</span>
            </div>
          ))}
        </div>
      </div>
    );
  }

  return (
    <div>
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-white">{t("admin.backup.title")}</h1>
        <p className="text-white/50 text-sm">{t("admin.backup.subtitle")}</p>
      </div>

      <div className="grid gap-6 md:grid-cols-2">
        {/* Export */}
        <div className="bg-[#1a1f36]/50 border border-white/10 rounded-xl p-6">
          <div className="flex items-center gap-3 mb-4">
            <div className="p-2 bg-oklavier-blue/20 rounded-lg">
              <Download className="size-5 text-oklavier-blue" />
            </div>
            <div>
              <h2 className="text-white font-semibold">{t("admin.backup.export_title")}</h2>
              <p className="text-white/40 text-sm">{t("admin.backup.export_desc")}</p>
            </div>
          </div>
          <p className="text-white/30 text-xs mb-4">{t("admin.backup.export_includes")}</p>
          <Button onClick={handleExport} disabled={exporting} className="bg-oklavier-blue hover:bg-oklavier-blue/80 text-white gap-2">
            {exporting ? <Loader2 className="size-4 animate-spin" /> : <Download className="size-4" />}
            {t("admin.backup.export_button")}
          </Button>
        </div>

        {/* Import */}
        <div className="bg-[#1a1f36]/50 border border-white/10 rounded-xl p-6">
          <div className="flex items-center gap-3 mb-4">
            <div className="p-2 bg-orange-500/20 rounded-lg">
              <Upload className="size-5 text-orange-400" />
            </div>
            <div>
              <h2 className="text-white font-semibold">{t("admin.backup.import_title")}</h2>
              <p className="text-white/40 text-sm">{t("admin.backup.import_desc")}</p>
            </div>
          </div>

          <input ref={fileRef} type="file" accept=".json" onChange={handleFileSelect} className="hidden" />

          {!preview ? (
            <Button onClick={() => fileRef.current?.click()} variant="outline" className="border-white/10 text-white/60 hover:text-white gap-2">
              <Upload className="size-4" /> {t("admin.backup.select_file")}
            </Button>
          ) : (
            <div>
              <div className="flex items-center gap-2 mb-2">
                <AlertCircle className="size-4 text-orange-400" />
                <p className="text-orange-400 text-sm">{t("admin.backup.import_warning")}</p>
              </div>
              {renderPreviewStats()}
              <div className="flex gap-2 mt-4">
                <Button onClick={() => { setPreview(null); setImportResult(null); if (fileRef.current) fileRef.current.value = ""; }} variant="ghost" className="text-white/50">
                  {t("common.cancel")}
                </Button>
                <Button onClick={handleImport} disabled={importing} className="bg-orange-500 hover:bg-orange-500/80 text-white gap-2">
                  {importing ? <Loader2 className="size-4 animate-spin" /> : <Upload className="size-4" />}
                  {t("admin.backup.import_button")}
                </Button>
              </div>
              {renderImportResult()}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
