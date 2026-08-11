"use client";

import { useActionState } from "react";
import { createQuestionAction, updateQuestionAction } from "@/lib/admin-actions";
import type { FormState } from "@/lib/actions";

const initialState: FormState = { error: null };

export function QuestionForm({
  testId,
  questionId,
  text,
}: {
  testId: string;
  questionId?: string;
  text?: string;
}) {
  const action = questionId
    ? updateQuestionAction.bind(null, testId, questionId)
    : createQuestionAction.bind(null, testId);
  const [state, formAction, pending] = useActionState(action, initialState);

  return (
    <form action={formAction} className="admin-inline-actions">
      <input type="text" name="text" defaultValue={text} placeholder="Question text" required />
      <button type="submit" className="btn-small" disabled={pending}>
        {pending ? "..." : questionId ? "Save" : "Add question"}
      </button>
      {state.error && <span role="alert">{state.error}</span>}
    </form>
  );
}
