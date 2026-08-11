import { submitCourseForReviewAction } from "@/lib/instructor-actions";
import { PublicationBadge } from "@/components/instructor/PublicationBadge";
import { ConfirmButton } from "@/components/ConfirmButton";
import type { Course } from "@/lib/api";

// Deliberately no "Publish" button here at all — production publish is
// admin/moderation-only (see Stage 14 item 5). An instructor's only lever
// is submitting a draft/rejected course into the review queue.
export function PublicationPanel({ course }: { course: Course }) {
  const canSubmit = course.publication_status === "draft" || course.publication_status === "rejected";

  return (
    <div className="admin-card">
      <h2>Публикация</h2>
      <p>
        Статус: <PublicationBadge status={course.publication_status} />
      </p>

      {course.publication_status === "rejected" && course.rejection_reason && (
        <p role="alert">Причина отклонения: {course.rejection_reason}</p>
      )}

      {course.publication_status === "pending_review" && (
        <p className="subtitle">Курс отправлен на проверку администратору. Ожидайте решения.</p>
      )}

      {course.publication_status === "published" && (
        <p className="subtitle">Курс опубликован и виден студентам в каталоге.</p>
      )}

      {canSubmit && (
        <form action={submitCourseForReviewAction.bind(null, course.id)}>
          <ConfirmButton
            className="btn-primary"
            confirmMessage="Отправить курс на проверку администратору? После этого редактирование останется доступным, но повторно отправить курс можно будет только если его отклонят."
          >
            Отправить на проверку
          </ConfirmButton>
        </form>
      )}
    </div>
  );
}
