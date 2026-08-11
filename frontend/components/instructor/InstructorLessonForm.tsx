"use client";

import { useActionState } from "react";
import { createInstructorLessonAction, updateInstructorLessonAction } from "@/lib/instructor-actions";
import type { FormState } from "@/lib/actions";
import type { Lesson } from "@/lib/api";

const initialState: FormState = { error: null };

export function InstructorLessonForm({
  courseId,
  moduleId,
  lesson,
}: {
  courseId: string;
  moduleId: string;
  lesson?: Lesson;
}) {
  const action = lesson
    ? updateInstructorLessonAction.bind(null, courseId, lesson.id)
    : createInstructorLessonAction.bind(null, courseId, moduleId);
  const [state, formAction, pending] = useActionState(action, initialState);

  return (
    <form action={formAction} className="admin-form">
      <label>
        Название
        <input type="text" name="title" defaultValue={lesson?.title} required />
      </label>
      <label>
        Slug
        <input type="text" name="slug" defaultValue={lesson?.slug} required />
      </label>
      <label>
        Описание
        <textarea name="description" rows={2} defaultValue={lesson?.description} />
      </label>
      <label>
        Video URL (необязательно — можно загрузить файл ниже)
        <input type="text" name="video_url" defaultValue={lesson?.video_url} />
      </label>
      <label>
        Длительность (сек)
        <input type="number" name="duration_seconds" min={0} defaultValue={lesson?.duration_seconds ?? 0} />
      </label>
      <label className="admin-form-checkbox">
        <input type="checkbox" name="is_free" defaultChecked={lesson?.is_free} />
        Бесплатный урок
      </label>
      <label className="admin-form-checkbox">
        <input type="checkbox" name="published" defaultChecked={lesson?.published} />
        Опубликован
      </label>
      {state.error && <p role="alert">{state.error}</p>}
      <button type="submit" className="btn-small" disabled={pending}>
        {pending ? "..." : lesson ? "Сохранить урок" : "Добавить урок"}
      </button>
    </form>
  );
}
