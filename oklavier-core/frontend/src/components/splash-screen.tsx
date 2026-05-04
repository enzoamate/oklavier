"use client";

import { motion, AnimatePresence } from "framer-motion";
import { useBranding } from "@/lib/use-branding";
import { OklavierLogo } from "@/components/oklavier-logo";

export function SplashScreen({
  show,
  logo,
}: {
  show: boolean;
  logo?: React.ReactNode;
}) {
  const { branding } = useBranding();

  const resolvedLogo = logo || (branding.logo_url ? (
    <img src={branding.logo_url} alt={branding.app_name} style={{ width: 64, height: 64, objectFit: "contain" }} />
  ) : (
    <OklavierLogo className="w-20 h-20" gradient={{ id: "splash-grad", from: "#7096ff", to: "#65d5c5" }} />
  ));

  return (
    <AnimatePresence>
      {show && (
        <motion.div
          initial={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          transition={{ duration: 0.5, ease: "easeInOut" }}
          className="fixed inset-0 z-[9999] flex items-center justify-center bg-[#0f1225]"
        >
          <div className="relative inline-flex items-center justify-center" style={{ width: 120, height: 120 }}>
            {/* Logo pulse */}
            <motion.div
              animate={{ scale: [1, 0.9, 0.9, 1, 1], opacity: [1, 0.48, 0.48, 1, 1] }}
              transition={{ duration: 2, repeatDelay: 1, repeat: Infinity, ease: "easeInOut" }}
            >
              {resolvedLogo}
            </motion.div>

            {/* Primary outline - rotating square to circle */}
            <motion.span
              className="absolute border-[3px] border-oklavier-blue/25"
              style={{ width: "calc(100% - 20px)", height: "calc(100% - 20px)" }}
              animate={{
                scale: [1.6, 1, 1, 1.6, 1.6],
                rotate: [270, 0, 0, 270, 270],
                opacity: [0.25, 1, 1, 1, 0.25],
                borderRadius: ["25%", "25%", "50%", "50%", "25%"],
              }}
              transition={{ ease: "linear", duration: 3.2, repeat: Infinity }}
            />

            {/* Secondary outline */}
            <motion.span
              className="absolute w-full h-full border-[8px] border-oklavier-blue/25"
              animate={{
                scale: [1, 1.2, 1.2, 1, 1],
                rotate: [0, 270, 270, 0, 0],
                opacity: [1, 0.25, 0.25, 0.25, 1],
                borderRadius: ["25%", "25%", "50%", "50%", "25%"],
              }}
              transition={{ ease: "linear", duration: 3.2, repeat: Infinity }}
            />
          </div>
        </motion.div>
      )}
    </AnimatePresence>
  );
}

export function PageTransition({ children }: { children: React.ReactNode }) {
  return (
    <motion.div
      initial={{ opacity: 0, y: 10 }}
      animate={{ opacity: 1, y: 0 }}
      exit={{ opacity: 0, y: -10 }}
      transition={{ duration: 0.3, ease: "easeInOut" }}
    >
      {children}
    </motion.div>
  );
}
