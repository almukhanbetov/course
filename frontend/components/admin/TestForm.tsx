"use client";

import { useActionState } from "react";
import { createTestAction, updateTestAction } from "@/lib/admin-actions";
import type { FormState } from "@/lib/actions";
import type { Course } from "@/lib/api";
import type { AdminTestSummary } from "@/lib/admin-api";

const initialState: FormState = { error: null };

export function TestForm({ test, courses }: { test?: AdminTestSummary; courses: Course[] }) {
  const action = test ? updateTestAction.bind(null, test.id) : createTestAction;
  const [state, formAction, pending] = useActionState(action, initialState);

  return (
    <form action={formAction} className="admin-form">
      <label>
        Course
        <select name="course_id" defaultValue={test?.course_id ?? ""} required>
          <option value="" disabled>
            Select a course
          </option>
          {courses.map((course) => (
            <option key={course.id} value={course.id}>
              {course.title}
            </option>
          ))}
        </select>
      </label>
      <label>
        Title
        <input type="text" name="title" defaultValue={test?.title} required />
      </label>
      <label>
        Description
        <textarea name="description" rows={3} defaultValue={test?.description} />
      </label>
      <label>
        Passing score (%)
        <input type="number" name="passing_score" min={0} max={100} defaultValue={test?.passing_score ?? 70} />
      </label>
      <label className="admin-form-checkbox">
        <input type="checkbox" name="published" defaultChecked={test?.published} />
        Published
      </label>
      <label className="admin-form-checkbox">
        <input type="checkbox" name="is_final" defaultChecked={test?.is_final} />
        Final test (gated on all lessons completed)
      </label>
      {state.error && <p role="alert">{state.error}</p>}
      <button type="submit" className="btn-primary" disabled={pending}>
        {pending ? "Saving..." : "Save"}
      </button>
    </form>
  );
}
