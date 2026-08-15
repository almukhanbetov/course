"use client";

import { useCallback, useSyncExternalStore } from "react";
import { IconMoon, IconSun } from "@/components/shell/icons";

type Theme = "light" | "dark";

// useSyncExternalStore, not useState+useEffect: the actual state lives on
// <html data-theme> (set pre-hydration by the blocking script in
// RootLayout, see app/layout.tsx), not in this component — this is exactly
// the "read external mutable state, stay SSR-safe" case the hook exists
// for. getServerSnapshot fixes the SSR/first-paint value to "dark" (the
// only theme the server ever renders), and React reconciles against the
// real DOM value right after hydration.
const listeners = new Set<() => void>();

function subscribe(onStoreChange: () => void) {
  listeners.add(onStoreChange);
  return () => listeners.delete(onStoreChange);
}

function getSnapshot(): Theme {
  return document.documentElement.getAttribute("data-theme") === "light" ? "light" : "dark";
}

function getServerSnapshot(): Theme {
  return "dark";
}

function applyTheme(next: Theme) {
  document.documentElement.setAttribute("data-theme", next);
  try {
    localStorage.setItem("theme", next);
  } catch {
    // Private browsing / storage disabled — the toggle still works for
    // the current tab, it just won't be remembered next visit.
  }
  listeners.forEach((notify) => notify());
}

export function ThemeToggle() {
  const theme = useSyncExternalStore(subscribe, getSnapshot, getServerSnapshot);

  const toggle = useCallback(() => {
    applyTheme(theme === "light" ? "dark" : "light");
  }, [theme]);

  return (
    <button
      type="button"
      onClick={toggle}
      className="theme-toggle-btn"
      aria-label={theme === "light" ? "Включить тёмную тему" : "Включить светлую тему"}
    >
      {theme === "light" ? <IconMoon size={17} /> : <IconSun size={17} />}
    </button>
  );
}
