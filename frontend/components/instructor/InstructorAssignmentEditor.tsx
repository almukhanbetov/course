"use client";

import { useActionState } from "react";
import { createInstructorAssignmentAction, deleteInstructorAssignmentAction, updateInstructorAssignmentAction } from "@/lib/instructor-actions";
import { ConfirmButton } from "@/components/ConfirmButton";
import type { FormState } from "@/lib/actions";
import type { InstructorAssignment } from "@/lib/instructor-api";

const initialState: FormState = { error: null };

export function InstructorAssignmentEditor({
  courseId,
  lessonId,
  assignment,
}: {
  courseId: string;
  lessonId: string;
  assignment: InstructorAssignment | null;
}) {
  const action = assignment
    ? updateInstructorAssignmentAction.bind(null, courseId, assignment.id)
    : createInstructorAssignmentAction.bind(null, courseId, lessonId);
  const [state, formAction, pending] = useActionState(action, initialState);

  return (
    <div className="video-upload-block">
      <form action={formAction} className="admin-form">
        <label>
          Название задания
          <input type="text" name="title" defaultValue={assignment?.title} required />
        </label>
        <label>
          Описание
          <textarea name="description" rows={2} defaultValue={assignment?.description} />
        </label>
        <label>
          Инструкции для студента
          <textarea name="instructions" rows={3} defaultValue={assignment?.instructions} />
        </label>
        <label>
          Максимальный балл (необязательно)
          <input type="number" name="max_score" min={1} defaultValue={assignment?.max_score} />
        </label>
        <label className="admin-form-checkbox">
          <input type="checkbox" name="required" defaultChecked={assignment?.required ?? true} />
          Обязательное (влияет на завершение урока)
        </label>
        <label className="admin-form-checkbox">
          <input type="checkbox" name="published" defaultChecked={assignment?.published} />
          Опубликовано (видно студентам)
        </label>
        {state.error && <p role="alert">{state.error}</p>}
        <button type="submit" className="btn-small" disabled={pending}>
          {pending ? "..." : assignment ? "Сохранить задание" : "Создать задание"}
        </button>
      </form>
      {assignment && (
        <form action={deleteInstructorAssignmentAction.bind(null, courseId, assignment.id)} className="mt-3">
          <ConfirmButton
            className="btn-danger"
            confirmMessage="Удалить это задание? Возможно только если ещё нет отправленных решений."
          >
            Удалить задание
          </ConfirmButton>
        </form>
      )}
    </div>
  );
}
