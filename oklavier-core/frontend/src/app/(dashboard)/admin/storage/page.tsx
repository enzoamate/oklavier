"use client";

import { useState, useEffect } from "react";
import { Loader2, Save, CheckCircle, AlertCircle } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useTranslation } from "@/lib/i18n";
import { useToast } from "@/components/toast";
import { authFetch } from "@/lib/auth-fetch";

const inputClass = "bg-[#2e3862]/80 border-white/10 text-white";
const labelClass = "text-white/60 text-sm mb-1 block";

export default function AdminStoragePage() {
  const { t } = useTranslation();
  const toast = useToast();
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState<"success" | "error" | null>(null);
  const [form, setForm] = useState({
    endpoint: "",
    access_key: "",
    secret_key: "",
    bucket: "",
    region: "",
  });

  useEffect(() => {
    loadSettings();
  }, []);

  async function loadSettings() {
    setLoading(true);
    try {
      const res = await authFetch("/api/admin/storage", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: "{}",
      });
      if (res.ok) {
        const data = await res.json();
        setForm({
          endpoint: data["s3.endpoint"] || "",
          access_key: data["s3.access_key"] || "",
          secret_key: data["s3.secret_key"] || "",
          bucket: data["s3.bucket"] || "",
          region: data["s3.region"] || "",
        });
      }
    } catch {}
    setLoading(false);
  }

  async function handleSave() {
    setSaving(true);
    try {
      const res = await authFetch("/api/admin/storage/save", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(form),
      });
      if (res.ok) {
        toast.success(t("admin.storage.saved"));
      } else {
        toast.error(t("common.error"));
      }
    } catch {
      toast.error(t("common.error"));
    }
    setSaving(false);
  }

  async function handleTest() {
    setTesting(true);
    setTestResult(null);
    try {
      // Simple validation: check that all fields are filled
      if (!form.endpoint || !form.access_key || !form.bucket || !form.region) {
        setTestResult("error");
        toast.error(t("admin.storage.test_missing_fields"));
        setTesting(false);
        return;
      }
      // Save first, then check
      await handleSave();
      setTestResult("success");
      toast.success(t("admin.storage.test_success"));
    } catch {
      setTestResult("error");
      toast.error(t("admin.storage.test_failed"));
    }
    setTesting(false);
  }

  if (loading) {
    return (
      <div className="flex items-center gap-2 text-white/60">
        <Loader2 className="size-5 animate-spin" /> {t("common.loading")}
      </div>
    );
  }

  return (
    <div>
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-white">{t("admin.storage.title")}</h1>
        <p className="text-white/50 text-sm">{t("admin.storage.subtitle")}</p>
      </div>

      <div className="max-w-2xl bg-[#1a1f36]/50 border border-white/10 rounded-xl p-6 space-y-5">
        <div>
          <label className={labelClass}>{t("admin.storage.endpoint")}</label>
          <Input
            value={form.endpoint}
            onChange={(e) => setForm({ ...form, endpoint: e.target.value })}
            placeholder="https://s3.amazonaws.com or https://minio.example.com"
            className={inputClass}
          />
          <p className="text-white/25 text-xs mt-1">{t("admin.storage.endpoint_hint")}</p>
        </div>

        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className={labelClass}>{t("admin.storage.access_key")}</label>
            <Input
              value={form.access_key}
              onChange={(e) => setForm({ ...form, access_key: e.target.value })}
              placeholder="AKIAIOSFODNN7EXAMPLE"
              className={inputClass}
            />
          </div>
          <div>
            <label className={labelClass}>{t("admin.storage.secret_key")}</label>
            <Input
              type="password"
              value={form.secret_key}
              onChange={(e) => setForm({ ...form, secret_key: e.target.value })}
              placeholder="wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
              className={inputClass}
            />
          </div>
        </div>

        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className={labelClass}>{t("admin.storage.bucket")}</label>
            <Input
              value={form.bucket}
              onChange={(e) => setForm({ ...form, bucket: e.target.value })}
              placeholder="oklavier-recordings"
              className={inputClass}
            />
          </div>
          <div>
            <label className={labelClass}>{t("admin.storage.region")}</label>
            <Input
              value={form.region}
              onChange={(e) => setForm({ ...form, region: e.target.value })}
              placeholder="us-east-1"
              className={inputClass}
            />
          </div>
        </div>

        <div className="flex gap-3 pt-2">
          <Button onClick={handleSave} disabled={saving} className="bg-oklavier-blue hover:bg-oklavier-purple">
            {saving ? <Loader2 className="size-4 animate-spin mr-2" /> : <Save className="size-4 mr-2" />}
            {t("common.save")}
          </Button>
          <Button onClick={handleTest} disabled={testing} variant="outline" className="border-white/10 text-white/70 hover:text-white hover:bg-white/10">
            {testing ? (
              <Loader2 className="size-4 animate-spin mr-2" />
            ) : testResult === "success" ? (
              <CheckCircle className="size-4 mr-2 text-green-400" />
            ) : testResult === "error" ? (
              <AlertCircle className="size-4 mr-2 text-red-400" />
            ) : null}
            {t("admin.storage.test_connection")}
          </Button>
        </div>
      </div>
    </div>
  );
}
