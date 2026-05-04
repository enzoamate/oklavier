"use client";

import { useState } from "react";
import { ColumnDef } from "@tanstack/react-table";
import { useTranslation } from "@/lib/i18n";
import { Loader2, Shield, ArrowUpDown, FileDown } from "lucide-react";
import { DataTable } from "@/components/data-table";
import { DarkSelect } from "@/components/dark-select";
import { useAPI } from "@/lib/api";
import { authFetch } from "@/lib/auth-fetch";

interface AuditEntry {
  id: string;
  user_id: string;
  user_email: string;
  action: string;
  resource_type: string;
  resource_id: string;
  details: string;
  ip_address: string;
  created_at: string;
}

const actionColors: Record<string, string> = {
  create: "bg-emerald-500/20 text-emerald-400",
  update: "bg-blue-500/20 text-blue-400",
  delete: "bg-red-500/20 text-red-400",
  destroy: "bg-red-500/20 text-red-400",
  toggle: "bg-yellow-500/20 text-yellow-400",
};

export default function AuditPage() {
  const { t } = useTranslation();
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState("");
  const [resourceFilter, setResourceFilter] = useState("");
  const [actionFilter, setActionFilter] = useState("");
  const [exporting, setExporting] = useState(false);
  const perPage = 25;

  const handleExport = async () => {
    setExporting(true);
    try {
      const res = await authFetch("/api/admin/audit/export", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ resource_type: resourceFilter, action: actionFilter }),
      });
      const blob = await res.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = "audit-log.csv";
      document.body.appendChild(a);
      a.click();
      a.remove();
      URL.revokeObjectURL(url);
    } finally {
      setExporting(false);
    }
  };

  const apiUrl = `/api/admin/audit?page=${page}&per_page=${perPage}&search=${encodeURIComponent(search)}&resource_type=${encodeURIComponent(resourceFilter)}&action=${encodeURIComponent(actionFilter)}`;
  const { data, isLoading } = useAPI<{ entries: AuditEntry[]; total: number }>(apiUrl, { revalidateOnFocus: false });

  const entries: AuditEntry[] = data?.entries || [];
  const total = data?.total || 0;
  const totalPages = Math.ceil(total / perPage);

  const columns: ColumnDef<AuditEntry>[] = [
    {
      accessorKey: "created_at",
      header: ({ column }) => (
        <button className="flex items-center gap-1" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
          {t("admin.audit.date")}
          <ArrowUpDown className="size-3" />
        </button>
      ),
      cell: ({ row }) => (
        <span className="text-white/60 text-xs whitespace-nowrap">
          {new Date(row.original.created_at).toLocaleString()}
        </span>
      ),
    },
    {
      accessorKey: "user_email",
      header: ({ column }) => (
        <button className="flex items-center gap-1" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
          {t("admin.audit.user")}
          <ArrowUpDown className="size-3" />
        </button>
      ),
      cell: ({ row }) => (
        <span className="text-white/80 text-xs">{row.original.user_email || row.original.user_id || "-"}</span>
      ),
    },
    {
      accessorKey: "action",
      header: t("admin.audit.action"),
      cell: ({ row }) => (
        <span className={`px-2 py-0.5 rounded-full text-xs font-medium ${actionColors[row.original.action] || "bg-white/10 text-white/60"}`}>
          {row.original.action}
        </span>
      ),
    },
    {
      accessorKey: "resource_type",
      header: t("admin.audit.resource"),
      cell: ({ row }) => (
        <div>
          <span className="text-white/60 text-xs">{row.original.resource_type}</span>
          {row.original.resource_id && (
            <span className="text-white/30 text-xs ml-1">#{row.original.resource_id.slice(0, 8)}</span>
          )}
        </div>
      ),
    },
    {
      accessorKey: "details",
      header: t("admin.audit.details"),
      cell: ({ row }) => (
        <span className="text-white/50 text-xs max-w-[200px] truncate block">{row.original.details || "-"}</span>
      ),
    },
    {
      accessorKey: "ip_address",
      header: "IP",
      cell: ({ row }) => (
        <span className="text-white/30 text-xs font-mono">{row.original.ip_address}</span>
      ),
    },
  ];

  if (isLoading && !data) {
    return <div className="flex items-center gap-2 text-white/60"><Loader2 className="size-5 animate-spin" /> {t("common.loading")}</div>;
  }

  return (
    <div>
      {/* Header */}
      <div className="flex items-center justify-between mb-4">
        <div>
          <h1 className="text-xl font-bold text-white flex items-center gap-2">
            <Shield className="size-5 text-oklavier-blue" />
            {t("admin.audit.title")}
          </h1>
          <p className="text-white/40 text-sm">{t("admin.audit.subtitle", { count: total })}</p>
        </div>
        <button
          onClick={handleExport}
          disabled={exporting}
          className="flex items-center gap-2 px-3 py-2 bg-white/5 hover:bg-white/10 border border-white/10 rounded-lg text-sm text-white/80 transition-colors disabled:opacity-50"
        >
          {exporting ? <Loader2 className="size-4 animate-spin" /> : <FileDown className="size-4" />}
          {t("admin.audit.export_csv")}
        </button>
      </div>

      <DataTable
        columns={columns}
        data={entries}
        manualPagination
        page={page}
        pageCount={totalPages}
        total={total}
        onPageChange={setPage}
        onSearchChange={(s) => { setSearch(s); setPage(1); }}
        emptyMessage={t("admin.audit.no_entries")}
        toolbar={
          <div className="flex items-center gap-2">
            <DarkSelect
              value={resourceFilter}
              onChange={(v) => { setResourceFilter(v); setPage(1); }}
              options={[
                { value: "", label: t("admin.audit.all_resources") },
                { value: "workspace", label: "Workspace" },
                { value: "session", label: "Session" },
                { value: "user", label: "User" },
                { value: "group", label: "Group" },
                { value: "agent", label: "Agent" },
                { value: "auth_method", label: "Auth Method" },
              ]}
            />
            <DarkSelect
              value={actionFilter}
              onChange={(v) => { setActionFilter(v); setPage(1); }}
              options={[
                { value: "", label: t("admin.audit.all_actions") },
                { value: "create", label: t("admin.audit.action_create") },
                { value: "update", label: t("admin.audit.action_update") },
                { value: "delete", label: t("admin.audit.action_delete") },
                { value: "toggle", label: t("admin.audit.action_toggle") },
                { value: "destroy", label: t("admin.audit.action_destroy") },
              ]}
            />
          </div>
        }
      />
    </div>
  );
}
