"use client";

import { useActionState } from "react";
import { reviewSubmissionAction } from "@/lib/instructor-actions";
import type { FormState } from "@/lib/actions";
import type { SubmissionDetail } from "@/lib/instructor-api";

const initialState: FormState = { error: null };

export function ReviewForm({ submission }: { submission: SubmissionDetail }) {
  const action = reviewSubmissionAction.bind(null, submission.id);
  const [state, formAction, pending] = useActionState(action, initialState);

  return (
    <form action={formAction} className="admin-form">
      <label>
        Решение
        <select name="status" defaultValue="approved">
          <option value="approved">Принять</option>
          <option value="needs_revision">Отправить на доработку</option>
        </select>
      </label>
      <label>
        Балл{submission.max_score ? ` (0–${submission.max_score})` : ""}
        <input type="number" name="score" min={0} max={submission.max_score} defaultValue={submission.score} />
      </label>
      <label>
        Комментарий
        <textarea name="feedback" rows={4} defaultValue={submission.instructor_feedback} />
      </label>
      {state.error && <p role="alert">{state.error}</p>}
      <button type="submit" className="btn-primary" disabled={pending}>
        {pending ? "Сохранение..." : "Сохранить проверку"}
      </button>
    </form>
  );
}
