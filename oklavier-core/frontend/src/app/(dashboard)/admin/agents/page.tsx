"use client";

import { useState, useEffect } from "react";
import { Plus, Trash2, Server, CheckCircle, XCircle, Clock, Cpu, HardDrive, Monitor, Play, Copy, Check, ArrowUpDown } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useTranslation } from "@/lib/i18n";
import { useToast } from "@/components/toast";
import { ConfirmModal } from "@/components/confirm-modal";
import { useAPI, apiAction } from "@/lib/api";
import { DataTable } from "@/components/data-table";
import { ColumnDef } from "@tanstack/react-table";

interface Agent {
  id: string;
  name: string;
  region: string;
  namespace: string;
  status: string;
  node_count: number;
  cpu_total: number;
  memory_total_gb: number;
  active_sessions: number;
  version: string;
  last_heartbeat: string;
}

function timeAgo(date: string, neverLabel: string) {
  if (!date) return neverLabel;
  const diff = Date.now() - new Date(date).getTime();
  const secs = Math.floor(diff / 1000);
  if (secs < 60) return `${secs}s`;
  if (secs < 3600) return `${Math.floor(secs / 60)}m`;
  return `${Math.floor(secs / 3600)}h`;
}

function isAgentAlive(a: Agent) {
  return a.last_heartbeat && (Date.now() - new Date(a.last_heartbeat).getTime()) < 60000;
}

