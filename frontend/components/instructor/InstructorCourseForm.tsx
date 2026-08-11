"use client";

import { useActionState } from "react";
import { createMyCourseAction, updateMyCourseAction } from "@/lib/instructor-actions";
import type { FormState } from "@/lib/actions";
import type { Category, Course } from "@/lib/api";

const initialState: FormState = { error: null };

// Basic information + Category — deliberately no Published/Publication
// status field here at all: those live only in PublicationPanel, and the
// backend's InstructorCourseInput has no field for them either.
export function InstructorCourseForm({ course, categories }: { course?: Course; categories: Category[] }) {
  const action = course ? updateMyCourseAction.bind(null, course.id) : createMyCourseAction;
  const [state, formAction, pending] = useActionState(action, initialState);

  return (
    <form action={formAction} className="admin-form">
      <label>
        Название
        <input type="text" name="title" defaultValue={course?.title} required />
      </label>
      <label>
        Slug
        <input type="text" name="slug" defaultValue={course?.slug} required />
      </label>
      <label>
        Описание
        <textarea name="description" rows={4} defaultValue={course?.description} />
      </label>
      <label>
        Уровень
        <select name="level" defaultValue={course?.level ?? "beginner"}>
          <option value="beginner">Начальный</option>
          <option value="intermediate">Средний</option>
          <option value="advanced">Продвинутый</option>
        </select>
      </label>
      <label>
        Изображение (URL)
        <input type="text" name="image_url" defaultValue={course?.image_url} />
      </label>
      <label>
        Категория
        <select name="category_id" defaultValue={course?.category_id ?? ""}>
          <option value="">Без категории</option>
          {categories.map((c) => (
            <option key={c.id} value={c.id}>
              {c.name}
            </option>
          ))}
        </select>
      </label>
      <label>
        Доступ
        <select name="access_type" defaultValue={course?.access_type ?? "free"}>
          <option value="free">Бесплатный</option>
          <option value="subscription">По подписке</option>
        </select>
      </label>
      {state.error && <p role="alert">{state.error}</p>}
      <button type="submit" className="btn-primary" disabled={pending}>
        {pending ? "Сохранение..." : "Сохранить"}
      </button>
    </form>
  );
}
