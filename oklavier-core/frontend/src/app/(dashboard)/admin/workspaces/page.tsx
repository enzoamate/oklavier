"use client";

import { useState, useEffect } from "react";
import { useSearchParams } from "next/navigation";
import { Plus, Pencil, Trash2, Loader2, Power, PowerOff, ChevronDown, ChevronRight, ArrowUpDown, X } from "lucide-react";
import { ColumnDef } from "@tanstack/react-table";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { DataTable } from "@/components/data-table";
import { useTranslation } from "@/lib/i18n";
import { ConfirmModal } from "@/components/confirm-modal";
import { useToast } from "@/components/toast";
import { useAPI, apiAction, invalidate } from "@/lib/api";
import { DarkSelect } from "@/components/dark-select";

interface Workspace {
  id: string;
  name: string;
  friendly_name: string;
  description: string;
  image_src: string;
  docker_image: string;
  cores: number;
  memory: number;
  category: string;
  enabled: boolean;
  groups?: string[];
  docker_registry: string;
  docker_user: string;
  docker_password: string;
  session_time_limit: number;
  gpu_count: number;
  uncompressed_size_mb: number;
  shm_size: string;
  restrict_to_agent: string;
  restrict_to_region: string;
  run_config: any;
  exec_config: any;
  volume_mappings: any;
  categories: string[];
  notes: string;
  persistent: boolean;
  persistent_size: string;
  workspace_type: string;
  server_hostname: string;
  server_port: number;
  server_protocol: string;
  server_username: string;
  server_password: string;
  server_domain: string;
  server_ignore_cert: boolean;
  server_security: string;
  server_auth_mode: string;
  server_allow_remember: boolean;
  server_default_settings: string;
  record_sessions: boolean;
}

interface Group {
  id: string;
  name: string;
  color: string;
}

interface Agent {
  id: string;
  name: string;
  region: string;
}

function CollapsibleSection({ title, defaultOpen, children }: { title: string; defaultOpen?: boolean; children: React.ReactNode }) {
  const [open, setOpen] = useState(defaultOpen ?? false);
  return (
    <div className="border border-white/10 rounded-lg overflow-hidden">
      <button
        type="button"
        onClick={() => setOpen(!open)}
        className="w-full flex items-center justify-between px-4 py-3 bg-white/5 hover:bg-white/10 transition-colors text-white text-sm font-medium"
      >
        {title}
        {open ? <ChevronDown className="size-4 text-white/40" /> : <ChevronRight className="size-4 text-white/40" />}
      </button>
      {open && <div className="p-4 space-y-4">{children}</div>}
    </div>
  );
}

const inputClass = "bg-[#2e3862]/80 border-white/10 text-white";
const labelClass = "text-white/60 text-sm mb-1 block";
const textareaClass = "w-full bg-[#2e3862]/80 border border-white/10 text-white rounded-md px-3 py-2 text-sm font-mono min-h-[80px] focus:outline-none focus:ring-2 focus:ring-oklavier-blue/50";

