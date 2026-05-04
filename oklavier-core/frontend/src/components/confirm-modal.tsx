"use client";

import { Loader2, AlertTriangle } from "lucide-react";
import { useTranslation } from "@/lib/i18n";

interface ConfirmModalProps {
  open: boolean;
  title: string;
  message: string;
  confirmLabel?: string;
  loading?: boolean;
  variant?: "danger" | "warning";
  onConfirm: () => void;
  onCancel: () => void;
}

export function ConfirmModal({
  open,
  title,
  message,
  confirmLabel,
  loading = false,
  variant = "danger",
  onConfirm,
  onCancel,
}: ConfirmModalProps) {
  const { t } = useTranslation();

  if (!open) return null;

  return (
    <div className="fixed inset-0 z-[60] flex items-center justify-center">
      <div className="absolute inset-0 bg-black/50 backdrop-blur-sm" onClick={onCancel} />
      <div className="relative z-[70] w-full max-w-sm bg-[#1a1f36] border border-white/10 rounded-2xl p-6 shadow-2xl">
        <div className="flex items-start gap-4 mb-4">
          <div className={`size-10 rounded-xl flex items-center justify-center shrink-0 ${
            variant === "danger" ? "bg-red-500/10" : "bg-yellow-500/10"
          }`}>
            <AlertTriangle className={`size-5 ${
              variant === "danger" ? "text-red-400" : "text-yellow-400"
            }`} />
          </div>
          <div>
            <h3 className="text-white font-semibold">{title}</h3>
            <p className="text-white/50 text-sm mt-1">{message}</p>
          </div>
        </div>
        <div className="flex gap-3">
          <button
            onClick={onCancel}
            disabled={loading}
            className="flex-1 border border-white/10 text-white/70 hover:text-white hover:bg-white/10 rounded-lg py-2 text-sm font-medium transition-colors"
          >
            {t("common.cancel")}
          </button>
          <button
            onClick={onConfirm}
            disabled={loading}
            className={`flex-1 rounded-lg py-2 text-sm font-medium text-white transition-colors disabled:opacity-50 ${
              variant === "danger"
                ? "bg-red-500 hover:bg-red-600"
                : "bg-yellow-500 hover:bg-yellow-600"
            }`}
          >
            {loading ? <Loader2 className="size-4 animate-spin mx-auto" /> : (confirmLabel || t("common.delete"))}
          </button>
        </div>
      </div>
    </div>
  );
}
