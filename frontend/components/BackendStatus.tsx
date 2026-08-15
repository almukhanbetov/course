"use client";

import { useEffect, useState } from "react";
import { getHealth, type HealthStatus } from "@/lib/api";
import { getDictionary } from "@/lib/i18n/dictionaries";
import type { Locale } from "@/lib/i18n/locale";

type State =
  | { kind: "loading" }
  | { kind: "ready"; data: HealthStatus }
  | { kind: "error"; message: string };

export function BackendStatus({ locale }: { locale: Locale }) {
  const [state, setState] = useState<State>({ kind: "loading" });
  const dict = getDictionary(locale).backendStatus;

  useEffect(() => {
    let cancelled = false;

    getHealth()
      .then((data) => {
        if (!cancelled) setState({ kind: "ready", data });
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setState({
            kind: "error",
            message: err instanceof Error ? err.message : "Unknown error",
          });
        }
      });

    return () => {
      cancelled = true;
    };
  }, []);

  if (state.kind === "loading") {
    return <p>{dict.checking}</p>;
  }

  if (state.kind === "error") {
    return <p role="alert">{dict.unavailable(state.message)}</p>;
  }

  return <p>{dict.ready(state.data.status, state.data.database)}</p>;
}
