"use client";

import { useState, useEffect } from "react";
import { useSession } from "@/lib/auth-client";
import { useTranslation } from "@/lib/i18n";
import { useToast } from "@/components/toast";
import { User, Lock, Save, Loader2, ArrowLeft } from "lucide-react";
import Link from "next/link";
import { authFetch } from "@/lib/auth-fetch";

export default function ProfilePage() {
  const { data: session, mutate } = useSession();
  const { t } = useTranslation();
  const toast = useToast();
  const user = session?.user;

  const [name, setName] = useState("");
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [savingName, setSavingName] = useState(false);
  const [savingPassword, setSavingPassword] = useState(false);

  // Initialize name from session
  useEffect(() => {
    if (user?.name) setName(user.name);
  }, [user?.name]);

  async function handleUpdateName() {
    setSavingName(true);
    try {
      const res = await authFetch("/api/auth/update-profile", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name }),
      });
      if (res.ok) {
        toast.success(t("profile.name_updated"));
        mutate();
      } else {
        const data = await res.json();
        toast.error(data.error || t("common.error"));
      }
    } catch {
      toast.error(t("common.error"));
    }
    setSavingName(false);
  }

  async function handleChangePassword() {
    if (newPassword !== confirmPassword) {
      toast.error(t("profile.passwords_mismatch"));
      return;
    }
    setSavingPassword(true);
    try {
      const res = await authFetch("/api/auth/change-password", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ current_password: currentPassword, new_password: newPassword }),
      });
      if (res.ok) {
        toast.success(t("profile.password_changed"));
        setCurrentPassword("");
        setNewPassword("");
        setConfirmPassword("");
      } else {
        const data = await res.json();
        toast.error(data.error || t("common.error"));
      }
    } catch {
      toast.error(t("common.error"));
    }
    setSavingPassword(false);
  }

  if (!user) return null;

  return (
    <div className="min-h-screen bg-[#0f1225] p-8">
      <div className="max-w-2xl mx-auto space-y-6">
        <div className="flex items-center gap-4">
          <Link href="/workspaces" className="text-white/40 hover:text-white transition-colors">
            <ArrowLeft className="size-5" />
          </Link>
          <h1 className="text-xl font-bold text-white flex items-center gap-2">
            <User className="size-5 text-oklavier-blue" />
            {t("profile.title")}
          </h1>
        </div>

        {/* Info card */}
        <div className="bg-white/5 border border-white/10 rounded-xl p-6 space-y-4">
          <div className="flex items-center gap-4">
            <div className="size-16 rounded-full bg-oklavier-blue/20 flex items-center justify-center text-2xl font-bold text-oklavier-blue">
              {user.name?.[0]?.toUpperCase() || "?"}
            </div>
            <div>
              <p className="text-white font-medium">{user.name}</p>
              <p className="text-white/40 text-sm">{user.email}</p>
              <span
                className={`text-xs px-2 py-0.5 rounded-full ${
                  (user as any).role === "admin"
                    ? "bg-oklavier-blue/20 text-oklavier-blue"
                    : "bg-white/10 text-white/50"
                }`}
              >
                {(user as any).role || "user"}
              </span>
            </div>
          </div>
        </div>

        {/* Edit name */}
        <div className="bg-white/5 border border-white/10 rounded-xl p-6 space-y-4">
          <h2 className="text-white font-medium">{t("profile.edit_name")}</h2>
          <input
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            className="w-full bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-white text-sm focus:outline-none focus:border-oklavier-blue/50"
          />
          <button
            onClick={handleUpdateName}
            disabled={savingName}
            className="flex items-center gap-2 px-4 py-2 bg-oklavier-blue hover:bg-oklavier-purple rounded-lg text-white text-sm transition-colors disabled:opacity-50"
          >
            {savingName ? <Loader2 className="size-4 animate-spin" /> : <Save className="size-4" />}
            {t("common.save")}
          </button>
        </div>

        {/* Change password */}
        <div className="bg-white/5 border border-white/10 rounded-xl p-6 space-y-4">
          <h2 className="text-white font-medium flex items-center gap-2">
            <Lock className="size-4" />
            {t("profile.change_password")}
          </h2>
          <input
            type="password"
            placeholder={t("profile.current_password")}
            value={currentPassword}
            onChange={(e) => setCurrentPassword(e.target.value)}
            className="w-full bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-white text-sm focus:outline-none focus:border-oklavier-blue/50"
          />
          <input
            type="password"
            placeholder={t("profile.new_password")}
            value={newPassword}
            onChange={(e) => setNewPassword(e.target.value)}
            className="w-full bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-white text-sm focus:outline-none focus:border-oklavier-blue/50"
          />
          <input
            type="password"
            placeholder={t("profile.confirm_password")}
            value={confirmPassword}
            onChange={(e) => setConfirmPassword(e.target.value)}
            className="w-full bg-white/5 border border-white/10 rounded-lg px-3 py-2 text-white text-sm focus:outline-none focus:border-oklavier-blue/50"
          />
          <button
            onClick={handleChangePassword}
            disabled={savingPassword || !currentPassword || !newPassword}
            className="flex items-center gap-2 px-4 py-2 bg-oklavier-blue hover:bg-oklavier-purple rounded-lg text-white text-sm transition-colors disabled:opacity-50"
          >
            {savingPassword ? <Loader2 className="size-4 animate-spin" /> : <Lock className="size-4" />}
            {t("profile.change_password")}
          </button>
        </div>
      </div>
    </div>
  );
}
