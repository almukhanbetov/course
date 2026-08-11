const LABELS: Record<string, string> = {
  draft: "Черновик",
  submitted: "На проверке",
  needs_revision: "Требует доработки",
  approved: "Принято",
};

// Reuses the same badge-status-* classes as course publication status
// (Stage 14) — draft/pending_review/published/rejected map cleanly onto
// draft/submitted/needs_revision/approved's meaning and color intent.
const CLASS_MAP: Record<string, string> = {
  draft: "badge-status-draft",
  submitted: "badge-status-pending_review",
  needs_revision: "badge-status-rejected",
  approved: "badge-status-published",
};

export function SubmissionStatusBadge({ status }: { status: string }) {
  return <span className={`badge ${CLASS_MAP[status] ?? "badge-status-draft"}`}>{LABELS[status] ?? status}</span>;
}
