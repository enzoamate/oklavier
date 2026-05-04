"use client";

import i18n from "i18next";
import { initReactI18next, useTranslation, I18nextProvider } from "react-i18next";
import LanguageDetector from "i18next-browser-languagedetector";

import fr from "../../messages/fr.json";
import en from "../../messages/en.json";
import es from "../../messages/es.json";
import de from "../../messages/de.json";

i18n
  .use(LanguageDetector)
  .use(initReactI18next)
  .init({
    showSupportNotice: false,
    resources: {
      fr: { translation: fr },
      en: { translation: en },
      es: { translation: es },
      de: { translation: de },
    },
    fallbackLng: "en",
    interpolation: {
      escapeValue: false,
    },
    detection: {
      order: ["cookie", "navigator"],
      lookupCookie: "oklavier_lang",
      caches: ["cookie"],
      cookieOptions: { path: "/", maxAge: 60 * 60 * 24 * 365, sameSite: "lax" },
    },
  });

export { i18n, I18nextProvider, useTranslation };

export function I18nProvider({ children }: { children: React.ReactNode }) {
  return <I18nextProvider i18n={i18n}>{children}</I18nextProvider>;
}
