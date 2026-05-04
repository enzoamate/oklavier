"use client";

import { useState } from "react";
import { KeyRound, Mail, Shield, Globe, Server, Check, X, Loader2, Plus, Pencil, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useTranslation } from "@/lib/i18n";
import { useToast } from "@/components/toast";
import { ConfirmModal } from "@/components/confirm-modal";
import { useAPI, apiAction, invalidate } from "@/lib/api";

interface AuthMethod {
  id: string;
  type: "credentials" | "oidc";
  name: string;
  enabled: boolean;
  config: Record<string, string>;
}

interface FieldDef {
  key: string;
  labelKey: string;
  placeholder: string;
  type?: string;
}

export default function AuthMethodsPage() {
  const [showForm, setShowForm] = useState(false);
  const [formType, setFormType] = useState<"oidc" | null>(null);
  const [formConfig, setFormConfig] = useState<Record<string, string>>({});
  const [editing, setEditing] = useState<AuthMethod | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<string | null>(null);
  const [deleting, setDeleting] = useState(false);
  const { t } = useTranslation();
  const toast = useToast();

  const { data: methodsData, isLoading } = useAPI<{ methods: AuthMethod[] }>("/api/admin/auth-methods");
  const methods = methodsData?.methods || [
    { id: "credentials", type: "credentials" as const, name: "Email & Mot de passe", enabled: true, config: {} },
  ];

  const methodTypes = [
    { type: "credentials", labelKey: "admin.auth_methods.credentials", icon: Mail, descKey: "admin.auth_methods.credentials_desc", color: "text-green-400" },
    { type: "oidc", labelKey: "admin.auth_methods.oidc", icon: Globe, descKey: "admin.auth_methods.oidc_desc", color: "text-oklavier-blue" },
  ];

  const oidcFields: FieldDef[] = [
    { key: "client_id", labelKey: "admin.auth_methods.client_id", placeholder: "your-client-id" },
    { key: "client_secret", labelKey: "admin.auth_methods.client_secret", placeholder: "your-client-secret", type: "password" },
    { key: "issuer", labelKey: "admin.auth_methods.issuer_url", placeholder: "https://auth.example.com/realms/master" },
    { key: "display_name", labelKey: "admin.auth_methods.display_name", placeholder: "Se connecter avec Keycloak" },
    { key: "logo_url", labelKey: "admin.auth_methods.logo_url", placeholder: "https://..." },
  ];

  function getFields(type: string) {
    if (type === "oidc") return oidcFields;
    return [];
  }

  function openAdd(type: "oidc") {
    setEditing(null);
    setFormType(type);
    setFormConfig({});
    setShowForm(true);
  }

  function openEdit(method: AuthMethod) {
    setEditing(method);
    setFormType(method.type === "oidc" ? "oidc" : null);
    setFormConfig(method.config);
    setShowForm(true);
  }

  async function handleSave() {
    const res = await apiAction("/api/admin/auth-methods", editing ? "PUT" : "POST", { id: editing?.id, type: formType, config: formConfig });
    if (res.ok) {
      toast.success(editing ? t("admin.auth_methods.updated") : t("admin.auth_methods.created"));
    } else {
      toast.error(t("common.error"));
    }
    setShowForm(false);
    invalidate("/api/admin/auth-methods");
  }

  async function handleToggle(id: string, enabled: boolean) {
    const res = await apiAction("/api/admin/auth-methods", "PATCH", { id, enabled: !enabled });
    if (res.ok) {
      toast.success(t("admin.auth_methods.toggled"));
    } else {
      toast.error(t("common.error"));
    }
    invalidate("/api/admin/auth-methods");
  }

  function handleDelete(id: string) {
    setDeleteTarget(id);
  }

  async function confirmDelete() {
    if (!deleteTarget) return;
    setDeleting(true);
    const res = await apiAction("/api/admin/auth-methods", "DELETE", { id: deleteTarget });
    if (res.ok) {
      toast.success(t("admin.auth_methods.deleted"));
    } else {
      toast.error(t("common.error"));
    }
    invalidate("/api/admin/auth-methods");
    setDeleting(false);
    setDeleteTarget(null);
  }

  // Only show full-page loader on initial load (no data yet)
  if (!methodsData && isLoading) return <div className="flex items-center gap-2 text-white/60"><Loader2 className="size-5 animate-spin" /> {t("common.loading")}</div>;

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-white">{t("admin.auth_methods.title")}</h1>
          <p className="text-white/50 text-sm">{t("admin.auth_methods.subtitle")}</p>
        </div>
      </div>

      {/* Active methods */}
      <div className="space-y-3 mb-8">
        {methods.map((method) => {
          const typeInfo = methodTypes.find((ti) => ti.type === method.type);
          if (!typeInfo) return null;
          return (
            <div key={method.id} className="bg-[#1a1f36] border border-white/10 rounded-xl p-4 flex items-center gap-4">
              <div className={`size-10 rounded-lg bg-white/5 flex items-center justify-center ${typeInfo.color}`}>
                <typeInfo.icon className="size-5" />
              </div>
              <div className="flex-1">
                <div className="flex items-center gap-2">
                  <p className="text-white font-medium text-sm">{method.config?.display_name || t(typeInfo.labelKey)}</p>
                  <span className={`inline-flex items-center gap-1 text-[10px] px-1.5 py-0.5 rounded-full ${method.enabled ? "bg-green-500/10 text-green-400" : "bg-red-500/10 text-red-400"}`}>
                    {method.enabled ? <Check className="size-2.5" /> : <X className="size-2.5" />}
                    {method.enabled ? t("common.active") : t("common.disabled")}
                  </span>
                </div>
                <p className="text-white/40 text-xs">{t(typeInfo.descKey)}</p>
                {method.config?.issuer && <p className="text-oklavier-blue/60 text-xs mt-1">{method.config.issuer}</p>}
                {method.config?.url && <p className="text-orange-400/60 text-xs mt-1">{method.config.url}</p>}
              </div>
              <div className="flex items-center gap-1">
                {method.type !== "credentials" && (
                  <>
                    <button onClick={() => handleToggle(method.id, method.enabled)} className="p-2 rounded-lg text-white/40 hover:text-white hover:bg-white/10 transition-colors">
                      {method.enabled ? <X className="size-4" /> : <Check className="size-4" />}
                    </button>
                    <button onClick={() => openEdit(method)} className="p-2 rounded-lg text-white/40 hover:text-white hover:bg-white/10 transition-colors">
                      <Pencil className="size-4" />
                    </button>
                    <button onClick={() => handleDelete(method.id)} className="p-2 rounded-lg text-red-400/40 hover:text-red-400 hover:bg-red-500/10 transition-colors">
                      <Trash2 className="size-4" />
                    </button>
                  </>
                )}
              </div>
            </div>
          );
        })}
      </div>

      {/* Add new method */}
      <div>
        <h2 className="text-white/60 text-sm font-semibold mb-3">{t("admin.auth_methods.add_method")}</h2>
        <div className="grid grid-cols-3 gap-3">
          {methodTypes.filter((ti) => ti.type !== "credentials").map((type) => (
            <button
              key={type.type}
              onClick={() => openAdd("oidc")}
              className="bg-[#1a1f36] border border-white/10 rounded-xl p-4 text-left hover:bg-white/5 hover:border-white/20 transition-colors group"
            >
              <div className={`size-10 rounded-lg bg-white/5 flex items-center justify-center mb-3 ${type.color} group-hover:bg-white/10`}>
                <type.icon className="size-5" />
              </div>
              <p className="text-white text-sm font-medium">{t(type.labelKey)}</p>
              <p className="text-white/40 text-xs mt-1">{t(type.descKey)}</p>
              <div className="flex items-center gap-1 mt-3 text-oklavier-blue text-xs">
                <Plus className="size-3" /> {t("admin.auth_methods.configure")}
              </div>
            </button>
          ))}
        </div>
      </div>

      <ConfirmModal
        open={!!deleteTarget}
        title={t("admin.auth_methods.confirm_delete")}
        message={t("admin.auth_methods.confirm_delete")}
        variant="danger"
        loading={deleting}
        onConfirm={confirmDelete}
        onCancel={() => setDeleteTarget(null)}
      />

      {/* Config Modal */}
      {showForm && formType && (
        <div className="fixed inset-0 z-50 flex items-center justify-center">
          <div className="absolute inset-0 bg-black/50 backdrop-blur-sm" onClick={() => setShowForm(false)} />
          <div className="relative z-50 w-full max-w-lg bg-[#1a1f36] border border-white/10 rounded-2xl p-6 shadow-2xl max-h-[80vh] overflow-y-auto">
            <h2 className="text-white text-xl font-semibold mb-1">
              {editing ? t("common.edit") : t("admin.auth_methods.configure")} {t(methodTypes.find((ti) => ti.type === formType)?.labelKey || "")}
            </h2>
            <p className="text-white/40 text-sm mb-6">{t(methodTypes.find((ti) => ti.type === formType)?.descKey || "")}</p>

            <div className="space-y-4">
              {getFields(formType).map((field) => (
                <div key={field.key}>
                  <label className="text-white/60 text-sm mb-1 block">{t(field.labelKey)}</label>
                  {field.key === "certificate" ? (
                    <textarea
                      value={formConfig[field.key] || ""}
                      onChange={(e) => setFormConfig({ ...formConfig, [field.key]: e.target.value })}
                      placeholder={field.placeholder}
                      className="w-full h-24 bg-[#2e3862]/80 border border-white/10 rounded-lg text-white text-sm p-3 resize-none focus:outline-none focus:border-oklavier-blue"
                    />
                  ) : (
                    <Input
                      type={field.type || "text"}
                      value={formConfig[field.key] || ""}
                      onChange={(e) => setFormConfig({ ...formConfig, [field.key]: e.target.value })}
                      placeholder={field.placeholder}
                      className="bg-[#2e3862]/80 border-white/10 text-white"
                    />
                  )}
                </div>
              ))}
            </div>

            <div className="flex gap-3 mt-6">
              <Button onClick={() => setShowForm(false)} variant="outline" className="flex-1">{t("common.cancel")}</Button>
              <Button onClick={handleSave} className="flex-1 bg-oklavier-blue hover:bg-oklavier-purple">{editing ? t("common.save") : t("admin.auth_methods.activate")}</Button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
