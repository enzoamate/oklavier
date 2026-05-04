import { LoginForm } from "@/components/login-form"
import { LangSwitcher } from "@/components/lang-switcher"
import { LoginLogo, LoginRightPanel } from "@/components/login-branding"

export default function LoginPage() {
  return (
    <div className="grid min-h-svh lg:grid-cols-2">
      <div className="flex flex-col gap-4 p-6 md:p-10">
        <div className="flex justify-between items-center">
          <LoginLogo />
          <LangSwitcher variant="light" />
        </div>
        <div className="flex flex-1 items-center justify-center">
          <div className="w-full max-w-xs">
            <LoginForm />
          </div>
        </div>
      </div>
      <div className="relative hidden bg-muted lg:block overflow-hidden">
        <LoginRightPanel />
      </div>
    </div>
  )
}
