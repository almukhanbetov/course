"use client";

import { useState } from "react";
import { getSubmissionFileDownloadUrlAction } from "@/lib/actions";
import type { SubmissionFile } from "@/lib/api";

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

export function SubmissionFileDownloadLink({ file }: { file: SubmissionFile }) {
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  async function handleClick() {
    setLoading(true);
    setError(null);
    const result = await getSubmissionFileDownloadUrlAction(file.id);
    setLoading(false);
    if (result.error || !result.url) {
      setError(result.error ?? "Не удалось получить файл");
      return;
    }
    window.open(result.url, "_blank", "noopener,noreferrer");
  }

  return (
    <span>
      <button type="button" className="btn-small" onClick={handleClick} disabled={loading}>
        {loading ? "..." : `📎 ${file.original_filename}`} ({formatBytes(file.size_bytes)})
      </button>
      {error && <span role="alert"> {error}</span>}
    </span>
  );
}
