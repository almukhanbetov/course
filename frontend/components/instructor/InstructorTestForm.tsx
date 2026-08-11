"use client";

import { useActionState } from "react";
import { createInstructorTestAction, updateInstructorTestAction } from "@/lib/instructor-actions";
import type { FormState } from "@/lib/actions";
import type { InstructorTestSummary } from "@/lib/instructor-api";

const initialState: FormState = { error: null };

// Course is fixed by the route (courseId), never a selector — an instructor
// can only ever attach a test to their own course.
export function InstructorTestForm({ courseId, test }: { courseId: string; test?: InstructorTestSummary }) {
  const action = test
    ? updateInstructorTestAction.bind(null, courseId, test.id)
    : createInstructorTestAction.bind(null, courseId);
  const [state, formAction, pending] = useActionState(action, initialState);

  return (
    <form action={formAction} className="admin-form">
      <label>
        Название
        <input type="text" name="title" defaultValue={test?.title} required />
      </label>
      <label>
        Проходной балл (%)
        <input type="number" name="passing_score" min={0} max={100} defaultValue={test?.passing_score ?? 70} />
      </label>
      <label className="admin-form-checkbox">
        <input type="checkbox" name="published" defaultChecked={test?.published} />
        Опубликован
      </label>
      <label className="admin-form-checkbox">
        <input type="checkbox" name="is_final" defaultChecked={test?.is_final} />
        Финальный тест (требует прохождения всех уроков)
      </label>
      {state.error && <p role="alert">{state.error}</p>}
      <button type="submit" className="btn-primary" disabled={pending}>
        {pending ? "Сохранение..." : "Сохранить"}
      </button>
    </form>
  );
}
