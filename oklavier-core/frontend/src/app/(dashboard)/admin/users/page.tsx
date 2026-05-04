"use client";

import { useState } from "react";
import { Plus, Trash2, Loader2, Shield, ShieldOff, Ban, KeyRound, Globe, ArrowUpDown, RotateCcw, X } from "lucide-react";
import { DarkCheckbox } from "@/components/dark-checkbox";
import { DarkSelect } from "@/components/dark-select";
import { ColumnDef } from "@tanstack/react-table";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { DataTable } from "@/components/data-table";
import { useTranslation } from "@/lib/i18n";
import { ConfirmModal } from "@/components/confirm-modal";
import { useToast } from "@/components/toast";
import { useAPI, apiAction } from "@/lib/api";

interface User {
  id: string;
  name: string;
  email: string;
  role: string;
  banned: boolean;
  createdAt: string;
  auth_provider: string;
  oidc_provider_name: string;
  oidc_roles: string[];
  groups: string[];
}

export default function AdminUsersPage() {
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState("");
  const perPage = 25;

  const apiUrl = `/api/admin/users?page=${page}&per_page=${perPage}&search=${encodeURIComponent(search)}`;
  const { data, isLoading, mutate } = useAPI<{ users: User[]; total: number }>(apiUrl);
  const users = data?.users || [];
  const total = data?.total || 0;
  const totalPages = Math.ceil(total / perPage);
  const [showForm, setShowForm] = useState(false);
  const [form, setForm] = useState({ name: "", email: "", password: "", role: "user" });
  const [deleteTarget, setDeleteTarget] = useState<string | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [resetTarget, setResetTarget] = useState<string | null>(null);
  const [resetPassword, setResetPassword] = useState("");
  const [selected, setSelected] = useState<string[]>([]);
  const [bulkDeleteConfirm, setBulkDeleteConfirm] = useState(false);
  const [bulkBanConfirm, setBulkBanConfirm] = useState(false);
  const [bulkDeleting, setBulkDeleting] = useState(false);
  const [bulkUpdating, setBulkUpdating] = useState(false);
  const { t } = useTranslation();
  const toast = useToast();

  function toggleSelect(id: string) {
    setSelected((prev) =>
      prev.includes(id) ? prev.filter((s) => s !== id) : [...prev, id]
    );
  }

  function toggleSelectAll() {
    if (selected.length === users.length) {
      setSelected([]);
    } else {
      setSelected(users.map((u) => u.id));
    }
  }

  async function handleResetPassword() {
    if (!resetTarget || !resetPassword) return;
    const res = await apiAction("/api/admin/users", "PATCH", { id: resetTarget, action: "reset-password", new_password: resetPassword });
    if (res.ok) {
      toast.success(t("admin.users.password_reset_success"));
    } else {
      toast.error(res.data?.error || t("common.error"));
    }
    mutate();
    setResetTarget(null);
    setResetPassword("");
  }

  async function handleCreate() {
    const res = await apiAction("/api/admin/users", "POST", form);
    if (res.ok) {
      toast.success(t("admin.users.created_success"));
    } else {
      toast.error(t("common.error"));
    }
    setShowForm(false);
    setForm({ name: "", email: "", password: "", role: "user" });
    mutate();
  }

  function handleDelete(id: string) {
    setDeleteTarget(id);
  }

  async function confirmDelete() {
    if (!deleteTarget) return;
    setDeleting(true);
    const res = await apiAction("/api/admin/users", "DELETE", { id: deleteTarget });
    if (res.ok) {
      toast.success(t("admin.users.deleted"));
    } else {
      toast.error(t("common.error"));
    }
    mutate();
    setDeleting(false);
    setDeleteTarget(null);
  }

  async function handleToggleRole(id: string, currentRole: string) {
    const newRole = currentRole === "admin" ? "user" : "admin";
    const res = await apiAction("/api/admin/users", "PATCH", { id, role: newRole });
    if (res.ok) {
      toast.success(t("admin.users.role_updated"));
    } else {
      toast.error(t("common.error"));
    }
    mutate();
  }

  async function handleToggleBan(id: string, banned: boolean) {
    const res = await apiAction("/api/admin/users", "PATCH", { id, banned: !banned });
    if (res.ok) {
      toast.success(t("admin.users.ban_updated"));
    } else {
      toast.error(t("common.error"));
    }
    mutate();
  }

  async function handleBulkDelete() {
    setBulkDeleting(true);
    const res = await apiAction("/api/admin/users/bulk-delete", "POST", { user_ids: selected });
    if (res.ok) {
      toast.success(t("admin.users.bulk_deleted", { count: res.data?.deleted || selected.length }));
      setSelected([]);
    } else {
      toast.error(t("common.error"));
    }
    mutate();
    setBulkDeleting(false);
    setBulkDeleteConfirm(false);
  }

  async function handleBulkBan() {
    setBulkUpdating(true);
    const res = await apiAction("/api/admin/users/bulk-update", "POST", { user_ids: selected, banned: true });
    if (res.ok) {
      toast.success(t("admin.users.ban_updated"));
      setSelected([]);
    } else {
      toast.error(t("common.error"));
    }
    mutate();
    setBulkUpdating(false);
    setBulkBanConfirm(false);
  }

  async function handleBulkRoleChange(role: string) {
    const res = await apiAction("/api/admin/users/bulk-update", "POST", { user_ids: selected, role });
    if (res.ok) {
      toast.success(t("admin.users.role_updated"));
      setSelected([]);
    } else {
      toast.error(t("common.error"));
    }
    mutate();
  }

  const columns: ColumnDef<User>[] = [
    {
      id: "select",
      header: () => (
        <DarkCheckbox
          checked={users.length > 0 && selected.length === users.length}
          indeterminate={selected.length > 0 && selected.length < users.length}
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
      accessorKey: "name",
      header: ({ column }) => (
        <button className="flex items-center gap-1" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
          {t("admin.users.user")}
          <ArrowUpDown className="size-3" />
        </button>
      ),
      cell: ({ row }) => (
        <div>
          <p className="text-white font-medium">{row.original.name}</p>
          <p className="text-white/40 text-xs">{row.original.email}</p>
        </div>
      ),
    },
    {
      id: "auth",
      header: t("admin.users.auth"),
      cell: ({ row }) => (
        <span className={`inline-flex items-center gap-1.5 text-[10px] px-2 py-1 rounded-full ${
          row.original.auth_provider === "oidc" ? "bg-oklavier-blue/10 text-oklavier-blue" : "bg-white/5 text-white/40"
        }`}>
          {row.original.auth_provider === "oidc" ? <Globe className="size-3" /> : <KeyRound className="size-3" />}
          {row.original.auth_provider === "oidc" ? (row.original.oidc_provider_name || t("admin.users.sso")) : t("admin.users.local")}
        </span>
      ),
    },
    {
      accessorKey: "role",
      header: t("admin.users.role"),
      cell: ({ row }) => (
        <span className={`inline-flex items-center gap-1.5 text-xs px-2 py-1 rounded-full ${
          row.original.role === "admin" ? "bg-oklavier-blue/10 text-oklavier-blue" : "bg-white/5 text-white/50"
        }`}>
          {row.original.role === "admin" ? <Shield className="size-3" /> : null}
          {row.original.role}
        </span>
      ),
    },
    {
      id: "groups_roles",
      header: t("admin.users.groups_roles"),
      cell: ({ row }) => (
        <div className="flex flex-wrap gap-1 max-w-xs">
          {row.original.groups?.map((g) => (
            <span key={g} className="text-[10px] bg-green-500/10 text-green-400 px-1.5 py-0.5 rounded">{g}</span>
          ))}
          {row.original.oidc_roles?.slice(0, 3).map((r) => (
            <span key={r} className="text-[10px] bg-orange-400/10 text-orange-400 px-1.5 py-0.5 rounded font-mono">{r}</span>
          ))}
          {row.original.oidc_roles && row.original.oidc_roles.length > 3 && (
            <span className="text-[10px] text-white/30">+{row.original.oidc_roles.length - 3}</span>
          )}
        </div>
      ),
    },
    {
      id: "status",
      header: t("admin.users.status"),
      cell: ({ row }) => row.original.banned ? (
        <span className="inline-flex items-center gap-1.5 text-xs px-2 py-1 rounded-full bg-red-500/10 text-red-400">
          <Ban className="size-3" /> {t("admin.users.banned")}
        </span>
      ) : (
        <span className="inline-flex items-center gap-1.5 text-xs px-2 py-1 rounded-full bg-green-500/10 text-green-400">
          {t("common.active")}
        </span>
      ),
    },
    {
      accessorKey: "createdAt",
      header: ({ column }) => (
        <button className="flex items-center gap-1" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
          {t("admin.users.created")}
          <ArrowUpDown className="size-3" />
        </button>
      ),
      cell: ({ row }) => (
        <span className="text-white/50 text-sm">{new Date(row.original.createdAt).toLocaleDateString("fr-FR")}</span>
      ),
    },
    {
      id: "actions",
      cell: ({ row }) => (
        <div className="flex items-center justify-end gap-1">
          <button onClick={() => handleToggleRole(row.original.id, row.original.role)} className="p-2 rounded-lg text-white/40 hover:text-white hover:bg-white/10 transition-colors" title={row.original.role === "admin" ? t("admin.users.demote_admin") : t("admin.users.promote_admin")}>
            {row.original.role === "admin" ? <ShieldOff className="size-4" /> : <Shield className="size-4" />}
          </button>
          <button onClick={() => { setResetTarget(row.original.id); setResetPassword(""); }} className="p-2 rounded-lg text-blue-400/40 hover:text-blue-400 hover:bg-blue-500/10 transition-colors" title={t("admin.users.reset_password")}>
            <RotateCcw className="size-4" />
          </button>
          <button onClick={() => handleToggleBan(row.original.id, row.original.banned)} className="p-2 rounded-lg text-yellow-400/40 hover:text-yellow-400 hover:bg-yellow-500/10 transition-colors" title={row.original.banned ? t("admin.users.unban") : t("admin.users.ban")}>
            <Ban className="size-4" />
          </button>
          <button onClick={() => handleDelete(row.original.id)} className="p-2 rounded-lg text-red-400/40 hover:text-red-400 hover:bg-red-500/10 transition-colors">
            <Trash2 className="size-4" />
          </button>
        </div>
      ),
    },
  ];

  // Only show full-page loader on initial load (no data yet)
  if (!data && isLoading) return <div className="flex items-center gap-2 text-white/60"><Loader2 className="size-5 animate-spin" /> {t("common.loading")}</div>;

  return (
    <div>
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-white">{t("admin.users.title")}</h1>
        <p className="text-white/50 text-sm">{t("admin.users.subtitle", { count: total })}</p>
      </div>

      <DataTable
        columns={columns}
        data={users}
        searchPlaceholder={t("common.search")}
        manualPagination
        page={page}
        pageCount={totalPages}
        total={total}
        onPageChange={setPage}
        onSearchChange={(s) => { setSearch(s); setPage(1); }}
        toolbar={
          <Button onClick={() => setShowForm(true)} className="bg-oklavier-blue hover:bg-oklavier-purple">
            <Plus className="size-4 mr-2" /> {t("admin.users.add")}
          </Button>
        }
      />

      {/* Floating bulk action bar */}
      {selected.length > 0 && (
        <div className="fixed bottom-6 left-1/2 -translate-x-1/2 bg-[#1a1f36]/90 backdrop-blur-xl border border-white/10 rounded-xl px-6 py-3 flex items-center gap-4 shadow-2xl z-50">
          <span className="text-white/60 text-sm">{selected.length} {t("common.selected")}</span>
          <button
            onClick={() => setBulkBanConfirm(true)}
            className="px-4 py-2 bg-yellow-500/20 text-yellow-400 rounded-lg text-sm font-medium hover:bg-yellow-500/30 transition-colors"
          >
            {t("admin.users.ban_selected")}
          </button>
          <DarkSelect
            value=""
            placeholder={t("admin.users.role")}
            dropUp
            options={[
              { value: "user", label: t("admin.users.user_role") },
              { value: "admin", label: t("admin.users.admin") },
            ]}
            onChange={(v) => handleBulkRoleChange(v)}
          />
          <button
            onClick={() => setBulkDeleteConfirm(true)}
            className="px-4 py-2 bg-red-500/20 text-red-400 rounded-lg text-sm font-medium hover:bg-red-500/30 transition-colors"
          >
            {t("admin.users.delete_selected")}
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

      {/* Single delete modal */}
      <ConfirmModal
        open={!!deleteTarget}
        title={t("admin.users.confirm_delete")}
        message={t("admin.users.confirm_delete")}
        variant="danger"
        loading={deleting}
        onConfirm={confirmDelete}
        onCancel={() => setDeleteTarget(null)}
      />

      {/* Bulk delete modal */}
      <ConfirmModal
        open={bulkDeleteConfirm}
        title={t("admin.users.confirm_bulk_delete", { count: selected.length })}
        message={t("admin.users.confirm_bulk_delete", { count: selected.length })}
        confirmLabel={t("admin.users.delete_selected")}
        variant="danger"
        loading={bulkDeleting}
        onConfirm={handleBulkDelete}
        onCancel={() => setBulkDeleteConfirm(false)}
      />

      {/* Bulk ban modal */}
      <ConfirmModal
        open={bulkBanConfirm}
        title={t("admin.users.confirm_bulk_ban", { count: selected.length })}
        message={t("admin.users.confirm_bulk_ban", { count: selected.length })}
        confirmLabel={t("admin.users.ban_selected")}
        variant="danger"
        loading={bulkUpdating}
        onConfirm={handleBulkBan}
        onCancel={() => setBulkBanConfirm(false)}
      />

      {/* Reset Password Modal */}
      {resetTarget && (
        <div className="fixed inset-0 z-50 flex items-center justify-center">
          <div className="absolute inset-0 bg-black/50 backdrop-blur-sm" onClick={() => setResetTarget(null)} />
          <div className="relative z-50 w-full max-w-sm bg-[#1a1f36] border border-white/10 rounded-2xl p-6 shadow-2xl">
            <h2 className="text-white text-lg font-semibold mb-4">{t("admin.users.reset_password")}</h2>
            <div>
              <label className="text-white/60 text-sm mb-1 block">{t("admin.users.new_password")}</label>
              <Input type="password" value={resetPassword} onChange={(e) => setResetPassword(e.target.value)} placeholder="Min. 8 chars, A-z, 0-9" className="bg-[#2e3862]/80 border-white/10 text-white" />
            </div>
            <p className="text-white/30 text-xs mt-2">{t("admin.users.reset_password_warning")}</p>
            <div className="flex gap-3 mt-5">
              <Button onClick={() => setResetTarget(null)} variant="outline" className="flex-1">{t("common.cancel")}</Button>
              <Button onClick={handleResetPassword} disabled={!resetPassword || resetPassword.length < 8} className="flex-1 bg-oklavier-blue hover:bg-oklavier-purple">{t("common.confirm")}</Button>
            </div>
          </div>
        </div>
      )}

      {/* Create User Modal */}
      {showForm && (
        <div className="fixed inset-0 z-50 flex items-center justify-center">
          <div className="absolute inset-0 bg-black/50 backdrop-blur-sm" onClick={() => setShowForm(false)} />
          <div className="relative z-50 w-full max-w-md bg-[#1a1f36] border border-white/10 rounded-2xl p-6 shadow-2xl">
            <h2 className="text-white text-xl font-semibold mb-6">{t("admin.users.create_user")}</h2>
            <div className="space-y-4">
              <div>
                <label className="text-white/60 text-sm mb-1 block">{t("admin.users.name")}</label>
                <Input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} className="bg-[#2e3862]/80 border-white/10 text-white" />
              </div>
              <div>
                <label className="text-white/60 text-sm mb-1 block">{t("admin.users.email")}</label>
                <Input type="email" value={form.email} onChange={(e) => setForm({ ...form, email: e.target.value })} className="bg-[#2e3862]/80 border-white/10 text-white" />
              </div>
              <div>
                <label className="text-white/60 text-sm mb-1 block">{t("admin.users.password")}</label>
                <Input type="password" value={form.password} onChange={(e) => setForm({ ...form, password: e.target.value })} className="bg-[#2e3862]/80 border-white/10 text-white" />
              </div>
            </div>
            <div className="flex gap-3 mt-6">
              <Button onClick={() => setShowForm(false)} variant="outline" className="flex-1">{t("common.cancel")}</Button>
              <Button onClick={handleCreate} className="flex-1 bg-oklavier-blue hover:bg-oklavier-purple">{t("common.create")}</Button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