export default function AdminWorkspacesPage() {
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState("");
  const perPage = 25;

  const apiUrl = `/api/admin/workspaces?page=${page}&per_page=${perPage}&search=${encodeURIComponent(search)}`;
  const { data: wData, mutate } = useAPI<{ workspaces: Workspace[]; total: number }>(apiUrl);
  const { data: gData } = useAPI<{ groups: Group[] }>("/api/admin/groups");
  const { data: aData } = useAPI<{ agents: Agent[] }>("/api/admin/agents");
  const workspaces = (wData?.workspaces || []).map(w => ({ ...w, groups: w.groups || [] }));
  const total = wData?.total || 0;
  const totalPages = Math.ceil(total / perPage);
  const groups = gData?.groups || [];
  const agents = aData?.agents || [];
  const initialLoading = !wData && !gData && !aData;
  const [showForm, setShowForm] = useState(false);
  const [editing, setEditing] = useState<Workspace | null>(null);
  const [selectedGroups, setSelectedGroups] = useState<string[]>([]);
  const [form, setForm] = useState({
    name: "", friendly_name: "", description: "", image_src: "",
    docker_image: "", cores: "2", memory: "2768", category: "Desktop", enabled: true,
    docker_registry: "", docker_user: "", docker_password: "",
    session_time_limit: "0", gpu_count: "0", uncompressed_size_mb: "0", shm_size: "256m",
    restrict_to_agent: "", restrict_to_region: "",
    run_config: "{}", exec_config: "{}", volume_mappings: "{}",
    categories: "", notes: "",
    persistent: false, persistent_size: "",
    workspace_type: "container",
    server_hostname: "", server_port: "3389", server_protocol: "rdp",
    server_username: "", server_password: "", server_domain: "",
    server_ignore_cert: true, server_security: "any", server_auth_mode: "static",
    server_allow_remember: false, server_default_settings: "{}",
    record_sessions: false,
  });
  const [deleteTarget, setDeleteTarget] = useState<string | null>(null);
  const [deleting, setDeleting] = useState(false);
  const { t } = useTranslation();
  const toast = useToast();

  const searchParams = useSearchParams();

  useEffect(() => {
    // Check if coming from registry install
    const installParam = searchParams.get("install");
    if (installParam) {
      const params = new URLSearchParams(installParam);
      setForm({
        name: params.get("name") || "",
        friendly_name: params.get("friendly_name") || "",
        description: params.get("description") || "",
        image_src: params.get("image_src") || "",
        docker_image: params.get("docker_image") || "",
        cores: params.get("cores") || "2",
        memory: String(Math.round(Number(params.get("memory") || "2768000000") / 1000000)),
        category: params.get("category") || "Desktop",
        enabled: true,
        docker_registry: "https://index.docker.io/v1/",
        docker_user: "",
        docker_password: "",
        session_time_limit: "0",
        gpu_count: "0",
        uncompressed_size_mb: "0",
        shm_size: "512m",
        restrict_to_agent: "",
        restrict_to_region: "",
        run_config: "{}",
        exec_config: "[]",
        volume_mappings: "{}",
        categories: params.get("category") || "",
        notes: "",
        persistent: false,
        persistent_size: "5Gi",
        workspace_type: "container",
        server_hostname: "", server_port: "3389", server_protocol: "rdp",
        server_username: "", server_password: "", server_domain: "",
        server_ignore_cert: true, server_security: "any", server_auth_mode: "static",
        server_allow_remember: false, server_default_settings: "{}",
        record_sessions: false,
      });
      setEditing(null);
      setShowForm(true);
      // Clean URL
      window.history.replaceState({}, "", "/admin/workspaces");
    }
  }, []);

  function jsonStringify(val: any): string {
    if (!val) return "{}";
    if (typeof val === "string") return val;
    try { return JSON.stringify(val, null, 2); } catch { return "{}"; }
  }

  function openNew() {
    setEditing(null);
    setSelectedGroups([]);
    setForm({
      name: "", friendly_name: "", description: "", image_src: "",
      docker_image: "", cores: "2", memory: "2768", category: "Desktop", enabled: true,
      docker_registry: "", docker_user: "", docker_password: "",
      session_time_limit: "0", gpu_count: "0", uncompressed_size_mb: "0", shm_size: "256m",
      restrict_to_agent: "", restrict_to_region: "",
      run_config: "{}", exec_config: "{}", volume_mappings: "{}",
      categories: "", notes: "",
      persistent: false, persistent_size: "",
      workspace_type: "container",
      server_hostname: "", server_port: "3389", server_protocol: "rdp",
      server_username: "", server_password: "", server_domain: "",
      server_ignore_cert: true, server_security: "any", server_auth_mode: "static",
      server_allow_remember: false, server_default_settings: "{}",
      record_sessions: false,
    });
    setShowForm(true);
  }

  function openEdit(w: Workspace) {
    setEditing(w);
    setSelectedGroups(w.groups?.map(gName => groups.find(g => g.name === gName)?.id || "").filter(Boolean) || []);
    setForm({
      name: w.name, friendly_name: w.friendly_name, description: w.description || "",
      image_src: w.image_src, docker_image: w.docker_image,
      cores: String(w.cores), memory: String(Math.round(w.memory / 1000000)),
      category: w.category, enabled: w.enabled,
      docker_registry: w.docker_registry || "", docker_user: w.docker_user || "", docker_password: w.docker_password || "",
      session_time_limit: String(w.session_time_limit || 0),
      gpu_count: String(w.gpu_count || 0),
      uncompressed_size_mb: String(w.uncompressed_size_mb || 0),
      shm_size: w.shm_size || "256m",
      restrict_to_agent: w.restrict_to_agent || "",
      restrict_to_region: w.restrict_to_region || "",
      run_config: jsonStringify(w.run_config),
      exec_config: jsonStringify(w.exec_config),
      volume_mappings: jsonStringify(w.volume_mappings),
      categories: Array.isArray(w.categories) ? w.categories.join("\n") : "",
      notes: w.notes || "",
      persistent: w.persistent || false,
      persistent_size: w.persistent_size || "",
      workspace_type: w.workspace_type || "container",
      server_hostname: w.server_hostname || "",
      server_port: String(w.server_port || (w.server_protocol === "vnc" ? 5900 : 3389)),
      server_protocol: w.server_protocol || "rdp",
      server_username: w.server_username || "",
      server_password: w.server_password || "",
      server_domain: w.server_domain || "",
      server_ignore_cert: w.server_ignore_cert ?? true,
      server_security: w.server_security || "any",
      server_auth_mode: w.server_auth_mode || "static",
      server_allow_remember: w.server_allow_remember || false,
      server_default_settings: jsonStringify(w.server_default_settings),
      record_sessions: w.record_sessions || false,
    });
    setShowForm(true);
  }

  async function handleSave() {
    const body: any = {
      ...form,
      cores: parseFloat(form.cores),
      memory: parseInt(form.memory) * 1000000,
      session_time_limit: parseInt(form.session_time_limit) || 0,
      gpu_count: parseInt(form.gpu_count) || 0,
      uncompressed_size_mb: parseInt(form.uncompressed_size_mb) || 0,
      categories: form.categories.split("\n").map(s => s.trim()).filter(Boolean),
      id: editing?.id,
      server_port: parseInt(form.server_port) || 0,
      docker_image: form.workspace_type === "server" ? (form.docker_image || "server") : form.docker_image,
    };
    const res = await apiAction("/api/admin/workspaces", editing ? "PUT" : "POST", body);

    if (!res.ok) {
      toast.error(t("common.error"));
      return;
    }

    // Save group assignments
    const workspaceId = editing?.id || body.id;
    if (workspaceId && selectedGroups.length > 0) {
      const groupRes = await apiAction("/api/admin/groups", "PUT", { type: "workspace", workspace_id: workspaceId, group_ids: selectedGroups });
      if (!groupRes.ok) {
        toast.error(t("common.error"));
        return;
      }
      toast.success(t("admin.workspaces.groups_saved"));
    }

    toast.success(editing ? t("admin.workspaces.updated") : t("admin.workspaces.created"));
    setShowForm(false);
    mutate();
    invalidate("/api/admin/groups");
  }

  function handleDelete(id: string) {
    setDeleteTarget(id);
  }

  async function confirmDelete() {
    if (!deleteTarget) return;
    setDeleting(true);
    const res = await apiAction("/api/admin/workspaces", "DELETE", { id: deleteTarget });
    if (res.ok) {
      toast.success(t("admin.workspaces.deleted"));
    } else {
      toast.error(t("common.error"));
    }
    mutate();
    setDeleting(false);
    setDeleteTarget(null);
  }

  async function handleToggle(id: string, enabled: boolean) {
    const res = await apiAction("/api/admin/workspaces", "PATCH", { id, enabled: !enabled });
    if (res.ok) {
      toast.success(t("admin.workspaces.toggled"));
    } else {
      toast.error(t("common.error"));
    }
    mutate();
  }

  const columns: ColumnDef<Workspace>[] = [
    {
      accessorKey: "friendly_name",
      header: ({ column }) => (
        <button className="flex items-center gap-1" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
          {t("admin.workspaces.image")}
          <ArrowUpDown className="size-3" />
        </button>
      ),
      cell: ({ row }) => (
        <div className="flex items-center gap-3">
          {row.original.image_src && (
            <img
              src={row.original.image_src.startsWith("http") ? row.original.image_src : `/api/proxy-img/${row.original.image_src}`}
              className="size-8 rounded"
              alt=""
            />
          )}
          <div>
            <div className="flex items-center gap-2">
              <p className="text-white font-medium">{row.original.friendly_name}</p>
              {row.original.workspace_type === "server" && (
                <span className="text-[10px] px-1.5 py-0.5 rounded bg-amber-500/20 text-amber-400 font-medium uppercase">Server</span>
              )}
            </div>
            <p className="text-white/30 text-xs">{row.original.name}</p>
          </div>
        </div>
      ),
    },
    {
      accessorKey: "docker_image",
      header: t("admin.workspaces.docker_image"),
      cell: ({ row }) => row.original.workspace_type === "server" ? (
        <code className="text-xs text-amber-400/80">{(row.original.server_protocol || "rdp").toUpperCase()} → {row.original.server_hostname}:{row.original.server_port}</code>
      ) : (
        <code className="text-xs text-oklavier-blue/80">{row.original.docker_image}</code>
      ),
    },
    {
      accessorKey: "category",
      header: t("admin.workspaces.category"),
    },
    {
      id: "groups",
      header: t("admin.workspaces.groups"),
      cell: ({ row }) => (
        <div className="flex gap-1 flex-wrap">
          {row.original.groups && row.original.groups.length > 0 ? row.original.groups.map((gName) => {
            const g = groups.find(gr => gr.name === gName);
            return (
              <span key={gName} className="text-[10px] px-2 py-0.5 rounded-full" style={{ backgroundColor: (g?.color || "#7096ff") + "20", color: g?.color || "#7096ff" }}>
                {gName}
              </span>
            );
          }) : <span className="text-white/20 text-xs">{t("admin.workspaces.all")}</span>}
        </div>
      ),
    },
    {
      id: "resources",
      header: t("admin.workspaces.resources"),
      cell: ({ row }) => (
        <span className="text-white/50 text-xs">{row.original.cores} vCPU / {Math.round(row.original.memory / 1000000)} Mo</span>
      ),
    },
    {
      id: "status",
      header: t("admin.workspaces.status"),
      cell: ({ row }) => (
        <button
          onClick={() => handleToggle(row.original.id, row.original.enabled)}
          className={`inline-flex items-center gap-1.5 text-xs px-2 py-1 rounded-full ${row.original.enabled ? "bg-green-500/10 text-green-400" : "bg-red-500/10 text-red-400"}`}
        >
          {row.original.enabled ? <Power className="size-3" /> : <PowerOff className="size-3" />}
          {row.original.enabled ? t("common.enabled") : t("common.disabled")}
        </button>
      ),
    },
    {
      id: "actions",
      cell: ({ row }) => (
        <div className="flex gap-1">
          <button onClick={() => openEdit(row.original)} className="p-2 rounded-lg text-white/40 hover:text-white hover:bg-white/10 transition-colors">
            <Pencil className="size-4" />
          </button>
          <button onClick={() => handleDelete(row.original.id)} className="p-2 rounded-lg text-red-400/40 hover:text-red-400 hover:bg-red-500/10 transition-colors">
            <Trash2 className="size-4" />
          </button>
        </div>
      ),
    },
  ];

  // Only show full-page loader on initial load (no data yet)
  if (initialLoading) return <div className="flex items-center gap-2 text-white/60"><Loader2 className="size-5 animate-spin" /> {t("common.loading")}</div>;

  return (
    <div>
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-white">{t("admin.workspaces.title")}</h1>
        <p className="text-white/50 text-sm">{t("admin.workspaces.subtitle", { count: total })}</p>
      </div>

      <DataTable
        columns={columns}
        data={workspaces}
        searchPlaceholder={t("common.search")}
        manualPagination
        page={page}
        pageCount={totalPages}
        total={total}
        onPageChange={setPage}
        onSearchChange={(s) => { setSearch(s); setPage(1); }}
        toolbar={
          <Button onClick={openNew} className="bg-oklavier-blue hover:bg-oklavier-purple">
            <Plus className="size-4 mr-2" /> {t("admin.workspaces.add")}
          </Button>
        }
      />

      <ConfirmModal
        open={!!deleteTarget}
        title={t("admin.workspaces.confirm_delete")}
        message={t("admin.workspaces.confirm_delete")}
        variant="danger"
        loading={deleting}
        onConfirm={confirmDelete}
        onCancel={() => setDeleteTarget(null)}
      />

      {/* Add/Edit Modal */}
      {showForm && (
        <div className="fixed inset-0 z-50 flex items-center justify-center">
          <div className="absolute inset-0 bg-black/50 backdrop-blur-sm" onClick={() => setShowForm(false)} />
          <div className="relative z-50 w-full max-w-2xl max-h-[90vh] flex flex-col bg-[#1a1f36] border border-white/10 rounded-2xl shadow-2xl">
            {/* Fixed header */}
            <div className="flex items-center justify-between px-6 py-4 border-b border-white/10">
              <h2 className="text-white text-xl font-semibold">{editing ? t("admin.workspaces.edit_workspace") : t("admin.workspaces.add_workspace")}</h2>
              <button onClick={() => setShowForm(false)} className="p-1.5 rounded-lg text-white/40 hover:text-white hover:bg-white/10 transition-colors">
                <X className="size-5" />
              </button>
            </div>

            {/* Scrollable content */}
            <div className="flex-1 overflow-y-auto px-6 py-4 space-y-3 dark-scroll" style={{ scrollbarWidth: "thin", scrollbarColor: "rgba(255,255,255,0.1) transparent" }}>

            {/* Workspace Type Toggle */}
            <div className="flex gap-2 mb-2">
              <button
                type="button"
                onClick={() => setForm({ ...form, workspace_type: "container" })}
                className={`flex-1 py-2.5 rounded-lg text-sm font-medium transition-all border ${
                  form.workspace_type === "container"
                    ? "bg-oklavier-blue/20 border-oklavier-blue/50 text-oklavier-blue"
                    : "border-white/10 text-white/40 hover:text-white/60 hover:border-white/20"
                }`}
              >
                {t("admin.workspaces.type_container")}
              </button>
              <button
                type="button"
                onClick={() => setForm({ ...form, workspace_type: "server", docker_image: form.docker_image || "server" })}
                className={`flex-1 py-2.5 rounded-lg text-sm font-medium transition-all border ${
                  form.workspace_type === "server"
                    ? "bg-amber-500/20 border-amber-500/50 text-amber-400"
                    : "border-white/10 text-white/40 hover:text-white/60 hover:border-white/20"
                }`}
              >
                {t("admin.workspaces.type_server")}
              </button>
            </div>

              {/* General */}
              <CollapsibleSection title={t("admin.workspaces.general")} defaultOpen>
                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <label className={labelClass}>{t("admin.workspaces.name")}</label>
                    <Input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} placeholder="chrome" className={inputClass} />
                  </div>
                  <div>
                    <label className={labelClass}>{t("admin.workspaces.friendly_name")}</label>
                    <Input value={form.friendly_name} onChange={(e) => setForm({ ...form, friendly_name: e.target.value })} placeholder="Google Chrome" className={inputClass} />
                  </div>
                </div>
                <div>
                  <label className={labelClass}>{t("admin.workspaces.description")}</label>
                  <Input value={form.description} onChange={(e) => setForm({ ...form, description: e.target.value })} placeholder="Description..." className={inputClass} />
                </div>
                <div>
                  <label className={labelClass}>{t("admin.workspaces.icon_url")}</label>
                  <Input value={form.image_src} onChange={(e) => setForm({ ...form, image_src: e.target.value })} placeholder="img/thumbnails/chrome.png" className={inputClass} />
                </div>
                <div>
                  <label className={labelClass}>{t("admin.workspaces.categories_label")} <span className="text-white/30">({t("admin.workspaces.one_per_line")})</span></label>
                  <textarea
                    value={form.categories}
                    onChange={(e) => setForm({ ...form, categories: e.target.value })}
                    placeholder={"Desktop\nBrowser\nDevelopment"}
                    className={textareaClass}
                    rows={3}
                  />
                </div>
                <div>
                  <label className={labelClass}>{t("admin.workspaces.category")}</label>
                  <Input value={form.category} onChange={(e) => setForm({ ...form, category: e.target.value })} placeholder="Desktop" className={inputClass} />
                </div>
                <div>
                  <label className={labelClass}>{t("admin.workspaces.notes")}</label>
                  <textarea
                    value={form.notes}
                    onChange={(e) => setForm({ ...form, notes: e.target.value })}
                    placeholder="Internal notes about this workspace..."
                    className={textareaClass}
                    rows={2}
                  />
                </div>
              </CollapsibleSection>

              {/* Server Connection (only for server type) */}
              {form.workspace_type === "server" && (
                <CollapsibleSection title={t("admin.workspaces.server_connection")} defaultOpen>
                  <div>
                    <label className={labelClass}>{t("admin.workspaces.protocol")}</label>
                    <DarkSelect
                      fullWidth
                      value={form.server_protocol}
                      onChange={(v) => setForm({ ...form, server_protocol: v, server_port: v === "vnc" ? "5900" : "3389" })}
                      options={[
                        { value: "rdp", label: "RDP" },
                        { value: "vnc", label: "VNC" },
                      ]}
                    />
                  </div>
                  <div className="grid grid-cols-3 gap-4">
                    <div className="col-span-2">
                      <label className={labelClass}>{t("admin.workspaces.hostname")}</label>
                      <Input value={form.server_hostname} onChange={(e) => setForm({ ...form, server_hostname: e.target.value })} placeholder="192.168.1.100" className={inputClass} />
                    </div>
                    <div>
                      <label className={labelClass}>{t("admin.workspaces.port")}</label>
                      <Input type="number" value={form.server_port} onChange={(e) => setForm({ ...form, server_port: e.target.value })} className={inputClass} />
                    </div>
                  </div>
                  <div className="flex items-center gap-3">
                    <button
                      type="button"
                      onClick={() => setForm({ ...form, server_auth_mode: form.server_auth_mode === "static" ? "prompt" : "static" })}
                      className={`relative w-10 h-5 rounded-full transition-colors ${form.server_auth_mode === "prompt" ? "bg-oklavier-blue" : "bg-white/20"}`}
                    >
                      <span className={`absolute top-0.5 left-0.5 w-4 h-4 rounded-full bg-white transition-transform ${form.server_auth_mode === "prompt" ? "translate-x-5" : ""}`} />
                    </button>
                    <span className="text-white text-sm">{t("admin.workspaces.ask_credentials")}</span>
                    {form.server_auth_mode === "prompt" && <span className="text-oklavier-blue/60 text-xs ml-auto">{t("admin.workspaces.ask_credentials_hint")}</span>}
                  </div>
                  {form.server_auth_mode === "prompt" && (
                    <div className="flex items-center gap-3">
                      <button
                        type="button"
                        onClick={() => setForm({ ...form, server_allow_remember: !form.server_allow_remember })}
                        className={`relative w-10 h-5 rounded-full transition-colors ${form.server_allow_remember ? "bg-oklavier-blue" : "bg-white/20"}`}
                      >
                        <span className={`absolute top-0.5 left-0.5 w-4 h-4 rounded-full bg-white transition-transform ${form.server_allow_remember ? "translate-x-5" : ""}`} />
                      </button>
                      <span className="text-white text-sm">{t("admin.workspaces.allow_remember")}</span>
                    </div>
                  )}
                  {form.server_protocol === "rdp" && form.server_auth_mode === "static" && (
                    <>
                      <div className="grid grid-cols-2 gap-4">
                        <div>
                          <label className={labelClass}>{t("admin.workspaces.username")}</label>
                          <Input value={form.server_username} onChange={(e) => setForm({ ...form, server_username: e.target.value })} placeholder="Administrator" className={inputClass} />
                        </div>
                        <div>
                          <label className={labelClass}>{t("admin.workspaces.password")}</label>
                          <Input type="password" value={form.server_password} onChange={(e) => setForm({ ...form, server_password: e.target.value })} placeholder="password" className={inputClass} />
                        </div>
                      </div>
                      <div className="grid grid-cols-2 gap-4">
                        <div>
                          <label className={labelClass}>{t("admin.workspaces.domain")} <span className="text-white/30">({t("workspace.optional")})</span></label>
                          <Input value={form.server_domain} onChange={(e) => setForm({ ...form, server_domain: e.target.value })} placeholder="WORKGROUP" className={inputClass} />
                        </div>
                        <div>
                          <label className={labelClass}>{t("admin.workspaces.security")}</label>
                          <DarkSelect
                            fullWidth
                            value={form.server_security}
                            onChange={(v) => setForm({ ...form, server_security: v })}
                            options={[
                              { value: "any", label: "Any" },
                              { value: "nla", label: "NLA" },
                              { value: "tls", label: "TLS" },
                              { value: "rdp", label: "RDP" },
                            ]}
                          />
                        </div>
                      </div>
                      <div className="flex items-center gap-3">
                        <button
                          type="button"
                          onClick={() => setForm({ ...form, server_ignore_cert: !form.server_ignore_cert })}
                          className={`relative w-10 h-5 rounded-full transition-colors ${form.server_ignore_cert ? "bg-oklavier-blue" : "bg-white/20"}`}
                        >
                          <span className={`absolute top-0.5 left-0.5 w-4 h-4 rounded-full bg-white transition-transform ${form.server_ignore_cert ? "translate-x-5" : ""}`} />
                        </button>
                        <span className="text-white text-sm">{t("admin.workspaces.ignore_cert")}</span>
                      </div>
                    </>
                  )}
                  {form.server_protocol === "vnc" && form.server_auth_mode === "static" && (
                    <div>
                      <label className={labelClass}>{t("admin.workspaces.vnc_password")} <span className="text-white/30">({t("workspace.optional")})</span></label>
                      <Input type="password" value={form.server_password} onChange={(e) => setForm({ ...form, server_password: e.target.value })} placeholder="password" className={inputClass} />
                    </div>
                  )}
                </CollapsibleSection>
              )}

              {/* Container (only for container type) */}
              {form.workspace_type === "container" && <CollapsibleSection title={t("admin.workspaces.container")} defaultOpen>
                <div>
                  <label className={labelClass}>{t("admin.workspaces.docker_image")}</label>
                  <Input value={form.docker_image} onChange={(e) => setForm({ ...form, docker_image: e.target.value })} placeholder="eamate/oklavier:chrome" className={inputClass} />
                </div>
                <div className="grid grid-cols-3 gap-4">
                  <div>
                    <label className={labelClass}>{t("admin.workspaces.docker_registry")}</label>
                    <Input value={form.docker_registry} onChange={(e) => setForm({ ...form, docker_registry: e.target.value })} placeholder="registry.example.com" className={inputClass} />
                  </div>
                  <div>
                    <label className={labelClass}>{t("admin.workspaces.docker_user")}</label>
                    <Input value={form.docker_user} onChange={(e) => setForm({ ...form, docker_user: e.target.value })} placeholder="username" className={inputClass} />
                  </div>
                  <div>
                    <label className={labelClass}>{t("admin.workspaces.docker_password")}</label>
                    <Input type="password" value={form.docker_password} onChange={(e) => setForm({ ...form, docker_password: e.target.value })} placeholder="password" className={inputClass} />
                  </div>
                </div>
                <div className="grid grid-cols-4 gap-4">
                  <div>
                    <label className={labelClass}>{t("admin.workspaces.cpu_cores")}</label>
                    <Input type="number" value={form.cores} onChange={(e) => setForm({ ...form, cores: e.target.value })} className={inputClass} />
                  </div>
                  <div>
                    <label className={labelClass}>{t("admin.workspaces.ram_mb")}</label>
                    <Input type="number" value={form.memory} onChange={(e) => setForm({ ...form, memory: e.target.value })} className={inputClass} />
                  </div>
                  <div>
                    <label className={labelClass}>{t("admin.workspaces.gpu_count")}</label>
                    <Input type="number" value={form.gpu_count} onChange={(e) => setForm({ ...form, gpu_count: e.target.value })} className={inputClass} />
                  </div>
                  <div>
                    <label className={labelClass}>{t("admin.workspaces.shm_size")}</label>
                    <Input value={form.shm_size} onChange={(e) => setForm({ ...form, shm_size: e.target.value })} placeholder="256m" className={inputClass} />
                  </div>
                </div>
                <div>
                  <label className={labelClass}>{t("admin.workspaces.uncompressed_size")} <span className="text-white/30">{t("admin.workspaces.unknown")}</span></label>
                  <Input type="number" value={form.uncompressed_size_mb} onChange={(e) => setForm({ ...form, uncompressed_size_mb: e.target.value })} className={inputClass} />
                </div>
              </CollapsibleSection>}

              {/* Default Display Settings (server only) */}
              {form.workspace_type === "server" && (
                <CollapsibleSection title={t("admin.workspaces.display_settings")}>
                  <p className="text-white/25 text-xs mb-3">These defaults apply when the user has no saved preferences.</p>
                  <div>
                    <label className={labelClass}>{t("admin.workspaces.default_settings_json")}</label>
                    <textarea
                      value={form.server_default_settings}
                      onChange={(e) => setForm({ ...form, server_default_settings: e.target.value })}
                      placeholder='{"enable-font-smoothing":"true","enable-wallpaper":"","enable-theming":"true","color-depth":"32"}'
                      className={textareaClass}
                      rows={4}
                    />
                    <p className="text-white/20 text-xs mt-1">Keys: enable-font-smoothing, enable-wallpaper, enable-theming, enable-desktop-composition, enable-full-window-drag, enable-menu-animations, enable-audio, color-depth, display-scale</p>
                  </div>
                </CollapsibleSection>
              )}

              {/* Session */}
              <CollapsibleSection title={t("admin.workspaces.session")}>
                <div>
                  <label className={labelClass}>{t("admin.workspaces.session_time_limit")} <span className="text-white/30">{t("admin.workspaces.unlimited")}</span></label>
                  <Input type="number" value={form.session_time_limit} onChange={(e) => setForm({ ...form, session_time_limit: e.target.value })} className={inputClass} />
                </div>
                {form.workspace_type === "container" && (
                  <>
                    <div>
                      <label className={labelClass}>{t("admin.workspaces.persistent_size")}</label>
                      <Input value={form.persistent_size} onChange={(e) => setForm({ ...form, persistent_size: e.target.value })} placeholder="10Gi" className={inputClass} disabled={!form.persistent} />
                    </div>
                    <div className="flex items-center gap-3">
                      <button
                        type="button"
                        onClick={() => setForm({ ...form, persistent: !form.persistent })}
                        className={`relative w-10 h-5 rounded-full transition-colors ${form.persistent ? "bg-oklavier-blue" : "bg-white/20"}`}
                      >
                        <span className={`absolute top-0.5 left-0.5 w-4 h-4 rounded-full bg-white transition-transform ${form.persistent ? "translate-x-5" : ""}`} />
                      </button>
                      <span className="text-white text-sm">{t("admin.workspaces.persistent")}</span>
                    </div>
                  </>
                )}
                <div className="flex items-center gap-3">
                  <button
                    type="button"
                    onClick={() => setForm({ ...form, record_sessions: !form.record_sessions })}
                    className={`relative w-10 h-5 rounded-full transition-colors ${form.record_sessions ? "bg-oklavier-blue" : "bg-white/20"}`}
                  >
                    <span className={`absolute top-0.5 left-0.5 w-4 h-4 rounded-full bg-white transition-transform ${form.record_sessions ? "translate-x-5" : ""}`} />
                  </button>
                  <span className="text-white text-sm">{t("admin.workspaces.record_sessions")}</span>
                  {form.record_sessions && <span className="text-oklavier-blue/60 text-xs ml-auto">{t("admin.workspaces.record_sessions_hint")}</span>}
                </div>
              </CollapsibleSection>

              {/* Restrictions */}
              <CollapsibleSection title={t("admin.workspaces.restrictions")}>
                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <label className={labelClass}>{t("admin.workspaces.restrict_agent")}</label>
                    <DarkSelect
                      fullWidth
                      value={form.restrict_to_agent}
                      onChange={(v) => setForm({ ...form, restrict_to_agent: v })}
                      options={[
                        { value: "", label: t("admin.workspaces.any_agent") },
                        ...agents.map((a) => ({ value: a.id, label: `${a.name}${a.region ? ` (${a.region})` : ""}` })),
                      ]}
                    />
                  </div>
                  <div>
                    <label className={labelClass}>{t("admin.workspaces.restrict_region")}</label>
                    <Input value={form.restrict_to_region} onChange={(e) => setForm({ ...form, restrict_to_region: e.target.value })} placeholder="eu-west-1" className={inputClass} />
                  </div>
                </div>
              </CollapsibleSection>

              {/* Advanced (container only) */}
              {form.workspace_type === "container" && <CollapsibleSection title={t("admin.workspaces.advanced")}>
                <div>
                  <label className={labelClass}>{t("admin.workspaces.run_config")} <span className="text-white/30">(JSON)</span></label>
                  <textarea
                    value={form.run_config}
                    onChange={(e) => setForm({ ...form, run_config: e.target.value })}
                    placeholder="{}"
                    className={textareaClass}
                    rows={4}
                  />
                </div>
                <div>
                  <label className={labelClass}>{t("admin.workspaces.exec_config")} <span className="text-white/30">(JSON)</span></label>
                  <textarea
                    value={form.exec_config}
                    onChange={(e) => setForm({ ...form, exec_config: e.target.value })}
                    placeholder="{}"
                    className={textareaClass}
                    rows={4}
                  />
                </div>
                <div>
                  <label className={labelClass}>{t("admin.workspaces.volume_mappings")} <span className="text-white/30">(JSON)</span></label>
                  <textarea
                    value={form.volume_mappings}
                    onChange={(e) => setForm({ ...form, volume_mappings: e.target.value })}
                    placeholder="{}"
                    className={textareaClass}
                    rows={4}
                  />
                </div>
              </CollapsibleSection>}

              {/* Access / Groups */}
              {groups.length > 0 && (
                <CollapsibleSection title={t("admin.workspaces.access")}>
                  <div>
                    <label className={labelClass}>{t("admin.workspaces.authorized_groups")} <span className="text-white/20">{t("admin.workspaces.empty_all")}</span></label>
                    <div className="flex flex-wrap gap-2">
                      {groups.map((g) => (
                        <button key={g.id}
                          onClick={() => setSelectedGroups(prev => prev.includes(g.id) ? prev.filter(id => id !== g.id) : [...prev, g.id])}
                          className={`text-xs px-3 py-1.5 rounded-full border transition-all ${
                            selectedGroups.includes(g.id)
                              ? "border-transparent text-white"
                              : "border-white/10 text-white/40 hover:text-white/60"
                          }`}
                          style={selectedGroups.includes(g.id) ? { backgroundColor: g.color + "30", color: g.color, borderColor: g.color + "50" } : {}}
                        >
                          {g.name}
                        </button>
                      ))}
                    </div>
                  </div>
                </CollapsibleSection>
              )}

            </div>

            {/* Fixed footer */}
            <div className="flex justify-end gap-3 px-6 py-4 border-t border-white/10">
              <button onClick={() => setShowForm(false)} className="px-5 border border-white/10 text-white/70 hover:text-white hover:bg-white/10 rounded-lg py-2 text-sm transition-colors">{t("common.cancel")}</button>
              <Button onClick={handleSave} className="px-5 bg-oklavier-blue hover:bg-oklavier-purple">{editing ? t("common.save") : t("common.create")}</Button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
