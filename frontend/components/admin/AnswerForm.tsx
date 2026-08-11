"use client";

import { useActionState } from "react";
import { createAnswerAction } from "@/lib/admin-actions";
import type { FormState } from "@/lib/actions";

const initialState: FormState = { error: null };

export function AnswerForm({ testId, questionId }: { testId: string; questionId: string }) {
  const action = createAnswerAction.bind(null, testId, questionId);
  const [state, formAction, pending] = useActionState(action, initialState);

  return (
    <form action={formAction} className="admin-inline-actions">
      <input type="text" name="text" placeholder="Answer text" required />
      <label className="admin-form-checkbox">
        <input type="checkbox" name="is_correct" />
        Correct
      </label>
      <button type="submit" className="btn-small" disabled={pending}>
        {pending ? "..." : "Add answer"}
      </button>
      {state.error && <span role="alert">{state.error}</span>}
    </form>
  );
}
