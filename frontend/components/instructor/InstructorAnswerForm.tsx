"use client";

import { useActionState } from "react";
import { createInstructorAnswerAction } from "@/lib/instructor-actions";
import type { FormState } from "@/lib/actions";

const initialState: FormState = { error: null };

export function InstructorAnswerForm({ courseId, testId, questionId }: { courseId: string; testId: string; questionId: string }) {
  const action = createInstructorAnswerAction.bind(null, courseId, testId, questionId);
  const [state, formAction, pending] = useActionState(action, initialState);

  return (
    <form action={formAction} className="admin-inline-actions">
      <input type="text" name="text" placeholder="Текст ответа" required />
      <label className="admin-form-checkbox">
        <input type="checkbox" name="is_correct" />
        Правильный
      </label>
      <button type="submit" className="btn-small" disabled={pending}>
        {pending ? "..." : "Добавить ответ"}
      </button>
      {state.error && <span role="alert">{state.error}</span>}
    </form>
  );
}
