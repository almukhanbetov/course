"use client";

import { useActionState } from "react";
import { rejectCourseSubmissionAction } from "@/lib/admin-actions";
import type { FormState } from "@/lib/actions";

const initialState: FormState = { error: null };

export function RejectCourseForm({ courseId }: { courseId: string }) {
  const action = rejectCourseSubmissionAction.bind(null, courseId);
  const [state, formAction, pending] = useActionState(action, initialState);

  return (
    <form action={formAction} className="admin-inline-actions">
      <input type="text" name="rejection_reason" placeholder="Причина отклонения" required />
      <button type="submit" className="btn-danger" disabled={pending}>
        {pending ? "..." : "Reject"}
      </button>
      {state.error && <span role="alert">{state.error}</span>}
    </form>
  );
}
