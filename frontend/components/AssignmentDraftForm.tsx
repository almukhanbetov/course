"use client";

import { useActionState } from "react";
import { saveAssignmentDraftAction, submitAssignmentAction } from "@/lib/actions";
import type { FormState } from "@/lib/actions";

const initialState: FormState = { error: null };

export function AssignmentDraftForm({
  courseId,
  lessonId,
  assignmentId,
  textContent,
}: {
  courseId: string;
  lessonId: string;
  assignmentId: string;
  textContent?: string;
}) {
  const saveAction = saveAssignmentDraftAction.bind(null, courseId, lessonId, assignmentId);
  const [saveState, saveFormAction, savePending] = useActionState(saveAction, initialState);

  const submitAction = submitAssignmentAction.bind(null, courseId, lessonId, assignmentId);
  const [submitState, submitFormAction, submitPending] = useActionState(submitAction, initialState);

  return (
    <div>
      <form action={saveFormAction} className="admin-form">
        <label>
          Решение
          <textarea name="text_content" rows={6} defaultValue={textContent} placeholder="Опишите ваше решение..." />
        </label>
        {saveState.error && <p role="alert">{saveState.error}</p>}
        <button type="submit" className="btn-secondary" disabled={savePending}>
          {savePending ? "Сохранение..." : "Сохранить черновик"}
        </button>
      </form>

      <form action={submitFormAction} className="mt-3">
        {submitState.error && <p role="alert">{submitState.error}</p>}
        <button type="submit" className="btn-primary" disabled={submitPending}>
          {submitPending ? "Отправка..." : "Отправить на проверку"}
        </button>
      </form>
    </div>
  );
}
