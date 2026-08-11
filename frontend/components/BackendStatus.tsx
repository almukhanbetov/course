"use client";

import { useEffect, useState } from "react";
import { getHealth, type HealthStatus } from "@/lib/api";

type State =
  | { kind: "loading" }
  | { kind: "ready"; data: HealthStatus }
  | { kind: "error"; message: string };

export function BackendStatus() {
  const [state, setState] = useState<State>({ kind: "loading" });

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
    return <p>Проверка соединения с backend...</p>;
  }

  if (state.kind === "error") {
    return <p role="alert">Backend недоступен: {state.message}</p>;
  }

  return (
    <p>
      Backend: {state.data.status} · База данных: {state.data.database}
    </p>
  );
}
