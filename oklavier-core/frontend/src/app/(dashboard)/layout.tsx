"use client";

import { useState, useEffect, useRef } from "react";
import { usePathname } from "next/navigation";
import { SplashScreen } from "@/components/splash-screen";
import { ToastProvider } from "@/components/toast";
import { AuthGuard } from "@/components/auth-guard";

function getSection(path: string) {
  if (path.startsWith("/admin")) return "admin";
  if (path.startsWith("/sessions/")) return "session";
  return "user";
}

export default function DashboardLayout({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const [showSplash, setShowSplash] = useState(true); // Show on initial load (F5)
  const prevSection = useRef(getSection(pathname));
  const initialLoad = useRef(true);

  // Hide splash once page is mounted (initial load)
  useEffect(() => {
    if (initialLoad.current) {
      initialLoad.current = false;
      const timer = setTimeout(() => setShowSplash(false), 600);
      return () => clearTimeout(timer);
    }
  }, []);

  // Show splash on admin <-> user transitions
  useEffect(() => {
    if (initialLoad.current) return;
    const currentSection = getSection(pathname);
    if (
      prevSection.current !== currentSection &&
      currentSection !== "session" &&
      prevSection.current !== "session"
    ) {
      setShowSplash(true);
      const timer = setTimeout(() => setShowSplash(false), 500);
      prevSection.current = currentSection;
      return () => clearTimeout(timer);
    }
    prevSection.current = currentSection;
  }, [pathname]);

  return (
    <ToastProvider>
      <SplashScreen show={showSplash} />
      <AuthGuard>
        {children}
      </AuthGuard>
    </ToastProvider>
  );
}
