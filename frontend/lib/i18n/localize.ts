import type { LocalizedDescription, LocalizedTitle } from "@/lib/api";
import type { Locale } from "@/lib/i18n/locale";

// Every localize* helper falls back to the always-present Russian field
// whenever the requested locale has no translation yet (new content created
// after a translation was added, or simply never translated) — never an
// empty string, never an error.
export function localizeTitle(entity: LocalizedTitle & { title: string }, locale: Locale): string {
  if (locale === "kk") return entity.title_kk || entity.title;
  if (locale === "en") return entity.title_en || entity.title;
  return entity.title;
}

export function localizeDescription(entity: LocalizedDescription & { description: string }, locale: Locale): string {
  if (locale === "kk") return entity.description_kk || entity.description;
  if (locale === "en") return entity.description_en || entity.description;
  return entity.description;
}

export function localizeName(entity: { name: string; name_kk?: string; name_en?: string }, locale: Locale): string {
  if (locale === "kk") return entity.name_kk || entity.name;
  if (locale === "en") return entity.name_en || entity.name;
  return entity.name;
}
