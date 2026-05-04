"use client";

import { useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Field, FieldGroup, FieldLabel, FieldSeparator } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Loader2 } from "lucide-react";
import { signIn } from "@/lib/auth-client";
import { SplashScreen } from "@/components/splash-screen";
import { useTranslation } from "@/lib/i18n";

interface OIDCProvider {
  id: string;
  name: string;
  logo?: string;
}

export function LoginForm({ className, ...props }: React.ComponentProps<"form">) {
  const router = useRouter();
  const { t } = useTranslation();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const [splash, setSplash] = useState(false);
  const [oidcProviders, setOidcProviders] = useState<OIDCProvider[]>([]);

  useEffect(() => {
    fetch("/api/auth/providers")
      .then((r) => r.json())
      .then((data) => setOidcProviders(Array.isArray(data) ? data : data.providers || []))
      .catch(() => {});
  }, []);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setLoading(true);
    setError("");
    try {
      await signIn(email, password);
      setSplash(true);
      setTimeout(() => router.push("/workspaces"), 600);
    } catch (err) {
      setError(err instanceof Error ? err.message : t("auth.invalid_credentials"));
      setLoading(false);
    }
  }

  async function handleOIDC(providerId: string) {
    setSplash(true);
    // Redirect to Go API OIDC login endpoint
    window.location.assign(`/api/auth/oidc/${providerId}?callbackURL=/workspaces`);
  }

  const hasSSO = oidcProviders.length > 0;

  return (
    <>
      <SplashScreen show={splash} />
      <form className={cn("flex flex-col gap-6", className)} onSubmit={handleSubmit} {...props}>
        <FieldGroup>
          <div className="flex flex-col items-center gap-1 text-center">
            <h1 className="text-2xl font-bold">{t("auth.login_title")}</h1>
            <p className="text-sm text-balance text-muted-foreground">{t("auth.login_subtitle")}</p>
          </div>

          {error && (
            <div className="text-sm text-destructive bg-destructive/10 p-3 rounded-md text-center">{error}</div>
          )}

          {/* SSO Buttons */}
          {hasSSO && (
            <>
              {oidcProviders.map((provider) => (
                <Field key={provider.id}>
                  <Button
                    variant="outline"
                    type="button"
                    className="w-full"
                    onClick={() => handleOIDC(provider.id)}
                  >
                    {provider.logo && (
                      <img src={provider.logo} alt="" className="h-4 w-4 mr-2" />
                    )}
                    {t("auth.connect_with", { provider: provider.name })}
                  </Button>
                </Field>
              ))}
              <FieldSeparator>{t("auth.or_continue_with")}</FieldSeparator>
            </>
          )}

          {/* Local auth */}
          <Field>
            <FieldLabel htmlFor="email">{t("auth.email")}</FieldLabel>
            <Input id="email" type="email" placeholder={t("auth.email_placeholder")} value={email} onChange={(e) => setEmail(e.target.value)} required />
          </Field>
          <Field>
            <FieldLabel htmlFor="password">{t("auth.password")}</FieldLabel>
            <Input id="password" type="password" value={password} onChange={(e) => setPassword(e.target.value)} required />
          </Field>
          <Field>
            <Button type="submit" className="bg-oklavier-orange hover:bg-[#f54618]" disabled={loading}>
              {loading ? <Loader2 className="h-4 w-4 animate-spin" /> : t("auth.login_button")}
            </Button>
          </Field>
        </FieldGroup>
      </form>
    </>
  );
}
