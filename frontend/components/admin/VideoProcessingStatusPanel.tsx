"use client";

import { useEffect, useState } from "react";
import { getVideoProcessingStatusAction } from "@/lib/admin-actions";
import type { AttemptStatus, VideoProcessingStatus } from "@/lib/admin-api";

// Item 19: "Frontend может polling, например каждые несколько секунд" —
// no websocket, just a plain interval that stops once there's nothing left
// in flight to watch.
const POLL_MS = 4_000;

function statusLabel(status: string): string {
  switch (status) {
    case "uploaded":
      return "Video uploaded";
    case "processing":
      return "Processing…";
    case "ready":
      return "Ready";
    case "failed":
      return "Failed";
    default:
      return status;
  }
}

function isInFlight(attempt?: AttemptStatus): boolean {
  return attempt?.status === "uploaded" || attempt?.status === "processing";
}

function AttemptBlock({ label, attempt }: { label: string; attempt: AttemptStatus }) {
  return (
    <div className="my-2">
      <strong>
        {label}: {statusLabel(attempt.status)}
      </strong>
      {attempt.status === "failed" && attempt.error && (
        <p role="alert" className="my-course-meta">
          {attempt.error}
        </p>
      )}
      {attempt.renditions.length > 0 && (
        <ul className="lesson-list">
          {attempt.renditions.map((r) => (
            <li key={r.quality}>
              {r.quality}: {statusLabel(r.status)}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

export function VideoProcessingStatusPanel({ lessonId }: { lessonId: string }) {
  const [status, setStatus] = useState<VideoProcessingStatus | null | undefined>(undefined);

  useEffect(() => {
    let cancelled = false;
    let timer: ReturnType<typeof setTimeout> | undefined;

    async function tick() {
      const result = await getVideoProcessingStatusAction(lessonId);
      if (cancelled) return;
      const next = result.status ?? null;
      setStatus(next);
      if (isInFlight(next?.active) || isInFlight(next?.pending)) {
        timer = setTimeout(tick, POLL_MS);
      }
    }

    void tick();
    return () => {
      cancelled = true;
      if (timer) clearTimeout(timer);
    };
  }, [lessonId]);

  if (!status) return null;

  return (
    <div className="video-status-panel">
      {status.active && <AttemptBlock label="Active" attempt={status.active} />}
      {status.pending && <AttemptBlock label="Replacement in progress" attempt={status.pending} />}
    </div>
  );
}
