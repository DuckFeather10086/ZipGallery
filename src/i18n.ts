import { useSyncExternalStore } from "react";
import zh from "./locales/zh-CN.json";
import en from "./locales/en.json";
export type Locale = "zh-CN" | "en";
const dictionaries: Record<Locale, Record<keyof typeof zh, string>> = {
  "zh-CN": zh,
  en,
};
let locale: Locale =
  localStorage.getItem("zipgallery.locale") === "en" ? "en" : "zh-CN";
const listeners = new Set<() => void>();
export function setLocale(next: Locale) {
  locale = next;
  localStorage.setItem("zipgallery.locale", next);
  listeners.forEach((f) => f());
}
export function useLocale() {
  return useSyncExternalStore(
    (f) => {
      listeners.add(f);
      return () => listeners.delete(f);
    },
    () => locale,
  );
}
export function t(
  key: keyof typeof zh,
  vars: Record<string, string | number> = {},
) {
  const singular = `${key}_one` as keyof typeof zh;
  let text =
    vars.count === 1 && singular in dictionaries[locale]
      ? dictionaries[locale][singular]
      : dictionaries[locale][key];
  for (const [k, v] of Object.entries(vars))
    text = text.replaceAll(`{${k}}`, String(v));
  return text;
}
export function errorMessage(error: unknown) {
  const raw = error instanceof Error ? error.message : String(error),
    match = raw.match(/E_[A-Z_]+/);
  if (match && match[0] in zh) {
    const key = match[0] as keyof typeof zh;
    const detail = raw.slice(raw.indexOf(key) + key.length).trim();
    return t(key) + (detail ? " " + detail : "");
  }
  return raw;
}
