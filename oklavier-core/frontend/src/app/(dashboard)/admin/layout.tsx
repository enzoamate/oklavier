"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { Users, Play, ArrowLeft, Image, Shield, KeyRound, UsersRound, Loader2, Server, ScrollText, LayoutDashboard, User, Palette, HardDrive, Video, Activity, Link as LinkIcon, Download } from "lucide-react";
import { useSession, signOut } from "@/lib/auth-client";
import { useEffect } from "react";
import { LangSwitcher } from "@/components/lang-switcher";
import { useTranslation } from "@/lib/i18n";
import { useBranding } from "@/lib/use-branding";
import { OklavierLogo } from "@/components/oklavier-logo";

function useNavSections(t: (key: string) => string) {
  return [
    {
      label: "",
      items: [
        { href: "/admin", label: t("admin.dashboard.title"), icon: LayoutDashboard },
      ],
    },
    {
      label: t("admin.infrastructure"),
      items: [
        { href: "/admin/workspaces", label: t("admin.workspaces.title"), icon: Image },
        { href: "/admin/sessions", label: t("admin.sessions.title"), icon: Play },
        { href: "/admin/agents", label: t("admin.agents.title"), icon: Server },
        { href: "/admin/recordings", label: t("admin.recordings.title"), icon: Video },
      ],
    },
    {
      label: t("admin.access_management"),
      items: [
        { href: "/admin/users", label: t("admin.users.title"), icon: Users },
        { href: "/admin/groups", label: t("admin.groups.title"), icon: UsersRound },
        { href: "/admin/auth-methods", label: t("admin.auth_methods.title"), icon: KeyRound },
        { href: "/admin/guest-links", label: t("admin.guest_links.title"), icon: LinkIcon },
      ],
    },
    {
      label: t("admin.customization"),
      items: [
        { href: "/admin/branding", label: t("admin.branding.title"), icon: Palette },
        { href: "/admin/storage", label: t("admin.storage.title"), icon: HardDrive },
        { href: "/admin/backup", label: t("admin.backup.title"), icon: Download },
      ],
    },
    {
      label: t("admin.debug"),
      items: [
        { href: "/admin/health", label: t("admin.health.title"), icon: Activity },
        { href: "/admin/logs", label: t("admin.logs.title"), icon: ScrollText },
        { href: "/admin/audit", label: t("admin.audit.title"), icon: Shield },
      ],
    },
  ];
}

export default function AdminLayout({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const { data: session, isPending } = useSession();

  useEffect(() => {
    if (!isPending && (!session?.user || ((session.user as any).role) !== "admin")) {
      router.push("/workspaces");
    }
  }, [session, isPending, router]);

  const { t } = useTranslation();
  const { branding } = useBranding();
  const navSections = useNavSections(t);

  if (isPending) {
    return <div className="min-h-screen bg-[#0f1225] flex items-center justify-center"><Loader2 className="size-8 animate-spin text-oklavier-blue" /></div>;
  }

  if (!session?.user || ((session.user as any).role) !== "admin") return null;

  return (
    <div className="min-h-screen bg-[#0f1225] dark">
      {/* Top bar */}
      <div className="h-14 bg-[#1a1f36] border-b border-white/10 flex items-center justify-between px-6">
        <div className="flex items-center gap-4">
          {branding.logo_url ? (
            <img src={branding.logo_url} alt={branding.app_name} className="h-7" />
          ) : (
            <OklavierLogo className="size-7" gradient={{ id: "ag1", from: "#7096ff", to: "#65d5c5" }} />
          )}
          <span className="text-white font-semibold">{branding.app_name}</span>
          <span className="text-white/30">|</span>
          <span className="text-white/50 text-sm">{t("admin.title")}</span>
        </div>
        <div className="flex items-center gap-4">
          <Link href="/workspaces" className="flex items-center gap-2 text-white/60 hover:text-white text-sm transition-colors">
            <ArrowLeft className="size-4" />
            {t("admin.back_to_spaces")}
          </Link>
          <LangSwitcher variant="dark" />
          <div className="h-4 w-px bg-white/10" />
          <Link href="/profile" className="flex items-center gap-1 text-white/40 hover:text-white text-sm transition-colors">
            <User className="size-4" />
            {t("profile.title")}
          </Link>
          <div className="h-4 w-px bg-white/10" />
          <button
            onClick={async () => { await signOut(); window.location.href = "/login"; }}
            className="text-white/40 hover:text-white text-sm transition-colors"
          >
            {t("auth.logout")}
          </button>
        </div>
      </div>

      <div className="flex">
        {/* Sidebar */}
        <div className="w-64 min-h-[calc(100vh-3.5rem)] bg-[#1a1f36]/50 border-r border-white/10 py-4">
          {navSections.map((section, idx) => (
            <div key={section.label || idx} className="mb-6">
              {section.label && <p className="text-[10px] uppercase tracking-widest text-white/30 font-semibold px-6 mb-2">{section.label}</p>}
              <nav className="space-y-0.5 px-3">
                {section.items.map((item) => {
                  const active = item.href === "/admin" ? pathname === "/admin" : pathname.startsWith(item.href);
                  return (
                    <Link
                      key={item.href}
                      href={item.href}
                      className={`flex items-center gap-3 px-3 py-2 rounded-lg text-sm transition-colors ${
                        active
                          ? "bg-oklavier-blue/20 text-white"
                          : "text-white/50 hover:text-white hover:bg-white/5"
                      }`}
                    >
                      <item.icon className={`size-4 ${active ? "text-oklavier-blue" : ""}`} />
                      {item.label}
                    </Link>
                  );
                })}
              </nav>
            </div>
          ))}
          {branding.version && (
            <div className="px-6 mt-auto pt-4">
              <p className="text-[10px] text-white/15">v{branding.version}</p>
            </div>
          )}
        </div>

        {/* Content */}
        <div className="flex-1 p-6">
          {children}
        </div>
      </div>
    </div>
  );
}
