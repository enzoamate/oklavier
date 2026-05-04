"use client";

import { useState, useEffect } from "react";
import { Palette, Save, Loader2 } from "lucide-react";
import { useTranslation } from "@/lib/i18n";
import { useToast } from "@/components/toast";
import { useAPI, apiAction } from "@/lib/api";
import { mutate as globalMutate } from "swr";
import { ImageUpload } from "@/components/image-upload";

export default function BrandingPage() {
  const { t } = useTranslation();
  const toast = useToast();
  const { data, mutate } = useAPI<any>("/api/admin/branding");
  const [form, setForm] = useState({
    app_name: "Oklavier",
    logo_url: "",
    favicon_url: "",
    creator: "",
    creator_url: "",
    primary_color: "#4F46E5",
    accent_color: "#F97316",
    login_bg: "",
  });
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (data) {
      setForm({
        app_name: data.app_name || "Oklavier",
        logo_url: data.logo_url || "",
        favicon_url: data.favicon_url || "",
        creator: data.creator || "",
        creator_url: data.creator_url || "",
        primary_color: data.primary_color || "#4F46E5",
        accent_color: data.accent_color || "#F97316",
        login_bg: data.login_bg || "",
      });
    }
  }, [data]);

  async function handleSave() {
    setSaving(true);
    const res = await apiAction("/api/admin/branding", "POST", form);
    if (res.ok) {
      toast.success(t("admin.branding.saved"));
      mutate();
      globalMutate("/api/branding");
    } else {
      toast.error(t("common.error"));
    }
    setSaving(false);
  }

  return (
    <div>
      <div className="mb-6">
        <h1 className="text-xl font-bold text-white flex items-center gap-2">
          <Palette className="size-5 text-oklavier-blue" />
          {t("admin.branding.title")}
        </h1>
        <p className="text-white/40 text-sm">{t("admin.branding.subtitle")}</p>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* General */}
        <div className="bg-white/5 border border-white/10 rounded-xl p-6 space-y-4">
          <h2 className="text-white font-medium text-sm uppercase tracking-wider">{t("admin.branding.general")}</h2>

          <div>
            <label className="text-white/60 text-sm mb-1 block">{t("admin.branding.app_name")}</label>
            <input value={form.app_name} onChange={e => setForm({...form, app_name: e.target.value})}
              className="w-full bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-white text-sm" />
          </div>

          <div>
            <label className="text-white/60 text-sm mb-1 block">{t("admin.branding.creator")}</label>
            <input value={form.creator} onChange={e => setForm({...form, creator: e.target.value})}
              placeholder="Your Company"
              className="w-full bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-white text-sm placeholder-white/20" />
            <p className="text-white/20 text-xs mt-1">{form.creator ? `${form.creator} | By eamate` : "By eamate"}</p>
          </div>

        </div>

        {/* Logos */}
        <div className="bg-white/5 border border-white/10 rounded-xl p-6 space-y-4">
          <h2 className="text-white font-medium text-sm uppercase tracking-wider">{t("admin.branding.logos")}</h2>
          <ImageUpload value={form.logo_url} onChange={v => setForm({...form, logo_url: v})} label="Logo" hint="SVG or 200x40px" />
          <ImageUpload value={form.favicon_url} onChange={v => setForm({...form, favicon_url: v})} label={t("admin.branding.favicon_url")} hint="SVG or 32x32px" previewHeight="h-8" />
        </div>

        {/* Colors */}
        <div className="bg-white/5 border border-white/10 rounded-xl p-6 space-y-4">
          <h2 className="text-white font-medium text-sm uppercase tracking-wider">{t("admin.branding.colors")}</h2>

          <div className="flex gap-4">
            <div className="flex-1">
              <label className="text-white/60 text-sm mb-1 block">{t("admin.branding.primary_color")}</label>
              <div className="flex items-center gap-2">
                <input type="color" value={form.primary_color} onChange={e => setForm({...form, primary_color: e.target.value})}
                  className="h-10 w-14 rounded cursor-pointer bg-transparent border-0" />
                <input value={form.primary_color} onChange={e => setForm({...form, primary_color: e.target.value})}
                  className="flex-1 bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-white text-sm font-mono" />
              </div>
            </div>
            <div className="flex-1">
              <label className="text-white/60 text-sm mb-1 block">{t("admin.branding.accent_color")}</label>
              <div className="flex items-center gap-2">
                <input type="color" value={form.accent_color} onChange={e => setForm({...form, accent_color: e.target.value})}
                  className="h-10 w-14 rounded cursor-pointer bg-transparent border-0" />
                <input value={form.accent_color} onChange={e => setForm({...form, accent_color: e.target.value})}
                  className="flex-1 bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-white text-sm font-mono" />
              </div>
            </div>
          </div>

          {/* Preview */}
          <div className="mt-4 p-4 bg-black/20 rounded-lg">
            <p className="text-white/40 text-xs mb-2">{t("admin.branding.preview")}</p>
            <div className="flex items-center gap-3">
              <div className="w-8 h-8 rounded" style={{ backgroundColor: form.primary_color }} />
              <div className="w-8 h-8 rounded" style={{ backgroundColor: form.accent_color }} />
              <span className="text-white text-sm font-medium">{form.app_name}</span>
              <span className="text-white/30 text-xs">— {form.creator ? `${form.creator} | By eamate` : "By eamate"}</span>
            </div>
          </div>
        </div>
        {/* Login Background */}
        <div className="bg-white/5 border border-white/10 rounded-xl p-6 space-y-4">
          <h2 className="text-white font-medium text-sm uppercase tracking-wider">{t("admin.branding.login_bg")}</h2>
          <ImageUpload value={form.login_bg} onChange={v => setForm({...form, login_bg: v})} label={t("admin.branding.login_bg_label")} hint="SVG or 1920x1080px" previewHeight="h-24" />
        </div>
      </div>

      <div className="mt-6">
        <button onClick={handleSave} disabled={saving}
          className="flex items-center gap-2 px-6 py-2.5 bg-oklavier-blue hover:bg-oklavier-purple rounded-lg text-white text-sm transition-colors disabled:opacity-50">
          {saving ? <Loader2 className="size-4 animate-spin" /> : <Save className="size-4" />}
          {t("common.save")}
        </button>
      </div>
    </div>
  );
}
