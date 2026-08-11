"use client";

import { useActionState } from "react";
import { createModuleAction, updateModuleAction } from "@/lib/admin-actions";
import type { FormState } from "@/lib/actions";

const initialState: FormState = { error: null };

export function ModuleForm({
  courseId,
  moduleId,
  title,
}: {
  courseId: string;
  moduleId?: string;
  title?: string;
}) {
  const action = moduleId
    ? updateModuleAction.bind(null, courseId, moduleId)
    : createModuleAction.bind(null, courseId);
  const [state, formAction, pending] = useActionState(action, initialState);

  return (
    <form action={formAction} className="admin-inline-actions">
      <input type="text" name="title" defaultValue={title} placeholder="Module title" required />
      <button type="submit" className="btn-small" disabled={pending}>
        {pending ? "..." : moduleId ? "Save" : "Add module"}
      </button>
      {state.error && <span role="alert">{state.error}</span>}
    </form>
  );
}
