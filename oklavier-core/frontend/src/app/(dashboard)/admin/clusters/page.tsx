"use client";

import { useState, useEffect } from "react";
import { Plus, Pencil, Trash2, Loader2, Server, CheckCircle, XCircle, RefreshCw, Cpu, HardDrive, Monitor } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useTranslation } from "@/lib/i18n";
import { useToast } from "@/components/toast";
import { authFetch } from "@/lib/auth-fetch";

interface Cluster {
  id: string;
  name: string;
  description: string;
  namespace: string;
  is_default: boolean;
  enabled: boolean;
  status: string;
  node_count: number;
  cpu_total: number;
  memory_total_gb: number;
  last_check: string;
}

export default function ClustersPage() {
  const { t } = useTranslation();
  const toast = useToast();
  const [clusters, setClusters] = useState<Cluster[]>([]);
  const [loading, setLoading] = useState(true);
  const [showForm, setShowForm] = useState(false);
  const [editing, setEditing] = useState<Cluster | null>(null);
  const [testing, setTesting] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [form, setForm] = useState({ name: "", description: "", kubeconfig: "", namespace: "default" });

  async function fetchClusters() {
    try {
      const res = await authFetch("/api/admin/clusters");
      if (!res.ok) return;
      const data = await res.json();
      if (data.clusters) setClusters(data.clusters);
    } catch {
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => { fetchClusters(); }, []);

  function openNew() {
    setEditing(null);
    setForm({ name: "", description: "", kubeconfig: "", namespace: "default" });
    setShowForm(true);
  }

  function openEdit(c: Cluster) {
    setEditing(c);
    setForm({ name: c.name, description: c.description || "", kubeconfig: "", namespace: c.namespace });
    setShowForm(true);
  }

  async function handleSave() {
    setSaving(true);
    try {
      const res = await authFetch("/api/admin/clusters", {
        method: editing ? "PUT" : "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ ...form, id: editing?.id }),
      });
      if (!res.ok) { toast.error(t("common.error")); return; }
      setShowForm(false);
      toast.success(editing ? t("admin.clusters.updated") : t("admin.clusters.created"));
      fetchClusters();
    } catch (e) {
      toast.error(t("common.error"));
    } finally {
      setSaving(false);
    }
  }

  async function handleDelete(id: string) {
    if (!confirm(t("admin.clusters.confirm_delete"))) return;
    try {
      const res = await authFetch("/api/admin/clusters", {
        method: "DELETE",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ id }),
      });
      if (!res.ok) { toast.error(t("common.error")); return; }
      toast.success(t("admin.clusters.deleted"));
      fetchClusters();
    } catch (e) {
      toast.error(t("common.error"));
    }
  }

  async function handleTest(id: string) {
    setTesting(id);
    try {
      const res = await authFetch("/api/admin/clusters/test", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ id }),
      });
      if (!res.ok) { toast.error(t("admin.clusters.test_error")); return; }
      const data = await res.json();
      if (data.status === "connected") {
        toast.success(t("admin.clusters.test_success"));
      } else {
        toast.error(t("admin.clusters.test_error"));
      }
      await fetchClusters();
    } catch (e) {
      toast.error(t("common.error"));
    } finally {
      setTesting(null);
    }
  }

  async function handleSetDefault(id: string) {
    try {
      const res = await authFetch("/api/admin/clusters", {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ id, is_default: true }),
      });
      if (!res.ok) { toast.error(t("common.error")); return; }
      fetchClusters();
    } catch (e) {
      toast.error(t("common.error"));
    }
  }

  // Only show full-page loader on initial load (no data yet)
  if (loading && clusters.length === 0) return <div className="flex items-center gap-2 text-white/60"><Loader2 className="size-5 animate-spin" /> {t("common.loading")}</div>;

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-white">{t("admin.clusters.title")}</h1>
          <p className="text-white/50 text-sm">{t("admin.clusters.subtitle", { count: clusters.length })}</p>
        </div>
        <Button onClick={openNew} className="bg-oklavier-blue hover:bg-oklavier-purple">
          <Plus className="size-4 mr-2" /> {t("admin.clusters.add")}
        </Button>
      </div>

      {clusters.length === 0 ? (
        <div className="bg-[#1a1f36] border border-white/10 rounded-xl p-12 text-center">
          <Server className="size-12 text-white/20 mx-auto mb-4" />
          <p className="text-white/40">{t("admin.clusters.no_clusters")}</p>
          <p className="text-white/25 text-sm mt-1">{t("admin.clusters.no_clusters_desc")}</p>
        </div>
      ) : (
        <div className="space-y-3">
          {clusters.map((c) => (
            <div key={c.id} className="bg-[#1a1f36] border border-white/10 rounded-xl p-5">
              <div className="flex items-center gap-4">
                <div className={`size-12 rounded-xl flex items-center justify-center ${
                  c.status === "connected" ? "bg-green-500/10" : c.status === "error" ? "bg-red-500/10" : "bg-white/5"
                }`}>
                  <Server className={`size-6 ${
                    c.status === "connected" ? "text-green-400" : c.status === "error" ? "text-red-400" : "text-white/30"
                  }`} />
                </div>
                <div className="flex-1">
                  <div className="flex items-center gap-2">
                    <p className="text-white font-semibold">{c.name}</p>
                    {c.is_default && (
                      <span className="text-[10px] bg-oklavier-blue/20 text-oklavier-blue px-2 py-0.5 rounded-full">{t("common.default")}</span>
                    )}
                    <span className={`inline-flex items-center gap-1 text-[10px] px-2 py-0.5 rounded-full ${
                      c.status === "connected" ? "bg-green-500/10 text-green-400" :
                      c.status === "error" ? "bg-red-500/10 text-red-400" :
                      "bg-white/5 text-white/40"
                    }`}>
                      {c.status === "connected" ? <CheckCircle className="size-2.5" /> : c.status === "error" ? <XCircle className="size-2.5" /> : null}
                      {c.status}
                    </span>
                  </div>
                  {c.description && <p className="text-white/40 text-sm">{c.description}</p>}
                  <div className="flex items-center gap-4 mt-2">
                    <span className="text-white/30 text-xs">ns: <code className="text-oklavier-blue/60">{c.namespace}</code></span>
                    {c.status === "connected" && (
                      <>
                        <span className="flex items-center gap-1 text-white/30 text-xs"><Monitor className="size-3" /> {c.node_count} nodes</span>
                        <span className="flex items-center gap-1 text-white/30 text-xs"><Cpu className="size-3" /> {c.cpu_total} vCPU</span>
                        <span className="flex items-center gap-1 text-white/30 text-xs"><HardDrive className="size-3" /> {c.memory_total_gb} Go RAM</span>
                      </>
                    )}
                  </div>
                </div>
                <div className="flex items-center gap-1">
                  <button
                    onClick={() => handleTest(c.id)}
                    disabled={testing === c.id}
                    className="p-2 rounded-lg text-white/40 hover:text-white hover:bg-white/10 transition-colors"
                  >
                    <RefreshCw className={`size-4 ${testing === c.id ? "animate-spin" : ""}`} />
                  </button>
                  {!c.is_default && (
                    <button onClick={() => handleSetDefault(c.id)} className="p-2 rounded-lg text-oklavier-blue/40 hover:text-oklavier-blue hover:bg-oklavier-blue/10 transition-colors text-xs">
                      {t("admin.clusters.set_default")}
                    </button>
                  )}
                  <button onClick={() => openEdit(c)} className="p-2 rounded-lg text-white/40 hover:text-white hover:bg-white/10 transition-colors">
                    <Pencil className="size-4" />
                  </button>
                  <button onClick={() => handleDelete(c.id)} className="p-2 rounded-lg text-red-400/40 hover:text-red-400 hover:bg-red-500/10 transition-colors">
                    <Trash2 className="size-4" />
                  </button>
                </div>
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Add/Edit Modal */}
      {showForm && (
        <div className="fixed inset-0 z-50 flex items-center justify-center">
          <div className="absolute inset-0 bg-black/50 backdrop-blur-sm" onClick={() => setShowForm(false)} />
          <div className="relative z-50 w-full max-w-lg bg-[#1a1f36] border border-white/10 rounded-2xl p-6 shadow-2xl max-h-[80vh] overflow-y-auto">
            <h2 className="text-white text-xl font-semibold mb-6">{editing ? t("admin.clusters.edit") : t("admin.clusters.add")}</h2>
            <div className="space-y-4">
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="text-white/60 text-sm mb-1 block">{t("admin.clusters.name")}</label>
                  <Input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} placeholder="production-cluster" className="bg-[#2e3862]/80 border-white/10 text-white" />
                </div>
                <div>
                  <label className="text-white/60 text-sm mb-1 block">{t("admin.clusters.namespace")}</label>
                  <Input value={form.namespace} onChange={(e) => setForm({ ...form, namespace: e.target.value })} placeholder="default" className="bg-[#2e3862]/80 border-white/10 text-white" />
                </div>
              </div>
              <div>
                <label className="text-white/60 text-sm mb-1 block">{t("admin.clusters.description")}</label>
                <Input value={form.description} onChange={(e) => setForm({ ...form, description: e.target.value })} placeholder={t("admin.clusters.desc_placeholder")} className="bg-[#2e3862]/80 border-white/10 text-white" />
              </div>
              <div>
                <label className="text-white/60 text-sm mb-1 block">{t("admin.clusters.kubeconfig")}</label>
                <textarea
                  value={form.kubeconfig}
                  onChange={(e) => setForm({ ...form, kubeconfig: e.target.value })}
                  placeholder="apiVersion: v1&#10;kind: Config&#10;clusters:&#10;  - cluster:&#10;      server: https://..."
                  className="w-full h-48 bg-[#2e3862]/80 border border-white/10 rounded-lg text-white text-xs font-mono p-3 resize-none focus:outline-none focus:border-oklavier-blue"
                />
                <p className="text-white/25 text-xs mt-1">{t("admin.clusters.kubeconfig_placeholder")}</p>
              </div>
            </div>
            <div className="flex gap-3 mt-6">
              <button onClick={() => setShowForm(false)} className="flex-1 border border-white/10 text-white/70 hover:text-white hover:bg-white/10 rounded-lg py-2 px-4 text-sm font-medium transition-colors">{t("common.cancel")}</button>
              <Button onClick={handleSave} disabled={saving} className="flex-1 bg-oklavier-blue hover:bg-oklavier-purple">{saving ? <Loader2 className="size-4 animate-spin" /> : (editing ? t("common.save") : t("common.add"))}</Button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
