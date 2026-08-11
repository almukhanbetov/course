"use client";

import { useActionState } from "react";
import { createInstructorQuestionAction, updateInstructorQuestionAction } from "@/lib/instructor-actions";
import type { FormState } from "@/lib/actions";

const initialState: FormState = { error: null };

export function InstructorQuestionForm({
  courseId,
  testId,
  questionId,
  text,
}: {
  courseId: string;
  testId: string;
  questionId?: string;
  text?: string;
}) {
  const action = questionId
    ? updateInstructorQuestionAction.bind(null, courseId, testId, questionId)
    : createInstructorQuestionAction.bind(null, courseId, testId);
  const [state, formAction, pending] = useActionState(action, initialState);

  return (
    <form action={formAction} className="admin-inline-actions">
      <input type="text" name="text" defaultValue={text} placeholder="Текст вопроса" required />
      <button type="submit" className="btn-small" disabled={pending}>
        {pending ? "..." : questionId ? "Сохранить" : "Добавить вопрос"}
      </button>
      {state.error && <span role="alert">{state.error}</span>}
    </form>
  );
}
