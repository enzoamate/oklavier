"use client";

import { useState } from "react";
import { Trash2, Eye, RefreshCw, ArrowUpDown, Monitor, X } from "lucide-react";
import { DarkCheckbox } from "@/components/dark-checkbox";
import { Button } from "@/components/ui/button";
import { useTranslation } from "@/lib/i18n";
import { ConfirmModal } from "@/components/confirm-modal";
import { useToast } from "@/components/toast";
import { useAPI, apiAction } from "@/lib/api";
import { DataTable } from "@/components/data-table";
import { ColumnDef } from "@tanstack/react-table";

interface Session {
  id: string;
  user_id: string;
  user_name?: string;
  user_email?: string;
  image_name: string;
  image_src: string;
  pod_name: string;
  container_ip: string;
  status: string;
  created_at: string;
  expires_at: string;
  session_type?: string;
  agent_id?: string;
}

function timeAgo(date: string) {
  const diff = Date.now() - new Date(date).getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 60) return `${mins}m`;
  return `${Math.floor(mins / 60)}h${mins % 60}m`;
}

export default function AdminSessionsPage() {
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState("");
  const perPage = 25;

  const apiUrl = `/api/admin/sessions?page=${page}&per_page=${perPage}&search=${encodeURIComponent(search)}`;
  const { data, isLoading, mutate } = useAPI<{ sessions: Session[]; total: number }>(apiUrl, { refreshInterval: 5000 });
  const sessions = data?.sessions || [];
  const total = data?.total || 0;
  const totalPages = Math.ceil(total / perPage);
  const [deleteTarget, setDeleteTarget] = useState<string | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [shadowing, setShadowing] = useState<string | null>(null);
  const [selected, setSelected] = useState<string[]>([]);
  const [bulkConfirm, setBulkConfirm] = useState(false);
  const [bulkDeleting, setBulkDeleting] = useState(false);
  const { t } = useTranslation();
  const toast = useToast();

  function toggleSelect(id: string) {
    setSelected((prev) =>
      prev.includes(id) ? prev.filter((s) => s !== id) : [...prev, id]
    );
  }

  function toggleSelectAll() {
    if (selected.length === sessions.length) {
      setSelected([]);
    } else {
      setSelected(sessions.map((s) => s.id));
    }
  }

  function handleDestroy(id: string) {
    setDeleteTarget(id);
  }

  async function handleShadow(id: string) {
    setShadowing(id);
    const res = await apiAction("/api/admin/sessions/shadow", "POST", { session_id: id });
    if (res.ok && res.data?.agent_url && res.data?.shadow_id && res.data?.session_token) {
      const viewerUrl = `https://${res.data.agent_url}/sessions/${res.data.shadow_id}#token=${res.data.session_token}`;
      window.open(viewerUrl, "_blank");
    } else {
      toast.error(t("admin.sessions.shadow_error"));
    }
    setShadowing(null);
  }

  async function confirmDestroy() {
    if (!deleteTarget) return;
    setDeleting(true);
    const res = await apiAction("/api/sessions", "POST", { action: "destroy", session_id: deleteTarget });
    if (res.ok) {
      toast.success(t("admin.sessions.destroyed"));
    } else {
      toast.error(t("common.error"));
    }
    mutate();
    setDeleting(false);
    setDeleteTarget(null);
  }

  async function handleBulkDestroy() {
    setBulkDeleting(true);
    const res = await apiAction("/api/admin/sessions/bulk-destroy", "POST", { session_ids: selected });
    if (res.ok) {
      toast.success(t("admin.sessions.bulk_destroyed", { count: res.data?.destroyed || selected.length }));
      setSelected([]);
    } else {
      toast.error(t("common.error"));
    }
    mutate();
    setBulkDeleting(false);
    setBulkConfirm(false);
  }

  const columns: ColumnDef<Session>[] = [
    {
      id: "select",
      header: () => (
        <DarkCheckbox
          checked={sessions.length > 0 && selected.length === sessions.length}
          indeterminate={selected.length > 0 && selected.length < sessions.length}
          onChange={toggleSelectAll}
          title={t("common.select_all")}
        />
      ),
      cell: ({ row }) => (
        <DarkCheckbox
          checked={selected.includes(row.original.id)}
          onChange={() => toggleSelect(row.original.id)}
        />
      ),
      enableSorting: false,
    },
    {
      accessorKey: "user_name",
      header: ({ column }) => (
        <button className="flex items-center gap-1" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
          {t("admin.users.user")}
          <ArrowUpDown className="size-3" />
        </button>
      ),
      cell: ({ row }) => (
        <div>
          <p className="text-white text-sm">{row.original.user_name || "—"}</p>
          <p className="text-white/40 text-xs">{row.original.user_email}</p>
        </div>
      ),
    },
    {
      accessorKey: "image_name",
      header: ({ column }) => (
        <button className="flex items-center gap-1" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
          {t("admin.sessions.workspace")}
          <ArrowUpDown className="size-3" />
        </button>
      ),
      cell: ({ row }) => (
        <div className="flex items-center gap-3">
          <img src={row.original.image_src?.startsWith("http") ? row.original.image_src : `/api/proxy-img/${row.original.image_src}`} alt="" className="size-8 rounded" />
          <span className="text-white text-sm">{row.original.image_name}</span>
        </div>
      ),
    },
    {
      accessorKey: "pod_name",
      header: t("admin.sessions.pod"),
      cell: ({ row }) => <code className="text-white/50 text-xs">{row.original.pod_name}</code>,
    },
    {
      accessorKey: "container_ip",
      header: t("admin.sessions.ip"),
      cell: ({ row }) => <span className="text-white/50 text-sm">{row.original.container_ip}</span>,
    },
    {
      accessorKey: "status",
      header: ({ column }) => (
        <button className="flex items-center gap-1" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
          {t("admin.sessions.status")}
          <ArrowUpDown className="size-3" />
        </button>
      ),
      cell: ({ row }) => {
        const s = row.original;
        return (
          <span className={`inline-flex items-center gap-1.5 text-xs px-2 py-1 rounded-full ${
            s.status === "running" ? "bg-green-500/10 text-green-400" :
            s.status === "starting" ? "bg-yellow-500/10 text-yellow-400" :
            "bg-red-500/10 text-red-400"
          }`}>
            <span className={`size-1.5 rounded-full ${
              s.status === "running" ? "bg-green-500" :
              s.status === "starting" ? "bg-yellow-500" :
              "bg-red-500"
            }`} />
            {s.status}
          </span>
        );
      },
    },
    {
      accessorKey: "created_at",
      header: ({ column }) => (
        <button className="flex items-center gap-1" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
          {t("admin.sessions.duration")}
          <ArrowUpDown className="size-3" />
        </button>
      ),
      cell: ({ row }) => <span className="text-white/50 text-sm">{timeAgo(row.original.created_at)}</span>,
    },
    {
      id: "actions",
      header: () => <span className="sr-only">{t("common.actions")}</span>,
      cell: ({ row }) => (
        <div className="flex items-center justify-end gap-1">
          {row.original.status === "running" && row.original.agent_id && (
            <button
              onClick={() => handleShadow(row.original.id)}
              disabled={shadowing === row.original.id}
              className="p-2 rounded-lg text-white/40 hover:text-blue-400 hover:bg-blue-500/10 transition-colors disabled:opacity-50"
              title={t("admin.sessions.shadow")}
            >
              <Monitor className={`size-4 ${shadowing === row.original.id ? "animate-pulse" : ""}`} />
            </button>
          )}
          <a href={`/sessions/${row.original.id}`} className="p-2 rounded-lg text-white/40 hover:text-white hover:bg-white/10 transition-colors">
            <Eye className="size-4" />
          </a>
          <button onClick={() => handleDestroy(row.original.id)} className="p-2 rounded-lg text-red-400/40 hover:text-red-400 hover:bg-red-500/10 transition-colors">
            <Trash2 className="size-4" />
          </button>
        </div>
      ),
    },
  ];

  return (
    <div>
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-white">{t("admin.sessions.title")}</h1>
        <p className="text-white/50 text-sm">{t("admin.sessions.subtitle", { count: total })}</p>
      </div>

      <DataTable
        columns={columns}
        data={sessions}
        manualPagination
        page={page}
        pageCount={totalPages}
        total={total}
        onPageChange={setPage}
        onSearchChange={(s) => { setSearch(s); setPage(1); }}
        emptyMessage={t("admin.sessions.no_sessions")}
        toolbar={
          <Button onClick={() => mutate()} className="bg-oklavier-blue hover:bg-oklavier-purple">
            <RefreshCw className={`size-4 mr-2 ${isLoading ? "animate-spin" : ""}`} /> {t("common.refresh")}
          </Button>
        }
      />

      {/* Floating bulk action bar */}
      {selected.length > 0 && (
        <div className="fixed bottom-6 left-1/2 -translate-x-1/2 bg-[#1a1f36]/90 backdrop-blur-xl border border-white/10 rounded-xl px-6 py-3 flex items-center gap-4 shadow-2xl z-50">
          <span className="text-white/60 text-sm">{selected.length} {t("common.selected")}</span>
          <button
            onClick={() => setBulkConfirm(true)}
            className="px-4 py-2 bg-red-500/20 text-red-400 rounded-lg text-sm font-medium hover:bg-red-500/30 transition-colors"
          >
            {t("admin.sessions.destroy_all")}
          </button>
          <button
            onClick={() => setSelected([])}
            className="text-white/40 text-sm hover:text-white transition-colors flex items-center gap-1"
          >
            <X className="size-3" />
            {t("common.deselect")}
          </button>
        </div>
      )}

      {/* Single destroy modal */}
      <ConfirmModal
        open={!!deleteTarget}
        title={t("admin.sessions.confirm_destroy")}
        message={t("admin.sessions.confirm_destroy")}
        confirmLabel={t("common.delete")}
        variant="danger"
        loading={deleting}
        onConfirm={confirmDestroy}
        onCancel={() => setDeleteTarget(null)}
      />

      {/* Bulk destroy modal */}
      <ConfirmModal
        open={bulkConfirm}
        title={t("admin.sessions.confirm_bulk_destroy", { count: selected.length })}
        message={t("admin.sessions.confirm_bulk_destroy", { count: selected.length })}
        confirmLabel={t("admin.sessions.destroy_all")}
        variant="danger"
        loading={bulkDeleting}
        onConfirm={handleBulkDestroy}
        onCancel={() => setBulkConfirm(false)}
      />
    </div>
  );
}
