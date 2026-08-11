"use client";

import { useActionState } from "react";
import { addCourseToSpecialityAction } from "@/lib/admin-actions";
import type { FormState } from "@/lib/actions";
import type { Course } from "@/lib/api";

const initialState: FormState = { error: null };

export function AddCourseToRoadmapForm({ specialityId, courses }: { specialityId: string; courses: Course[] }) {
  const action = addCourseToSpecialityAction.bind(null, specialityId);
  const [state, formAction, pending] = useActionState(action, initialState);

  return (
    <form action={formAction} className="admin-inline-actions">
      <select name="course_id" required>
        {courses.map((course) => (
          <option key={course.id} value={course.id}>
            {course.title}
          </option>
        ))}
      </select>
      <label className="admin-form-checkbox">
        <input type="checkbox" name="required" defaultChecked />
        Required
      </label>
      <button type="submit" className="btn-small" disabled={pending}>
        {pending ? "..." : "Add"}
      </button>
      {state.error && <span role="alert">{state.error}</span>}
    </form>
  );
}
