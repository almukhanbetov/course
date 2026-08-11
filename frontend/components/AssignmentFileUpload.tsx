"use client";

import { useActionState } from "react";
import { uploadAssignmentFileAction } from "@/lib/actions";
import type { FormState } from "@/lib/actions";

const initialState: FormState = { error: null };

export function AssignmentFileUpload({
  courseId,
  lessonId,
  assignmentId,
}: {
  courseId: string;
  lessonId: string;
  assignmentId: string;
}) {
  const action = uploadAssignmentFileAction.bind(null, courseId, lessonId, assignmentId);
  const [state, formAction, pending] = useActionState(action, initialState);

  return (
    <form action={formAction} className="admin-inline-actions my-2">
      <input type="file" name="file" accept=".pdf,.txt,.zip,.png,.jpg,.jpeg" required />
      <button type="submit" className="btn-small" disabled={pending}>
        {pending ? "Загрузка…" : "Прикрепить файл"}
      </button>
      {state.error && <p role="alert">{state.error}</p>}
    </form>
  );
}
