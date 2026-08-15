"use client";

import { useEffect, useRef, useState } from "react";
import { LOCALES, LOCALE_COOKIE, type Locale } from "@/lib/i18n/locale";
import { IconChevronDown, IconGlobe } from "@/components/shell/icons";

const SHORT_LABELS: Record<Locale, string> = { ru: "RU", kk: "ҚАЗ", en: "EN" };
const FULL_LABELS: Record<Locale, string> = { ru: "Русский", kk: "Қазақша", en: "English" };

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
// lib/i18n/getLocale.ts) and reloads, letting the next server render
// produce the real translated HTML — no client-side text swapping, no risk
// of a stale server/client mismatch.
export function LocaleSwitcher({ locale }: { locale: Locale }) {
  const [open, setOpen] = useState(false);
  const wrapperRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function handleOutsideClick(e: MouseEvent) {
      if (wrapperRef.current && !wrapperRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    }
    function handleEscape(e: KeyboardEvent) {
      if (e.key === "Escape") setOpen(false);
    }
    document.addEventListener("mousedown", handleOutsideClick);
    document.addEventListener("keydown", handleEscape);
    return () => {
      document.removeEventListener("mousedown", handleOutsideClick);
      document.removeEventListener("keydown", handleEscape);
    };
  }, []);

  function switchTo(next: Locale) {
    setOpen(false);
    if (next === locale) return;
    setLocaleCookie(next);
    window.location.reload();
  }

  return (
    <div className="locale-switcher" ref={wrapperRef}>
      <button
        type="button"
        className="locale-switcher-trigger"
        onClick={() => setOpen((v) => !v)}
        aria-haspopup="menu"
        aria-expanded={open}
        aria-label="Язык / Тіл / Language"
      >
        <IconGlobe size={16} />
        {SHORT_LABELS[locale]}
        <IconChevronDown size={13} className={open ? "locale-switcher-chevron open" : "locale-switcher-chevron"} />
      </button>
      {open && (
        <ul className="locale-switcher-menu" role="menu">
          {LOCALES.map((l) => (
            <li key={l} role="none">
              <button
                type="button"
                role="menuitemradio"
                aria-checked={l === locale}
                className={l === locale ? "locale-switcher-item active" : "locale-switcher-item"}
                onClick={() => switchTo(l)}
              >
                {FULL_LABELS[l]}
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
