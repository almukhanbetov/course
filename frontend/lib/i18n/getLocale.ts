import "server-only";
import { cookies } from "next/headers";
import { DEFAULT_LOCALE, isLocale, LOCALE_COOKIE, type Locale } from "@/lib/i18n/locale";

// Split out of locale.ts specifically so that file can stay import-safe for
// Client Components (RecommendationCard, imported from the dashboard's
// client-side PersonalizedRecommendations, needs the Locale type/constants
// but must never pull next/headers into a client bundle — Next.js build
// fails hard if it does). The "server-only" import makes any accidental
// client-side import of *this* file fail loudly at build time instead.
//
// Server Components can't read localStorage, and content locale (unlike
// the theme toggle's CSS attribute) has to be correct in the very first
// server-rendered HTML — so it's a cookie, read here on every public page/
// layout that needs it. LocaleSwitcher (client) sets this same cookie and
// reloads the page so the next server render picks it up.
export async function getLocale(): Promise<Locale> {
  const store = await cookies();
  const value = store.get(LOCALE_COOKIE)?.value;
  return isLocale(value) ? value : DEFAULT_LOCALE;
}
