"use client";

import { useState, useRef } from "react";
import { Upload, X, Link } from "lucide-react";
import { useTranslation } from "@/lib/i18n";
import { authFetch } from "@/lib/auth-fetch";

interface ImageUploadProps {
  value: string;
  onChange: (value: string) => void;
  label: string;
  hint?: string;
  previewHeight?: string;
}

export function ImageUpload({ value, onChange, label, hint, previewHeight = "h-12" }: ImageUploadProps) {
  const { t } = useTranslation();
  const [mode, setMode] = useState<"url" | "upload">(value.startsWith("data:") ? "upload" : "url");
  const [uploading, setUploading] = useState(false);
  const fileRef = useRef<HTMLInputElement>(null);

  async function handleUpload(file: File) {
    if (file.size > 500 * 1024) {
      alert("Max 500KB");
      return;
    }
    setUploading(true);
    const formData = new FormData();
    formData.append("file", file);
    try {
      const res = await authFetch("/api/admin/upload", { method: "POST", body: formData });
      const data = await res.json();
      if (data.url) {
        onChange(data.url);
      }
    } catch {}
    setUploading(false);
  }

  return (
    <div className="space-y-2">
      <div className="flex items-baseline gap-2">
        <label className="text-white/60 text-sm">{label}</label>
        {hint && <span className="text-white/20 text-xs">{hint}</span>}
      </div>

      {/* Mode toggle */}
      <div className="flex gap-1 mb-2">
        <button
          type="button"
          onClick={() => setMode("url")}
          className={`px-2.5 py-1 rounded text-xs transition-colors ${mode === "url" ? "bg-white/10 text-white" : "text-white/30 hover:text-white/50"}`}
        >
          <Link className="size-3 inline mr-1" />URL
        </button>
        <button
          type="button"
          onClick={() => setMode("upload")}
          className={`px-2.5 py-1 rounded text-xs transition-colors ${mode === "upload" ? "bg-white/10 text-white" : "text-white/30 hover:text-white/50"}`}
        >
          <Upload className="size-3 inline mr-1" />Upload
        </button>
      </div>

      {mode === "url" ? (
        <input
          type="text"
          value={value.startsWith("data:") ? "" : value}
          onChange={(e) => onChange(e.target.value)}
          placeholder="https://..."
          className="w-full bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-white text-sm placeholder-white/20"
        />
      ) : (
        <div
          onClick={() => fileRef.current?.click()}
          onDragOver={(e) => { e.preventDefault(); e.stopPropagation(); }}
          onDrop={(e) => { e.preventDefault(); e.stopPropagation(); const f = e.dataTransfer.files[0]; if (f) handleUpload(f); }}
          className="border-2 border-dashed border-white/10 rounded-lg p-4 text-center cursor-pointer hover:border-white/20 transition-colors"
        >
          <input ref={fileRef} type="file" accept="image/svg+xml,image/png,image/jpeg,image/webp" className="hidden"
            onChange={(e) => { const f = e.target.files?.[0]; if (f) handleUpload(f); }} />
          {uploading ? (
            <p className="text-white/40 text-sm">Uploading...</p>
          ) : (
            <p className="text-white/40 text-sm">
              <Upload className="size-4 inline mr-1" />
              Drop or click (SVG, PNG, JPG, max 500KB)
            </p>
          )}
        </div>
      )}

      {/* Preview */}
      {value && (
        <div className="flex items-center gap-2 mt-2">
          <img src={value} alt="" className={`${previewHeight} object-contain rounded bg-white/5 p-1`} />
          <button type="button" onClick={() => onChange("")} className="p-1 text-red-400/50 hover:text-red-400">
            <X className="size-4" />
          </button>
        </div>
      )}
    </div>
  );
}
