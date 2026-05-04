"use client";

import { useBranding } from "@/lib/use-branding";
import { OklavierLogo } from "@/components/oklavier-logo";

export function LoginLogo() {
  const { branding, isLoading } = useBranding();
  if (isLoading) return <div className="h-8" />;

  return (
    <a href="#" className="flex items-center gap-2 font-semibold text-lg">
      {branding.logo_url ? (
        <img src={branding.logo_url} alt={branding.app_name} className="h-8 max-w-[200px] object-contain" />
      ) : (
        <>
          <OklavierLogo className="size-7" gradient={{ id: "lp1", from: "#4e549f", to: "#636fc1" }} />
          {branding.app_name}
        </>
      )}
    </a>
  );
}

export function LoginTagline() {
  const { branding, isLoading } = useBranding();
  if (isLoading) return null;
  const tagline = branding.creator ? `${branding.creator} | By eamate` : "By eamate";
  return <p className="text-sm opacity-60">{tagline}</p>;
}

export function LoginRightPanel() {
  const { branding, isLoading } = useBranding();
  if (isLoading) {
    return <div className="absolute inset-0 bg-gradient-to-br from-[#2e3862] via-[#4e549f] to-[#636fc1]" />;
  }

  // Custom background: replace everything
  if (branding.login_bg) {
    const isSvg = branding.login_bg.startsWith("data:image/svg") || branding.login_bg.endsWith(".svg");
    return (
      <>
        <div className="absolute inset-0 bg-gradient-to-br from-[#2e3862] via-[#4e549f] to-[#636fc1]" />
        {isSvg ? (
          <div className="absolute inset-0 flex items-center justify-center p-16">
            <img src={branding.login_bg} alt="" className="w-full h-full object-contain opacity-80" />
          </div>
        ) : (
          <img src={branding.login_bg} alt="" className="absolute inset-0 w-full h-full object-cover" />
        )}
        <div className="absolute bottom-10 left-10 right-10 text-white z-20">
          <LoginTagline />
        </div>
      </>
    );
  }

  // Default: gradient + decorative logo
  return (
    <>
      <div className="absolute inset-0 bg-gradient-to-br from-[#2e3862] via-[#4e549f] to-[#636fc1]" />
      <div className="absolute inset-0 flex items-center justify-center">
        <OklavierLogo className="w-64 h-64 opacity-10" fill="white" />
      </div>
      <div className="absolute bottom-10 left-10 right-10 text-white z-20">
        <LoginTagline />
      </div>
    </>
  );
}
