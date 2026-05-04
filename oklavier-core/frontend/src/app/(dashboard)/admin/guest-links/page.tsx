"use client";

import { useState } from "react";
import { Loader2, Plus, Trash2, Copy, Link, Lock, ChevronLeft, ChevronRight, ExternalLink } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useTranslation } from "@/lib/i18n";
import { usePostAPI, invalidate } from "@/lib/api";
import { authFetch } from "@/lib/auth-fetch";
import { useToast } from "@/components/toast";
import { ConfirmModal } from "@/components/confirm-modal";

interface GuestLink {
  id: string;
  workspace_id: string;
  workspace_name: string;
  token: string;
  created_by: string;
  label: string;
  has_password: boolean;
  max_uses: number;
  used_count: number;
  expires_at: string;
  created_at: string;
}

interface Workspace {
  id: string;
  friendly_name: string;
}

function formatDate(dateStr: string): string {
  try {
    return new Date(dateStr).toLocaleString();
  } catch {
    return dateStr;
  }
}

function isExpired(dateStr: string): boolean {
  try {
    return new Date(dateStr) < new Date();
  } catch {
    return false;
  }
}

export default function GuestLinksPage() {
  const { t } = useTranslation();
  const { toast } = useToast();
  const [page, setPage] = useState(1);
  const perPage = 25;
  const [showCreate, setShowCreate] = useState(false);
  const [deleteId, setDeleteId] = useState<string | null>(null);
  const [copiedToken, setCopiedToken] = useState<string | null>(null);

  // Form state
  const [workspaceId, setWorkspaceId] = useState("");
  const [label, setLabel] = useState("");
  const [password, setPassword] = useState("");
  const [maxUses, setMaxUses] = useState(1);
  const [duration, setDuration] = useState("24h");
  const [creating, setCreating] = useState(false);

  const { data, isLoading } = usePostAPI<{ links: GuestLink[]; total: number }>(
    `/api/admin/guest-links`
  );
  const { data: wsData } = usePostAPI<{ workspaces: Workspace[]; total: number }>(
    `/api/admin/workspaces`
  );

  const links = data?.links || [];
  const total = data?.total || 0;
  const totalPages = Math.max(1, Math.ceil(total / perPage));
  const workspaces = wsData?.workspaces || [];

  async function handleCreate() {
    if (!workspaceId) return;
    setCreating(true);
    try {
      const res = await authFetch("/api/admin/guest-links", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ _action: "create", workspace_id: workspaceId, label, password, max_uses: maxUses, duration }),
      });
      const data = await res.json();
      if (data.ok) {
        toast("success", t("toast.created"));
        setShowCreate(false);
        setLabel("");
        setPassword("");
        setMaxUses(1);
        setDuration("24h");
        setWorkspaceId("");
        invalidate("/api/admin/guest-links");
        // Show the token for copying
        if (data.token) {
          const url = `${window.location.origin}/guest/${data.token}`;
          navigator.clipboard.writeText(url);
          toast("success", t("toast.copied"));
        }
      } else {
        toast("error", data.error || t("toast.error"));
      }
    } catch {
      toast("error", t("toast.error"));
    }
    setCreating(false);
  }

  async function handleDelete() {
    if (!deleteId) return;
    const res = await authFetch("/api/admin/guest-links", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ _action: "delete", id: deleteId }),
    });
    const data = await res.json();
    if (data.ok) {
      toast("success", t("toast.deleted"));
      invalidate("/api/admin/guest-links");
    }
    setDeleteId(null);
  }

  function copyLink(token: string) {
    const url = `${window.location.origin}/guest/${token}`;
    navigator.clipboard.writeText(url);
    setCopiedToken(token);
    toast("success", t("toast.copied"));
    setTimeout(() => setCopiedToken(null), 2000);
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
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-white">{t("admin.guest_links.title")}</h1>
          <p className="text-white/50 text-sm">{t("admin.guest_links.subtitle", { count: total })}</p>
        </div>
        <Button onClick={() => setShowCreate(true)} className="bg-oklavier-blue hover:bg-oklavier-blue/80 text-white gap-2">
          <Plus className="size-4" /> {t("admin.guest_links.create")}
        </Button>
      </div>

      {/* Create modal */}
      {showCreate && (
        <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50" onClick={() => setShowCreate(false)}>
          <div className="bg-[#1a1f36] border border-white/10 rounded-xl p-6 w-full max-w-md" onClick={(e) => e.stopPropagation()}>
            <h2 className="text-lg font-semibold text-white mb-4">{t("admin.guest_links.create")}</h2>
            <div className="space-y-4">
              <div>
                <label className="block text-white/60 text-sm mb-1">{t("admin.guest_links.workspace")}</label>
                <select
                  value={workspaceId}
                  onChange={(e) => setWorkspaceId(e.target.value)}
                  className="w-full bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-white text-sm focus:ring-oklavier-blue focus:border-oklavier-blue"
                >
                  <option value="">{t("admin.guest_links.select_workspace")}</option>
                  {workspaces.map((w) => (
                    <option key={w.id} value={w.id}>{w.friendly_name}</option>
                  ))}
                </select>
              </div>
              <div>
                <label className="block text-white/60 text-sm mb-1">{t("admin.guest_links.label")}</label>
                <Input value={label} onChange={(e) => setLabel(e.target.value)} placeholder={t("admin.guest_links.label_placeholder")} className="bg-white/5 border-white/10 text-white" />
              </div>
              <div>
                <label className="block text-white/60 text-sm mb-1">{t("admin.guest_links.duration")}</label>
                <select
                  value={duration}
                  onChange={(e) => setDuration(e.target.value)}
                  className="w-full bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-white text-sm"
                >
                  <option value="1h">1 {t("admin.guest_links.hour")}</option>
                  <option value="8h">8 {t("admin.guest_links.hours")}</option>
                  <option value="24h">24 {t("admin.guest_links.hours")}</option>
                  <option value="7d">7 {t("admin.guest_links.days")}</option>
                  <option value="30d">30 {t("admin.guest_links.days")}</option>
                </select>
              </div>
              <div>
                <label className="block text-white/60 text-sm mb-1">{t("admin.guest_links.max_uses")}</label>
                <div className="flex gap-2">
                  {[1, 5, 10, 0].map((n) => (
                    <button
                      key={n}
                      onClick={() => setMaxUses(n)}
                      className={`px-3 py-1 rounded-lg text-sm border transition-colors ${maxUses === n ? "bg-oklavier-blue/20 border-oklavier-blue text-white" : "border-white/10 text-white/50 hover:text-white hover:border-white/30"}`}
                    >
                      {n === 0 ? t("admin.guest_links.unlimited") : n}
                    </button>
                  ))}
                </div>
              </div>
              <div>
                <label className="block text-white/60 text-sm mb-1">{t("admin.guest_links.password_optional")}</label>
                <Input type="password" value={password} onChange={(e) => setPassword(e.target.value)} placeholder={t("admin.guest_links.password_placeholder")} className="bg-white/5 border-white/10 text-white" />
              </div>
            </div>
            <div className="flex justify-end gap-2 mt-6">
              <Button variant="ghost" onClick={() => setShowCreate(false)} className="text-white/50">{t("common.cancel")}</Button>
              <Button onClick={handleCreate} disabled={!workspaceId || creating} className="bg-oklavier-blue hover:bg-oklavier-blue/80 text-white">
                {creating && <Loader2 className="size-4 animate-spin mr-2" />}
                {t("common.create")}
              </Button>
            </div>
          </div>
        </div>
      )}

      {/* Delete confirm */}
      <ConfirmModal
        open={!!deleteId}
        onCancel={() => setDeleteId(null)}
        onConfirm={handleDelete}
        title={t("common.confirm")}
        message={t("admin.guest_links.confirm_delete")}
      />

      {links.length === 0 ? (
        <div className="bg-[#1a1f36]/50 border border-white/10 rounded-xl p-12 text-center">
          <Link className="size-12 mx-auto text-white/10 mb-4" />
          <p className="text-white/40 text-sm">{t("admin.guest_links.empty")}</p>
          <p className="text-white/20 text-xs mt-1">{t("admin.guest_links.empty_hint")}</p>
        </div>
      ) : (
        <div className="bg-[#1a1f36]/50 border border-white/10 rounded-xl overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-white/10">
                <th className="text-left text-white/40 font-medium px-4 py-3">{t("admin.guest_links.label")}</th>
                <th className="text-left text-white/40 font-medium px-4 py-3">{t("admin.guest_links.workspace")}</th>
                <th className="text-left text-white/40 font-medium px-4 py-3">{t("admin.guest_links.uses")}</th>
                <th className="text-left text-white/40 font-medium px-4 py-3">{t("admin.guest_links.expires")}</th>
                <th className="text-left text-white/40 font-medium px-4 py-3">{t("admin.guest_links.status_col")}</th>
                <th className="text-right text-white/40 font-medium px-4 py-3"></th>
              </tr>
            </thead>
            <tbody>
              {links.map((link) => {
                const expired = isExpired(link.expires_at);
                const maxed = link.max_uses > 0 && link.used_count >= link.max_uses;
                const active = !expired && !maxed;
                return (
                  <tr key={link.id} className="border-b border-white/5 hover:bg-white/5 transition-colors">
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-2">
                        <span className="text-white">{link.label || "-"}</span>
                        {link.has_password && <Lock className="size-3 text-white/30" />}
                      </div>
                    </td>
                    <td className="px-4 py-3 text-white/70">{link.workspace_name}</td>
                    <td className="px-4 py-3 text-white/60">
                      {link.used_count}/{link.max_uses === 0 ? "\u221e" : link.max_uses}
                    </td>
                    <td className="px-4 py-3 text-white/50">{formatDate(link.expires_at)}</td>
                    <td className="px-4 py-3">
                      <span className={`px-2 py-0.5 rounded-full text-xs ${active ? "bg-green-500/20 text-green-400" : "bg-red-500/20 text-red-400"}`}>
                        {active ? t("common.active") : t("admin.guest_links.expired")}
                      </span>
                    </td>
                    <td className="px-4 py-3 text-right flex gap-1 justify-end">
                      <button
                        onClick={() => copyLink(link.token)}
                        className="p-2 rounded-lg text-oklavier-blue/60 hover:text-oklavier-blue hover:bg-oklavier-blue/10 transition-colors"
                        title={t("admin.guest_links.copy_link")}
                      >
                        {copiedToken === link.token ? <ExternalLink className="size-4" /> : <Copy className="size-4" />}
                      </button>
                      <button
                        onClick={() => setDeleteId(link.id)}
                        className="p-2 rounded-lg text-red-400/60 hover:text-red-400 hover:bg-red-400/10 transition-colors"
                        title={t("common.delete")}
                      >
                        <Trash2 className="size-4" />
                      </button>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>

          {totalPages > 1 && (
            <div className="flex items-center justify-between px-4 py-3 border-t border-white/10">
              <p className="text-white/30 text-xs">{total} {t("common.results")}</p>
              <div className="flex gap-1">
                <Button variant="ghost" size="sm" onClick={() => setPage(Math.max(1, page - 1))} disabled={page <= 1} className="text-white/40 hover:text-white">
                  <ChevronLeft className="size-4" />
                </Button>
                <Button variant="ghost" size="sm" onClick={() => setPage(Math.min(totalPages, page + 1))} disabled={page >= totalPages} className="text-white/40 hover:text-white">
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
