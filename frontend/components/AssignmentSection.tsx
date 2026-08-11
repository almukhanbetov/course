import type { StudentAssignment, Submission } from "@/lib/api";
import { SubmissionStatusBadge } from "@/components/SubmissionStatusBadge";
import { SubmissionFileDownloadLink } from "@/components/SubmissionFileDownloadLink";
import { AssignmentDraftForm } from "@/components/AssignmentDraftForm";
import { AssignmentFileUpload } from "@/components/AssignmentFileUpload";

const EDITABLE_STATUSES = new Set(["draft", "needs_revision"]);

export function AssignmentSection({
  courseId,
  lessonId,
  assignment,
  submission,
}: {
  courseId: string;
  lessonId: string;
  assignment: StudentAssignment;
  submission: Submission | null;
}) {
  const isEditable = !submission || EDITABLE_STATUSES.has(submission.status);

  return (
    <section className="admin-card">
      <h2>Практическое задание</h2>
      <h3>{assignment.title}</h3>
      {assignment.description && <p>{assignment.description}</p>}
      {assignment.instructions && <p className="my-course-meta">{assignment.instructions}</p>}
      {assignment.required && <p className="my-course-meta">Обязательное задание{assignment.max_score ? ` · максимум ${assignment.max_score} баллов` : ""}</p>}

      {submission && (
        <div className="my-2">
          <SubmissionStatusBadge status={submission.status} />
          {submission.score != null && <span className="my-course-meta"> Оценка: {submission.score}</span>}
        </div>
      )}

      {submission?.status === "needs_revision" && submission.instructor_feedback && (
        <p role="alert">Комментарий преподавателя: {submission.instructor_feedback}</p>
      )}
      {submission?.status === "approved" && submission.instructor_feedback && (
        <p className="subtitle">Комментарий преподавателя: {submission.instructor_feedback}</p>
      )}

      {submission && submission.files.length > 0 && (
        <div className="my-2">
          <p className="my-course-meta">Прикреплённые файлы:</p>
          <div className="admin-inline-actions">
            {submission.files.map((f) => (
              <SubmissionFileDownloadLink key={f.id} file={f} />
            ))}
          </div>
        </div>
      )}

      {isEditable ? (
        <>
          {submission?.status === "needs_revision" && (
            <p className="subtitle">Исправьте решение по комментарию преподавателя и отправьте задание снова.</p>
          )}
          <AssignmentFileUpload courseId={courseId} lessonId={lessonId} assignmentId={assignment.id} />
          <AssignmentDraftForm
            courseId={courseId}
            lessonId={lessonId}
            assignmentId={assignment.id}
            textContent={submission?.text_content}
          />
        </>
      ) : (
        <p className="subtitle">
          {submission?.status === "submitted" ? "Задание отправлено и ожидает проверки." : "Задание проверено и принято."}
        </p>
      )}
    </section>
  );
}
