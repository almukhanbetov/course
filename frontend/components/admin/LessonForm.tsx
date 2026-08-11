"use client";

import { useActionState } from "react";
import { createLessonAction, updateLessonAction } from "@/lib/admin-actions";
import type { FormState } from "@/lib/actions";
import type { Lesson } from "@/lib/api";

const initialState: FormState = { error: null };

export function LessonForm({
  courseId,
  moduleId,
  lesson,
}: {
  courseId: string;
  moduleId: string;
  lesson?: Lesson;
}) {
  const action = lesson
    ? updateLessonAction.bind(null, courseId, lesson.id)
    : createLessonAction.bind(null, courseId, moduleId);
  const [state, formAction, pending] = useActionState(action, initialState);

  return (
    <form action={formAction} className="admin-form">
      <label>
        Title
        <input type="text" name="title" defaultValue={lesson?.title} required />
      </label>
      <label>
        Slug
        <input type="text" name="slug" defaultValue={lesson?.slug} required />
      </label>
      <label>
        Description
        <textarea name="description" rows={2} defaultValue={lesson?.description} />
      </label>
      <label>
        Video URL
        <input type="text" name="video_url" defaultValue={lesson?.video_url} />
      </label>
      <label>
        Duration (seconds)
        <input type="number" name="duration_seconds" min={0} defaultValue={lesson?.duration_seconds ?? 0} />
      </label>
      <label className="admin-form-checkbox">
        <input type="checkbox" name="is_free" defaultChecked={lesson?.is_free} />
        Free
      </label>
      <label className="admin-form-checkbox">
        <input type="checkbox" name="published" defaultChecked={lesson?.published} />
        Published
      </label>
      {state.error && <p role="alert">{state.error}</p>}
      <button type="submit" className="btn-small" disabled={pending}>
        {pending ? "..." : lesson ? "Save lesson" : "Add lesson"}
      </button>
    </form>
  );
}