export default function AgentsPage() {
  const [showCreate, setShowCreate] = useState(false);
  const [showInstall, setShowInstall] = useState<{ token: string; name: string; namespace: string } | null>(null);
  const [copied, setCopied] = useState(false);
  const [form, setForm] = useState({ name: "", region: "", namespace: "oklavier", controlPlane: "", publicUrl: "" });
  const [deleteTarget, setDeleteTarget] = useState<string | null>(null);
  const [deleting, setDeleting] = useState(false);
  const { t } = useTranslation();
  const toast = useToast();

  const [page, setPage] = useState(1);
  const [search, setSearch] = useState("");
  const perPage = 25;

  const apiUrl = `/api/admin/agents?page=${page}&per_page=${perPage}&search=${encodeURIComponent(search)}`;
  const { data: agentsData, isLoading: loading, mutate } = useAPI<{ agents: Agent[]; total: number }>(apiUrl, { refreshInterval: 15000 });
  const agents = agentsData?.agents || [];
  const total = agentsData?.total || 0;
  const totalPages = Math.ceil(total / perPage);

  // Set default control plane URL from current page
  useEffect(() => {
    if (typeof window !== "undefined" && !form.controlPlane) {
      setForm(f => ({ ...f, controlPlane: window.location.origin }));
    }
  }, []);

  async function handleCreate() {
    const res = await apiAction("/api/admin/agents", "POST", { name: form.name, region: form.region, namespace: form.namespace, public_url: form.publicUrl });
    if (res.ok) {
      toast.success(t("admin.agents.created"));
      setShowCreate(false);
      setShowInstall({ token: res.data.token, name: form.name, namespace: form.namespace });
    } else {
      toast.error(t("common.error"));
    }
    mutate();
  }

  function handleDelete(id: string) {
    setDeleteTarget(id);
  }

  async function confirmDelete() {
    if (!deleteTarget) return;
    setDeleting(true);
    const res = await apiAction("/api/admin/agents", "DELETE", { id: deleteTarget });
    if (res.ok) {
      toast.success(t("admin.agents.deleted"));
    } else {
      toast.error(t("common.error"));
    }
    mutate();
    setDeleting(false);
    setDeleteTarget(null);
  }

  function getHelmCommand(token: string, name: string, namespace: string) {
    const cpUrl = form.controlPlane || window.location.origin;
    return `# Download the agent chart from: https://github.com/eamate/oklavier-agent
# Then run:
helm upgrade --install oklavier-agent ./oklavier-agent/helm \\
  --namespace ${namespace} \\
  --create-namespace \\
  --set agent.name="${name}" \\
  --set agent.token="${token}" \\
  --set agent.controlPlane="${cpUrl}" \\
  --set agent.region="${form.region || "default"}" \\
  --set image.repository="eamate/oklavier" \\
  --set image.tag="agent-latest"`;
  }

  function copyToClipboard(text: string) {
    navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  }

  const columns: ColumnDef<Agent>[] = [
    {
      accessorKey: "name",
      header: ({ column }) => (
        <button className="flex items-center gap-1" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
          {t("admin.agents.agent_name")}
          <ArrowUpDown className="size-3" />
        </button>
      ),
      cell: ({ row }) => {
        const a = row.original;
        return (
          <div className="flex items-center gap-2">
            <span className="text-white font-medium">{a.name}</span>
            <span className="text-[10px] bg-oklavier-blue/20 text-oklavier-blue px-2 py-0.5 rounded-full">{a.region}</span>
          </div>
        );
      },
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
        const a = row.original;
        const alive = isAgentAlive(a);
        return (
          <span className={`inline-flex items-center gap-1 text-xs px-2 py-1 rounded-full ${
            alive ? "bg-green-500/10 text-green-400" :
            a.status === "pending" ? "bg-yellow-500/10 text-yellow-400" :
            "bg-red-500/10 text-red-400"
          }`}>
            {alive ? <CheckCircle className="size-2.5" /> : a.status === "pending" ? <Clock className="size-2.5" /> : <XCircle className="size-2.5" />}
            {alive ? t("admin.agents.connected") : a.status === "pending" ? t("admin.agents.pending") : t("admin.agents.disconnected")}
          </span>
        );
      },
    },
    {
      accessorKey: "namespace",
      header: t("admin.agents.namespace"),
      cell: ({ row }) => <code className="text-oklavier-blue/60 text-xs">{row.original.namespace}</code>,
    },
    {
      id: "version",
      header: "Version",
      cell: ({ row }) => {
        const a = row.original;
        return a.version ? <code className="text-xs text-oklavier-blue/60">v{a.version}</code> : <span className="text-white/20 text-xs">—</span>;
      },
    },
    {
      id: "sessions",
      header: "Sessions",
      cell: ({ row }) => {
        const a = row.original;
        if (!isAgentAlive(a)) return <span className="text-white/20 text-xs">—</span>;
        return <span className="flex items-center gap-1 text-white/40 text-xs"><Play className="size-3" /> {a.active_sessions}</span>;
      },
    },
    {
      accessorKey: "last_heartbeat",
      header: ({ column }) => (
        <button className="flex items-center gap-1" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
          {t("admin.agents.synced")}
          <ArrowUpDown className="size-3" />
        </button>
      ),
      cell: ({ row }) => <span className="text-white/40 text-xs">{timeAgo(row.original.last_heartbeat, t("admin.agents.never"))}</span>,
    },
    {
      id: "actions",
      header: () => <span className="sr-only">{t("common.actions")}</span>,
      cell: ({ row }) => (
        <div className="flex items-center justify-end">
          <button onClick={() => handleDelete(row.original.id)} className="p-2 rounded-lg text-red-400/40 hover:text-red-400 hover:bg-red-500/10 transition-colors">
            <Trash2 className="size-4" />
          </button>
        </div>
      ),
    },
  ];

  return (
    <div>
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-white">{t("admin.agents.title")}</h1>
        <p className="text-white/50 text-sm">{t("admin.agents.subtitle", { count: total })}</p>
      </div>

      <DataTable
        columns={columns}
        data={agents}
        manualPagination
        page={page}
        pageCount={totalPages}
        total={total}
        onPageChange={setPage}
        onSearchChange={(s) => { setSearch(s); setPage(1); }}
        emptyMessage={t("admin.agents.no_agents")}
        toolbar={
          <Button onClick={() => { setShowCreate(true); setForm(f => ({ ...f, name: "", region: "", namespace: "oklavier" })); }} className="bg-oklavier-blue hover:bg-oklavier-purple">
            <Plus className="size-4 mr-2" /> {t("admin.agents.deploy")}
          </Button>
        }
      />

      <ConfirmModal
        open={!!deleteTarget}
        title="Delete agent"
        message="Are you sure you want to delete this agent?"
        variant="danger"
        loading={deleting}
        onConfirm={confirmDelete}
        onCancel={() => setDeleteTarget(null)}
      />

      {/* Create Modal */}
      {showCreate && (
        <div className="fixed inset-0 z-50 flex items-center justify-center">
          <div className="absolute inset-0 bg-black/50 backdrop-blur-sm" onClick={() => setShowCreate(false)} />
          <div className="relative z-50 w-full max-w-md bg-[#1a1f36] border border-white/10 rounded-2xl p-6 shadow-2xl">
            <h2 className="text-white text-xl font-semibold mb-6">{t("admin.agents.register")}</h2>
            <div className="space-y-4">
              <div>
                <label className="text-white/60 text-sm mb-1 block">{t("admin.agents.agent_name")}</label>
                <Input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} placeholder="paris-prod" className="bg-[#2e3862]/80 border-white/10 text-white" />
              </div>
              <div>
                <label className="text-white/60 text-sm mb-1 block">{t("admin.agents.region")}</label>
                <Input value={form.region} onChange={(e) => setForm({ ...form, region: e.target.value })} placeholder="eu-west-1 / paris / reunion" className="bg-[#2e3862]/80 border-white/10 text-white" />
              </div>
              <div>
                <label className="text-white/60 text-sm mb-1 block">{t("admin.agents.namespace")}</label>
                <Input value={form.namespace} onChange={(e) => setForm({ ...form, namespace: e.target.value })} placeholder="oklavier" className="bg-[#2e3862]/80 border-white/10 text-white" />
              </div>
              <div>
                <label className="text-white/60 text-sm mb-1 block">{t("admin.agents.control_plane")}</label>
                <Input value={form.controlPlane} onChange={(e) => setForm({ ...form, controlPlane: e.target.value })} placeholder="https://oklavier.example.com" className="bg-[#2e3862]/80 border-white/10 text-white" />
                <p className="text-white/25 text-xs mt-1">{t("admin.agents.control_plane_desc")}</p>
              </div>
              <div>
                <label className="text-white/60 text-sm mb-1 block">{t("admin.agents.public_url")} <span className="text-white/20">(optionnel)</span></label>
                <Input value={form.publicUrl} onChange={(e) => setForm({ ...form, publicUrl: e.target.value })} placeholder="http://192.168.1.50:4444 ou https://agent.example.com" className="bg-[#2e3862]/80 border-white/10 text-white" />
                <p className="text-white/25 text-xs mt-1">{t("admin.agents.public_url_desc")}</p>
              </div>
            </div>
            <div className="flex gap-3 mt-6">
              <button onClick={() => setShowCreate(false)} className="flex-1 border border-white/10 text-white/70 hover:text-white hover:bg-white/10 rounded-lg py-2 text-sm transition-colors">{t("common.cancel")}</button>
              <Button onClick={handleCreate} disabled={!form.name} className="flex-1 bg-oklavier-blue hover:bg-oklavier-purple">{t("admin.agents.generate_token")}</Button>
            </div>
          </div>
        </div>
      )}

      {/* Install Instructions Modal */}
      {showInstall && (
        <div className="fixed inset-0 z-50 flex items-center justify-center">
          <div className="absolute inset-0 bg-black/50 backdrop-blur-sm" onClick={() => setShowInstall(null)} />
          <div className="relative z-50 w-full max-w-2xl bg-[#1a1f36] border border-white/10 rounded-2xl p-6 shadow-2xl">
            <div className="flex items-center gap-3 mb-2">
              <div className="size-10 rounded-xl bg-green-500/10 flex items-center justify-center">
                <CheckCircle className="size-5 text-green-400" />
              </div>
              <div>
                <h2 className="text-white text-xl font-semibold">{t("admin.agents.registered")}</h2>
                <p className="text-white/40 text-sm">{t("admin.agents.run_command")}</p>
              </div>
            </div>

            <div className="mt-4 mb-2">
              <p className="text-white/50 text-xs mb-2 font-semibold uppercase tracking-wider">{t("admin.agents.helm_install")}</p>
              <div className="relative">
                <pre className="bg-[#0f1225] border border-white/10 rounded-lg p-4 text-xs text-green-400 font-mono overflow-x-auto whitespace-pre">
{getHelmCommand(showInstall.token, showInstall.name, showInstall.namespace)}
                </pre>
                <button
                  onClick={() => copyToClipboard(getHelmCommand(showInstall.token, showInstall.name, showInstall.namespace))}
                  className="absolute top-2 right-2 p-2 rounded-lg bg-white/10 text-white/60 hover:text-white hover:bg-white/20 transition-colors"
                >
                  {copied ? <Check className="size-4 text-green-400" /> : <Copy className="size-4" />}
                </button>
              </div>
            </div>

            <div className="bg-oklavier-blue/5 border border-oklavier-blue/20 rounded-lg p-3 mt-4">
              <p className="text-oklavier-blue text-xs">
                <strong>{t("admin.agents.token")} :</strong> <code className="bg-black/20 px-1.5 py-0.5 rounded text-[11px]">{showInstall.token}</code>
              </p>
              <p className="text-white/30 text-xs mt-1">{t("admin.agents.token_unique")}</p>
            </div>

            <p className="text-white/25 text-xs mt-4">
              {t("admin.agents.auto_connect")}
            </p>

            <div className="flex justify-end mt-4">
              <Button onClick={() => setShowInstall(null)} className="bg-oklavier-blue hover:bg-oklavier-purple">{t("common.close")}</Button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
