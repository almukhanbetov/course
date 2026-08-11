import type { CodeSubmissionStatus } from "@/lib/api";

const LABELS: Record<CodeSubmissionStatus, string> = {
  queued: "В очереди…",
  running: "Выполняется…",
  passed: "Пройдено",
  failed: "Не пройдено",
  compile_error: "Ошибка компиляции",
  runtime_error: "Ошибка выполнения",
  timeout: "Превышено время выполнения",
  internal_error: "Ошибка сервера",
};

const CLASS_MAP: Record<CodeSubmissionStatus, string> = {
  queued: "badge-status-draft",
  running: "badge-status-pending_review",
  passed: "badge-status-published",
  failed: "badge-status-rejected",
  compile_error: "badge-status-rejected",
  runtime_error: "badge-status-rejected",
  timeout: "badge-status-rejected",
  internal_error: "badge-status-rejected",
};

export function CodeSubmissionStatusBadge({ status }: { status: CodeSubmissionStatus }) {
  return <span className={`badge ${CLASS_MAP[status] ?? "badge-status-draft"}`}>{LABELS[status] ?? status}</span>;
}
