"use client";

import { useState } from "react";
import { Globe } from "lucide-react";
import { useTranslation } from "@/lib/i18n";

const languages = [
  { code: "fr", flag: "🇫🇷" },
  { code: "en", flag: "🇬🇧" },
  { code: "es", flag: "🇪🇸" },
  { code: "de", flag: "🇩🇪" },
];

export function LangSwitcher({ variant = "light" }: { variant?: "light" | "dark" }) {
  const [open, setOpen] = useState(false);
  const { t, i18n } = useTranslation();

  const isDark = variant === "dark";

  return (
    <div className="relative">
      <button
        onClick={() => setOpen(!open)}
        className={`size-9 rounded-full flex items-center justify-center transition-colors ${
          isDark
            ? "backdrop-blur-sm bg-white/10 text-white/80 hover:text-white hover:bg-white/20"
            : "bg-muted text-muted-foreground hover:bg-accent hover:text-foreground"
        }`}
      >
        <Globe className="size-4" />
      </button>
      {open && (
        <>
          <div className="fixed inset-0 z-40" onClick={() => setOpen(false)} />
          <div className={`absolute right-0 top-12 z-50 w-44 border rounded-xl shadow-2xl overflow-hidden ${
            isDark
              ? "backdrop-blur-xl bg-[#1a1f36]/95 border-white/10"
              : "bg-popover border-border"
          }`}>
            {languages.map((l) => (
              <button
                key={l.code}
                onClick={() => { i18n.changeLanguage(l.code); setOpen(false); }}
                className={`w-full flex items-center gap-3 px-4 py-2.5 text-sm transition-colors ${
                  i18n.language === l.code
                    ? isDark ? "bg-oklavier-blue/20 text-white" : "bg-accent text-foreground"
                    : isDark ? "text-white/70 hover:bg-white/5 hover:text-white" : "text-muted-foreground hover:bg-accent hover:text-foreground"
                }`}
              >
                <span className="text-base">{l.flag}</span>
                <span>{t(`lang.${l.code}`)}</span>
              </button>
            ))}
          </div>
        </>
      )}
    </div>
  );
}
