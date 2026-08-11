import type { Course } from "@/lib/api";

const LABELS: Record<Course["publication_status"], string> = {
  draft: "Черновик",
  pending_review: "На проверке",
  published: "Опубликован",
  rejected: "Отклонён",
};

export function PublicationBadge({ status }: { status: Course["publication_status"] }) {
  return <span className={`badge badge-status-${status}`}>{LABELS[status]}</span>;
}
