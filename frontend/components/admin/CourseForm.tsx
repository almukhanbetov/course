"use client";

import { useActionState } from "react";
import { createCourseAction, updateCourseAction } from "@/lib/admin-actions";
import type { FormState } from "@/lib/actions";
import type { Category, Course, PublicUser } from "@/lib/api";

const initialState: FormState = { error: null };

export function CourseForm({
  course,
  categories,
  instructors,
}: {
  course?: Course;
  categories: Category[];
  instructors: PublicUser[];
}) {
  const action = course ? updateCourseAction.bind(null, course.id) : createCourseAction;
  const [state, formAction, pending] = useActionState(action, initialState);

  return (
    <form action={formAction} className="admin-form">
      <label>
        Title
        <input type="text" name="title" defaultValue={course?.title} required />
      </label>
      <label>
        Slug
        <input type="text" name="slug" defaultValue={course?.slug} required />
      </label>
      <label>
        Description
        <textarea name="description" rows={4} defaultValue={course?.description} />
      </label>
      <label>
        Level
        <select name="level" defaultValue={course?.level ?? "beginner"}>
          <option value="beginner">beginner</option>
          <option value="intermediate">intermediate</option>
          <option value="advanced">advanced</option>
        </select>
      </label>
      <label>
        Image URL
        <input type="text" name="image_url" defaultValue={course?.image_url} />
      </label>
      <label>
        Category
        <select name="category_id" defaultValue={course?.category_id ?? ""}>
          <option value="">No category</option>
          {categories.map((c) => (
            <option key={c.id} value={c.id}>
              {c.name}
            </option>
          ))}
        </select>
      </label>
      <label>
        Instructor
        <select name="instructor_id" defaultValue={course?.instructor_id ?? ""}>
          <option value="">No instructor (admin-managed)</option>
          {instructors.map((i) => (
            <option key={i.id} value={i.id}>
              {i.first_name} {i.last_name} ({i.email})
            </option>
          ))}
        </select>
      </label>
      <label>
        Access type
        <select name="access_type" defaultValue={course?.access_type ?? "free"}>
          <option value="free">free — открыт всем</option>
          <option value="subscription">subscription — только по активной подписке</option>
        </select>
      </label>
      <label className="admin-form-checkbox">
        <input type="checkbox" name="published" defaultChecked={course?.published} />
        Published
      </label>
      {state.error && <p role="alert">{state.error}</p>}
      <button type="submit" className="btn-primary" disabled={pending}>
        {pending ? "Saving..." : "Save"}
      </button>
    </form>
  );
}
