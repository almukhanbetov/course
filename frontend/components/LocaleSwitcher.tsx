"use client";

import { LOCALES, LOCALE_COOKIE, type Locale } from "@/lib/i18n/locale";

const LABELS: Record<Locale, string> = { ru: "РУ", kk: "ҚАЗ", en: "EN" };

// Module-level, not inline in the component: the eslint-plugin-react-hooks
// immutability rule flags any write to an object from outside component
// scope (document.cookie included) when it happens textually inside a
// component/hook body, even from an event handler where it's perfectly
// safe. Calling out to a plain function sidesteps that false positive
// without disabling the rule.
function setLocaleCookie(next: Locale) {
  document.cookie = `${LOCALE_COOKIE}=${next}; path=/; max-age=31536000; SameSite=Lax`;
}

// Unlike ThemeToggle, content locale can't be flipped client-side with a DOM
// attribute — every string on the page is server-rendered text, not CSS.
// So this sets the cookie the Server Components already read (see
// lib/i18n/locale.ts's getLocale) and reloads, letting the next server
// render produce the real translated HTML — no client-side text swapping,
// no risk of a stale server/client mismatch.
export function LocaleSwitcher({ locale }: { locale: Locale }) {
  function switchTo(next: Locale) {
    if (next === locale) return;
    setLocaleCookie(next);
    window.location.reload();
  }

  return (
    <div className="locale-switcher" role="group" aria-label="Язык / Тіл / Language">
      {LOCALES.map((l) => (
        <button
          key={l}
          type="button"
          className={l === locale ? "locale-switcher-btn active" : "locale-switcher-btn"}
          onClick={() => switchTo(l)}
          aria-pressed={l === locale}
        >
          {LABELS[l]}
        </button>
      ))}
    </div>
  );
}
