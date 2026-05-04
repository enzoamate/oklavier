"use client";

import { useState } from "react";
import { Plus, Trash2, Loader2, UsersRound, ArrowRight, Link2, ArrowUpDown } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useTranslation } from "@/lib/i18n";
import { useToast } from "@/components/toast";
import { ConfirmModal } from "@/components/confirm-modal";
import { useAPI, apiAction } from "@/lib/api";
import { DataTable } from "@/components/data-table";
import { ColumnDef } from "@tanstack/react-table";

interface Group {
  id: string;
  name: string;
  description: string;
  color: string;
  is_default?: boolean;
  max_sessions?: number;
  max_cpu?: number;
  max_memory?: number;
}

interface Mapping {
  id: string;
  oidc_role: string;
  group_id: string;
}

interface DiscoveredRole {
  role: string;
  user_count: number;
}

export default function GroupsPage() {
  const [showCreateGroup, setShowCreateGroup] = useState(false);
  const [showCreateMapping, setShowCreateMapping] = useState(false);
  const [groupForm, setGroupForm] = useState({ name: "", description: "", color: "#7096ff", max_sessions: 0, max_cpu: 0, max_memory: 0 });
  const [mappingForm, setMappingForm] = useState({ oidc_role: "", group_id: "" });
  const [deleteGroupTarget, setDeleteGroupTarget] = useState<string | null>(null);
  const [deletingGroup, setDeletingGroup] = useState(false);
  const [deleteMappingTarget, setDeleteMappingTarget] = useState<string | null>(null);
  const [deletingMapping, setDeletingMapping] = useState(false);
  const { t } = useTranslation();
  const toast = useToast();

  const [page, setPage] = useState(1);
  const [search, setSearch] = useState("");
  const perPage = 25;

  const groupsApiUrl = `/api/admin/groups?page=${page}&per_page=${perPage}&search=${encodeURIComponent(search)}`;
  const { data: groupsData, mutate: mutateGroups } = useAPI<{ groups: Group[]; total: number }>(groupsApiUrl);
  const { data: mappingsData, mutate: mutateMappings } = useAPI<{ mappings: Mapping[] }>("/api/admin/oidc-mappings");
  const { data: rolesData, mutate: mutateRoles } = useAPI<{ roles: DiscoveredRole[] }>("/api/admin/oidc-roles");

  const allGroups = groupsData?.groups || [];
  const totalGroups = groupsData?.total || 0;
  const totalGroupPages = Math.ceil(totalGroups / perPage);
  const mappings = mappingsData?.mappings || [];
  const discoveredRoles = rolesData?.roles || [];
  const initialLoading = !groupsData && !mappingsData && !rolesData;

  function invalidateAll() {
    mutateGroups();
    mutateMappings();
    mutateRoles();
  }

  async function handleCreateGroup() {
    const res = await apiAction("/api/admin/groups", "POST", groupForm);
    if (res.ok) {
      toast.success(t("admin.groups.created"));
    } else {
      toast.error(t("common.error"));
    }
    setShowCreateGroup(false);
    setGroupForm({ name: "", description: "", color: "#7096ff", max_sessions: 0, max_cpu: 0, max_memory: 0 });
    invalidateAll();
  }

  function handleDeleteGroup(id: string) {
    setDeleteGroupTarget(id);
  }

  async function confirmDeleteGroup() {
    if (!deleteGroupTarget) return;
    setDeletingGroup(true);
    const res = await apiAction("/api/admin/groups", "DELETE", { id: deleteGroupTarget });
    if (res.ok) {
      toast.success(t("admin.groups.deleted"));
    } else {
      toast.error(t("common.error"));
    }
    invalidateAll();
    setDeletingGroup(false);
    setDeleteGroupTarget(null);
  }

  async function handleCreateMapping() {
    const res = await apiAction("/api/admin/oidc-mappings", "POST", mappingForm);
    if (res.ok) {
      toast.success(t("admin.groups.mapping_created"));
    } else {
      toast.error(t("common.error"));
    }
    setShowCreateMapping(false);
    setMappingForm({ oidc_role: "", group_id: "" });
    invalidateAll();
  }

  function handleDeleteMapping(id: string) {
    setDeleteMappingTarget(id);
  }

  async function confirmDeleteMapping() {
    if (!deleteMappingTarget) return;
    setDeletingMapping(true);
    const res = await apiAction("/api/admin/oidc-mappings", "DELETE", { id: deleteMappingTarget });
    if (res.ok) {
      toast.success(t("admin.groups.mapping_deleted"));
    } else {
      toast.error(t("common.error"));
    }
    invalidateAll();
    setDeletingMapping(false);
    setDeleteMappingTarget(null);
  }

  function getGroupName(id: string) {
    return allGroups.find(g => g.id === id)?.name || "?";
  }

  function getGroupColor(id: string) {
    return allGroups.find(g => g.id === id)?.color || "#7096ff";
  }

  const groupColumns: ColumnDef<Group>[] = [
    {
      id: "color",
      header: "",
      cell: ({ row }) => (
        <div className="size-3 rounded-full" style={{ backgroundColor: row.original.color }} />
      ),
      size: 40,
    },
    {
      accessorKey: "name",
      header: ({ column }) => (
        <button className="flex items-center gap-1" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
          {t("admin.groups.name")}
          <ArrowUpDown className="size-3" />
        </button>
      ),
      cell: ({ row }) => (
        <div className="flex items-center gap-2">
          <span className="text-white font-medium">{row.original.name}</span>
          {row.original.is_default && <span className="text-[9px] bg-white/10 text-white/50 px-1.5 py-0.5 rounded">{t("admin.groups.default_badge")}</span>}
        </div>
      ),
    },
    {
      accessorKey: "description",
      header: t("admin.groups.description"),
      cell: ({ row }) => <span className="text-white/40 text-sm">{row.original.description}</span>,
    },
    {
      id: "actions",
      header: () => <span className="sr-only">{t("common.actions")}</span>,
      cell: ({ row }) => {
        if (row.original.is_default) return null;
        return (
          <div className="flex items-center justify-end">
            <button onClick={() => handleDeleteGroup(row.original.id)} className="p-1.5 rounded text-red-400/30 hover:text-red-400 hover:bg-red-500/10 transition-colors">
              <Trash2 className="size-3.5" />
            </button>
          </div>
        );
      },
    },
  ];

  // Only show full-page loader on initial load (no data yet)
  if (initialLoading) return <div className="flex items-center gap-2 text-white/60"><Loader2 className="size-5 animate-spin" /> {t("common.loading")}</div>;

  return (
    <div>
      {/* Groups Section */}
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-white">{t("admin.groups.title")}</h1>
        <p className="text-white/50 text-sm">{t("admin.groups.subtitle", { count: totalGroups })}</p>
      </div>

      <DataTable
        columns={groupColumns}
        data={allGroups}
        manualPagination
        page={page}
        pageCount={totalGroupPages}
        total={totalGroups}
        onPageChange={setPage}
        onSearchChange={(s) => { setSearch(s); setPage(1); }}
        toolbar={
          <Button onClick={() => setShowCreateGroup(true)} className="bg-oklavier-blue hover:bg-oklavier-purple">
            <Plus className="size-4 mr-2" /> {t("admin.groups.create")}
          </Button>
        }
      />

      <div className="mb-10" />

      {/* OIDC Mappings Section */}
      <div className="flex items-center justify-between mb-4">
        <div>
          <h2 className="text-xl font-bold text-white">{t("admin.groups.mapping_title")}</h2>
          <p className="text-white/50 text-sm">{t("admin.groups.mapping_subtitle")}</p>
        </div>
        <Button onClick={() => setShowCreateMapping(true)} className="bg-oklavier-blue hover:bg-oklavier-purple">
          <Plus className="size-4 mr-2" /> {t("admin.groups.add_mapping")}
        </Button>
      </div>

      {mappings.length === 0 ? (
        <div className="bg-[#1a1f36] border border-white/10 rounded-xl p-8 text-center">
          <Link2 className="size-10 text-white/20 mx-auto mb-3" />
          <p className="text-white/40 text-sm">{t("admin.groups.no_mapping")}</p>
          <p className="text-white/25 text-xs mt-1">{t("admin.groups.mapping_hint")}</p>
        </div>
      ) : (
        <div className="space-y-2">
          {mappings.map((m) => (
            <div key={m.id} className="bg-[#1a1f36] border border-white/10 rounded-xl p-4 flex items-center gap-4">
              <code className="text-orange-400 bg-orange-400/10 px-3 py-1.5 rounded-lg text-sm font-mono">{m.oidc_role}</code>
              <ArrowRight className="size-4 text-white/30" />
              <div className="flex items-center gap-2">
                <div className="size-3 rounded-full" style={{ backgroundColor: getGroupColor(m.group_id) }} />
                <span className="text-white text-sm">{getGroupName(m.group_id)}</span>
              </div>
              <div className="flex-1" />
              <button onClick={() => handleDeleteMapping(m.id)} className="p-1.5 rounded text-red-400/30 hover:text-red-400 hover:bg-red-500/10 transition-colors">
                <Trash2 className="size-3.5" />
              </button>
            </div>
          ))}
        </div>
      )}

      <ConfirmModal
        open={!!deleteGroupTarget}
        title={t("admin.groups.confirm_delete")}
        message={t("admin.groups.confirm_delete")}
        variant="danger"
        loading={deletingGroup}
        onConfirm={confirmDeleteGroup}
        onCancel={() => setDeleteGroupTarget(null)}
      />

      <ConfirmModal
        open={!!deleteMappingTarget}
        title={t("admin.groups.confirm_delete_mapping")}
        message={t("admin.groups.confirm_delete_mapping")}
        variant="warning"
        loading={deletingMapping}
        onConfirm={confirmDeleteMapping}
        onCancel={() => setDeleteMappingTarget(null)}
      />

      {/* Create Group Modal */}
      {showCreateGroup && (
        <div className="fixed inset-0 z-50 flex items-center justify-center">
          <div className="absolute inset-0 bg-black/50 backdrop-blur-sm" onClick={() => setShowCreateGroup(false)} />
          <div className="relative z-50 w-full max-w-md bg-[#1a1f36] border border-white/10 rounded-2xl p-6 shadow-2xl">
            <h2 className="text-white text-xl font-semibold mb-6">{t("admin.groups.create")}</h2>
            <div className="space-y-4">
              <div>
                <label className="text-white/60 text-sm mb-1 block">{t("admin.groups.name")}</label>
                <Input value={groupForm.name} onChange={(e) => setGroupForm({ ...groupForm, name: e.target.value })} placeholder="Bureautique" className="bg-[#2e3862]/80 border-white/10 text-white" />
              </div>
              <div>
                <label className="text-white/60 text-sm mb-1 block">{t("admin.groups.description")}</label>
                <Input value={groupForm.description} onChange={(e) => setGroupForm({ ...groupForm, description: e.target.value })} placeholder="Navigateurs et outils de bureau" className="bg-[#2e3862]/80 border-white/10 text-white" />
              </div>
              <div>
                <label className="text-white/60 text-sm mb-1 block">{t("admin.groups.color")}</label>
                <div className="flex gap-2">
                  {["#7096ff", "#65d5c5", "#ff6136", "#636fc1", "#4e549f", "#f54618"].map((c) => (
                    <button key={c} onClick={() => setGroupForm({ ...groupForm, color: c })}
                      className={`size-8 rounded-full border-2 transition-all ${groupForm.color === c ? "border-white scale-110" : "border-transparent"}`}
                      style={{ backgroundColor: c }} />
                  ))}
                </div>
              </div>
              <div className="border-t border-white/10 pt-4 mt-2">
                <p className="text-white/40 text-xs mb-3">Quotas (0 = unlimited)</p>
                <div className="grid grid-cols-3 gap-3">
                  <div>
                    <label className="text-white/60 text-xs mb-1 block">Max Sessions</label>
                    <Input type="number" min={0} value={groupForm.max_sessions} onChange={(e) => setGroupForm({ ...groupForm, max_sessions: parseInt(e.target.value) || 0 })} className="bg-[#2e3862]/80 border-white/10 text-white" />
                  </div>
                  <div>
                    <label className="text-white/60 text-xs mb-1 block">Max CPU</label>
                    <Input type="number" min={0} step={0.5} value={groupForm.max_cpu} onChange={(e) => setGroupForm({ ...groupForm, max_cpu: parseFloat(e.target.value) || 0 })} className="bg-[#2e3862]/80 border-white/10 text-white" />
                  </div>
                  <div>
                    <label className="text-white/60 text-xs mb-1 block">Max Memory MB</label>
                    <Input type="number" min={0} value={groupForm.max_memory} onChange={(e) => setGroupForm({ ...groupForm, max_memory: parseInt(e.target.value) || 0 })} className="bg-[#2e3862]/80 border-white/10 text-white" />
                  </div>
                </div>
              </div>
            </div>
            <div className="flex gap-3 mt-6">
              <button onClick={() => setShowCreateGroup(false)} className="flex-1 border border-white/10 text-white/70 hover:text-white hover:bg-white/10 rounded-lg py-2 text-sm transition-colors">{t("common.cancel")}</button>
              <Button onClick={handleCreateGroup} disabled={!groupForm.name} className="flex-1 bg-oklavier-blue hover:bg-oklavier-purple">{t("common.create")}</Button>
            </div>
          </div>
        </div>
      )}

      {/* Create Mapping Modal */}
      {showCreateMapping && (
        <div className="fixed inset-0 z-50 flex items-center justify-center">
          <div className="absolute inset-0 bg-black/50 backdrop-blur-sm" onClick={() => setShowCreateMapping(false)} />
          <div className="relative z-50 w-full max-w-md bg-[#1a1f36] border border-white/10 rounded-2xl p-6 shadow-2xl">
            <h2 className="text-white text-xl font-semibold mb-6">{t("admin.groups.mapping_title")}</h2>
            <div className="space-y-4">
              <div>
                <label className="text-white/60 text-sm mb-1 block">{t("admin.groups.oidc_role")}</label>
                <Input value={mappingForm.oidc_role} onChange={(e) => setMappingForm({ ...mappingForm, oidc_role: e.target.value })} placeholder="ROLE:EXAMPLE-GROUP" className="bg-[#2e3862]/80 border-white/10 text-white font-mono mb-2" />
                {discoveredRoles.length > 0 && (
                  <div>
                    <p className="text-white/30 text-xs mb-1.5">{t("admin.groups.discovered_roles")} :</p>
                    <div className="flex flex-wrap gap-1.5 max-h-32 overflow-y-auto">
                      {discoveredRoles
                        .filter(r => !mappings.some(m => m.oidc_role === r.role))
                        .map((r) => (
                        <button key={r.role}
                          onClick={() => setMappingForm({ ...mappingForm, oidc_role: r.role })}
                          className={`text-[11px] px-2 py-1 rounded-lg font-mono transition-colors ${
                            mappingForm.oidc_role === r.role
                              ? "bg-orange-400/20 text-orange-400 border border-orange-400/30"
                              : "bg-white/5 text-white/40 hover:text-white/60 border border-transparent"
                          }`}
                        >
                          {r.role} <span className="text-white/20">({r.user_count})</span>
                        </button>
                      ))}
                    </div>
                  </div>
                )}
                {discoveredRoles.length === 0 && (
                  <p className="text-white/20 text-xs">{t("admin.groups.no_discovered")}</p>
                )}
              </div>
              <div>
                <label className="text-white/60 text-sm mb-1 block">{t("admin.groups.select_group")}</label>
                <div className="space-y-1">
                  {allGroups.map((g) => (
                    <button key={g.id}
                      onClick={() => setMappingForm({ ...mappingForm, group_id: g.id })}
                      className={`w-full flex items-center gap-3 px-3 py-2 rounded-lg text-sm transition-colors ${
                        mappingForm.group_id === g.id ? "bg-oklavier-blue/20 text-white" : "text-white/50 hover:bg-white/5"
                      }`}
                    >
                      <div className="size-3 rounded-full" style={{ backgroundColor: g.color }} />
                      {g.name}
                    </button>
                  ))}
                </div>
              </div>
            </div>
            <div className="flex gap-3 mt-6">
              <button onClick={() => setShowCreateMapping(false)} className="flex-1 border border-white/10 text-white/70 hover:text-white hover:bg-white/10 rounded-lg py-2 text-sm transition-colors">{t("common.cancel")}</button>
              <Button onClick={handleCreateMapping} disabled={!mappingForm.oidc_role || !mappingForm.group_id} className="flex-1 bg-oklavier-blue hover:bg-oklavier-purple">{t("common.add")}</Button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
