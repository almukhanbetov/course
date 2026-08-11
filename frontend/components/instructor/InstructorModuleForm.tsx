"use client";

import { useActionState } from "react";
import { createInstructorModuleAction, updateInstructorModuleAction } from "@/lib/instructor-actions";
import type { FormState } from "@/lib/actions";

const initialState: FormState = { error: null };

export function InstructorModuleForm({
  courseId,
  moduleId,
  title,
}: {
  courseId: string;
  moduleId?: string;
  title?: string;
}) {
  const action = moduleId
    ? updateInstructorModuleAction.bind(null, courseId, moduleId)
    : createInstructorModuleAction.bind(null, courseId);
  const [state, formAction, pending] = useActionState(action, initialState);

  return (
    <form action={formAction} className="admin-inline-actions">
      <input type="text" name="title" defaultValue={title} placeholder="Название модуля" required />
      <button type="submit" className="btn-small" disabled={pending}>
        {pending ? "..." : moduleId ? "Сохранить" : "Добавить модуль"}
      </button>
      {state.error && <span role="alert">{state.error}</span>}
    </form>
  );
}
