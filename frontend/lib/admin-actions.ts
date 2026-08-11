"use server";

import { redirect } from "next/navigation";
import { SERVER_API_URL } from "@/lib/api";
import { getSessionToken } from "@/lib/session";
import type { FormState } from "@/lib/actions";

async function adminFetch(path: string, init?: RequestInit): Promise<Response> {
  const token = await getSessionToken();
  if (!token) {
    redirect("/login");
  }

  return fetch(`${SERVER_API_URL}/api/v1${path}`, {
    ...init,
    headers: { ...(init?.headers ?? {}), Authorization: `Bearer ${token}`, "Content-Type": "application/json" },
    cache: "no-store",
  });
}

async function adminFetchJSON<T>(path: string): Promise<T> {
  const res = await adminFetch(path);
  if (!res.ok) {
    throw new Error(await parseError(res, "request failed"));
  }
  return res.json();
}

async function parseError(res: Response, fallback: string): Promise<string> {
  const body = await res.json().catch(() => null);
  return body?.error?.message ?? fallback;
}

// --- Users ---

export async function updateUserAction(userId: string, _prevState: FormState, formData: FormData): Promise<FormState> {
  const res = await adminFetch(`/admin/users/${userId}`, {
    method: "PUT",
    body: JSON.stringify({
      first_name: String(formData.get("first_name") ?? ""),
      last_name: String(formData.get("last_name") ?? ""),
      active: formData.get("active") === "on",
      role: String(formData.get("role") ?? ""),
    }),
  });
  if (!res.ok) return { error: await parseError(res, "Не удалось обновить пользователя") };
  redirect("/admin/users");
}

// --- Courses ---

function courseBody(formData: FormData) {
  const categoryId = String(formData.get("category_id") ?? "");
  const instructorId = String(formData.get("instructor_id") ?? "");
  return JSON.stringify({
    title: String(formData.get("title") ?? ""),
    slug: String(formData.get("slug") ?? ""),
    description: String(formData.get("description") ?? ""),
    level: String(formData.get("level") ?? "beginner"),
    image_url: String(formData.get("image_url") ?? ""),
    published: formData.get("published") === "on",
    access_type: String(formData.get("access_type") ?? "free"),
    category_id: categoryId || null,
    instructor_id: instructorId || null,
  });
}

export async function createCourseAction(_prevState: FormState, formData: FormData): Promise<FormState> {
  const res = await adminFetch(`/admin/courses`, { method: "POST", body: courseBody(formData) });
  if (!res.ok) return { error: await parseError(res, "Не удалось создать курс") };
  const course = await res.json();
  redirect(`/admin/courses/${course.id}`);
}

export async function updateCourseAction(courseId: string, _prevState: FormState, formData: FormData): Promise<FormState> {
  const res = await adminFetch(`/admin/courses/${courseId}`, { method: "PUT", body: courseBody(formData) });
  if (!res.ok) return { error: await parseError(res, "Не удалось сохранить курс") };
  redirect(`/admin/courses/${courseId}`);
}

export async function deleteCourseAction(courseId: string) {
  const res = await adminFetch(`/admin/courses/${courseId}`, { method: "DELETE" });
  if (!res.ok) {
    const message = await parseError(res, "Не удалось удалить курс");
    redirect(`/admin/courses/${courseId}?error=${encodeURIComponent(message)}`);
  }
  redirect("/admin/courses");
}

// --- Modules ---

export async function createModuleAction(courseId: string, _prevState: FormState, formData: FormData): Promise<FormState> {
  const res = await adminFetch(`/admin/courses/${courseId}/modules`, {
    method: "POST",
    body: JSON.stringify({ title: String(formData.get("title") ?? "") }),
  });
  if (!res.ok) return { error: await parseError(res, "Не удалось создать модуль") };
  redirect(`/admin/courses/${courseId}`);
}

export async function updateModuleAction(
  courseId: string,
  moduleId: string,
  _prevState: FormState,
  formData: FormData,
): Promise<FormState> {
  const res = await adminFetch(`/admin/modules/${moduleId}`, {
    method: "PUT",
    body: JSON.stringify({ title: String(formData.get("title") ?? "") }),
  });
  if (!res.ok) return { error: await parseError(res, "Не удалось сохранить модуль") };
  redirect(`/admin/courses/${courseId}`);
}

export async function deleteModuleAction(courseId: string, moduleId: string) {
  await adminFetch(`/admin/modules/${moduleId}`, { method: "DELETE" });
  redirect(`/admin/courses/${courseId}`);
}

interface CourseModulesShape {
  modules: { id: string; position: number }[];
}

export async function moveModuleAction(courseId: string, moduleId: string, direction: "up" | "down") {
  const course = await adminFetchJSON<CourseModulesShape>(`/admin/courses/${courseId}`);
  const sorted = [...course.modules].sort((a, b) => a.position - b.position);
  const idx = sorted.findIndex((m) => m.id === moduleId);
  const swapWith = direction === "up" ? idx - 1 : idx + 1;

  if (idx !== -1 && swapWith >= 0 && swapWith < sorted.length) {
    const items = sorted.map((m, i) => ({ id: m.id, position: i + 1 }));
    [items[idx].position, items[swapWith].position] = [items[swapWith].position, items[idx].position];
    await adminFetch(`/admin/courses/${courseId}/modules/reorder`, {
      method: "PUT",
      body: JSON.stringify({ items }),
    });
  }

  redirect(`/admin/courses/${courseId}`);
}

// --- Lessons ---

function lessonBody(formData: FormData) {
  return JSON.stringify({
    title: String(formData.get("title") ?? ""),
    slug: String(formData.get("slug") ?? ""),
    description: String(formData.get("description") ?? ""),
    video_url: String(formData.get("video_url") ?? ""),
    duration_seconds: Number(formData.get("duration_seconds") ?? 0),
    is_free: formData.get("is_free") === "on",
    published: formData.get("published") === "on",
  });
}

export async function createLessonAction(
  courseId: string,
  moduleId: string,
  _prevState: FormState,
  formData: FormData,
): Promise<FormState> {
  const res = await adminFetch(`/admin/modules/${moduleId}/lessons`, { method: "POST", body: lessonBody(formData) });
  if (!res.ok) return { error: await parseError(res, "Не удалось создать урок") };
  redirect(`/admin/courses/${courseId}`);
}

export async function updateLessonAction(
  courseId: string,
  lessonId: string,
  _prevState: FormState,
  formData: FormData,
): Promise<FormState> {
  const res = await adminFetch(`/admin/lessons/${lessonId}`, { method: "PUT", body: lessonBody(formData) });
  if (!res.ok) return { error: await parseError(res, "Не удалось сохранить урок") };
  redirect(`/admin/courses/${courseId}`);
}

export async function deleteLessonAction(courseId: string, lessonId: string) {
  await adminFetch(`/admin/lessons/${lessonId}`, { method: "DELETE" });
  redirect(`/admin/courses/${courseId}`);
}

interface ModuleLessonsShape {
  modules: { id: string; lessons: { id: string; position: number }[] }[];
}

export async function moveLessonAction(courseId: string, moduleId: string, lessonId: string, direction: "up" | "down") {
  const course = await adminFetchJSON<ModuleLessonsShape>(`/admin/courses/${courseId}`);
  const targetModule = course.modules.find((m) => m.id === moduleId);

  if (targetModule) {
    const sorted = [...targetModule.lessons].sort((a, b) => a.position - b.position);
    const idx = sorted.findIndex((l) => l.id === lessonId);
    const swapWith = direction === "up" ? idx - 1 : idx + 1;

    if (idx !== -1 && swapWith >= 0 && swapWith < sorted.length) {
      const items = sorted.map((l, i) => ({ id: l.id, position: i + 1 }));
      [items[idx].position, items[swapWith].position] = [items[swapWith].position, items[idx].position];
      await adminFetch(`/admin/modules/${moduleId}/lessons/reorder`, {
        method: "PUT",
        body: JSON.stringify({ items }),
      });
    }
  }

  redirect(`/admin/courses/${courseId}`);
}

// --- Course submissions (instructor authoring moderation) ---

export async function approveCourseSubmissionAction(courseId: string) {
  const res = await adminFetch(`/admin/course-submissions/${courseId}`, {
    method: "PUT",
    body: JSON.stringify({ status: "published" }),
  });
  if (!res.ok) {
    const message = await parseError(res, "Не удалось опубликовать курс");
    redirect(`/admin/course-submissions?error=${encodeURIComponent(message)}`);
  }
  redirect("/admin/course-submissions");
}

export async function rejectCourseSubmissionAction(courseId: string, _prevState: FormState, formData: FormData): Promise<FormState> {
  const res = await adminFetch(`/admin/course-submissions/${courseId}`, {
    method: "PUT",
    body: JSON.stringify({ status: "rejected", rejection_reason: String(formData.get("rejection_reason") ?? "") }),
  });
  if (!res.ok) return { error: await parseError(res, "Не удалось отклонить курс") };
  redirect("/admin/course-submissions");
}

// --- Categories ---
// There is deliberately no deleteCategoryAction: a category referenced by
// courses must never disappear from under them. "Deactivate" (via the same
// update form's Active checkbox) is the only supported way to retire one —
// see backend internal/categories/repository.go.

function categoryBody(formData: FormData) {
  return JSON.stringify({
    name: String(formData.get("name") ?? ""),
    slug: String(formData.get("slug") ?? ""),
    description: String(formData.get("description") ?? ""),
    position: Number(formData.get("position") ?? 0),
    active: formData.get("active") === "on",
  });
}

export async function createCategoryAction(_prevState: FormState, formData: FormData): Promise<FormState> {
  const res = await adminFetch(`/admin/categories`, { method: "POST", body: categoryBody(formData) });
  if (!res.ok) return { error: await parseError(res, "Не удалось создать категорию") };
  redirect("/admin/categories");
}

export async function updateCategoryAction(categoryId: string, _prevState: FormState, formData: FormData): Promise<FormState> {
  const res = await adminFetch(`/admin/categories/${categoryId}`, { method: "PUT", body: categoryBody(formData) });
  if (!res.ok) return { error: await parseError(res, "Не удалось сохранить категорию") };
  redirect("/admin/categories");
}

// --- Reviews (moderation) ---
// Deliberately the only mutation exposed: admins can hide/restore a review,
// never edit its rating or text — see backend internal/reviews/handler_admin.go.

export async function setReviewPublishedAction(reviewId: string, published: boolean) {
  await adminFetch(`/admin/reviews/${reviewId}`, {
    method: "PUT",
    body: JSON.stringify({ published }),
  });
  redirect("/admin/reviews");
}

// --- Specialities ---

function specialityBody(formData: FormData) {
  return JSON.stringify({
    title: String(formData.get("title") ?? ""),
    slug: String(formData.get("slug") ?? ""),
    description: String(formData.get("description") ?? ""),
    image_url: String(formData.get("image_url") ?? ""),
    published: formData.get("published") === "on",
  });
}

export async function createSpecialityAction(_prevState: FormState, formData: FormData): Promise<FormState> {
  const res = await adminFetch(`/admin/specialities`, { method: "POST", body: specialityBody(formData) });
  if (!res.ok) return { error: await parseError(res, "Не удалось создать специальность") };
  const speciality = await res.json();
  redirect(`/admin/specialities/${speciality.id}`);
}

export async function updateSpecialityAction(
  specialityId: string,
  _prevState: FormState,
  formData: FormData,
): Promise<FormState> {
  const res = await adminFetch(`/admin/specialities/${specialityId}`, { method: "PUT", body: specialityBody(formData) });
  if (!res.ok) return { error: await parseError(res, "Не удалось сохранить специальность") };
  redirect(`/admin/specialities/${specialityId}`);
}

export async function deleteSpecialityAction(specialityId: string) {
  await adminFetch(`/admin/specialities/${specialityId}`, { method: "DELETE" });
  redirect("/admin/specialities");
}

export async function addCourseToSpecialityAction(
  specialityId: string,
  _prevState: FormState,
  formData: FormData,
): Promise<FormState> {
  const res = await adminFetch(`/admin/specialities/${specialityId}/courses`, {
    method: "POST",
    body: JSON.stringify({
      course_id: String(formData.get("course_id") ?? ""),
      required: formData.get("required") === "on",
    }),
  });
  if (!res.ok) return { error: await parseError(res, "Не удалось добавить курс в roadmap") };
  redirect(`/admin/specialities/${specialityId}`);
}

export async function removeCourseFromSpecialityAction(specialityId: string, courseId: string) {
  await adminFetch(`/admin/specialities/${specialityId}/courses/${courseId}`, { method: "DELETE" });
  redirect(`/admin/specialities/${specialityId}`);
}

interface SpecialityCoursesShape {
  courses: { id: string; position: number; required: boolean }[];
}

export async function moveSpecialityCourseAction(specialityId: string, courseId: string, direction: "up" | "down") {
  const speciality = await adminFetchJSON<SpecialityCoursesShape>(`/specialities/${specialityId}`);
  const sorted = [...speciality.courses].sort((a, b) => a.position - b.position);
  const idx = sorted.findIndex((c) => c.id === courseId);
  const swapWith = direction === "up" ? idx - 1 : idx + 1;

  if (idx !== -1 && swapWith >= 0 && swapWith < sorted.length) {
    const items = sorted.map((c, i) => ({ course_id: c.id, position: i + 1, required: c.required }));
    [items[idx].position, items[swapWith].position] = [items[swapWith].position, items[idx].position];
    await adminFetch(`/admin/specialities/${specialityId}/courses/reorder`, {
      method: "PUT",
      body: JSON.stringify({ items }),
    });
  }

  redirect(`/admin/specialities/${specialityId}`);
}

export async function toggleRequiredAction(specialityId: string, courseId: string) {
  const speciality = await adminFetchJSON<SpecialityCoursesShape>(`/specialities/${specialityId}`);
  const items = speciality.courses.map((c) => ({
    course_id: c.id,
    position: c.position,
    required: c.id === courseId ? !c.required : c.required,
  }));

  await adminFetch(`/admin/specialities/${specialityId}/courses/reorder`, {
    method: "PUT",
    body: JSON.stringify({ items }),
  });
  redirect(`/admin/specialities/${specialityId}`);
}

// --- Tests ---

function testBody(formData: FormData) {
  const courseId = String(formData.get("course_id") ?? "");
  return JSON.stringify({
    course_id: courseId || null,
    title: String(formData.get("title") ?? ""),
    description: String(formData.get("description") ?? ""),
    passing_score: Number(formData.get("passing_score") ?? 70),
    published: formData.get("published") === "on",
    is_final: formData.get("is_final") === "on",
  });
}

export async function createTestAction(_prevState: FormState, formData: FormData): Promise<FormState> {
  const res = await adminFetch(`/admin/tests`, { method: "POST", body: testBody(formData) });
  if (!res.ok) return { error: await parseError(res, "Не удалось создать тест") };
  const test = await res.json();
  redirect(`/admin/tests/${test.id}`);
}

export async function updateTestAction(testId: string, _prevState: FormState, formData: FormData): Promise<FormState> {
  const res = await adminFetch(`/admin/tests/${testId}`, { method: "PUT", body: testBody(formData) });
  if (!res.ok) return { error: await parseError(res, "Не удалось сохранить тест") };
  redirect(`/admin/tests/${testId}`);
}

export async function deleteTestAction(testId: string) {
  await adminFetch(`/admin/tests/${testId}`, { method: "DELETE" });
  redirect("/admin/tests");
}

export async function createQuestionAction(testId: string, _prevState: FormState, formData: FormData): Promise<FormState> {
  const res = await adminFetch(`/admin/tests/${testId}/questions`, {
    method: "POST",
    body: JSON.stringify({ text: String(formData.get("text") ?? "") }),
  });
  if (!res.ok) return { error: await parseError(res, "Не удалось создать вопрос") };
  redirect(`/admin/tests/${testId}`);
}

export async function updateQuestionAction(
  testId: string,
  questionId: string,
  _prevState: FormState,
  formData: FormData,
): Promise<FormState> {
  const res = await adminFetch(`/admin/questions/${questionId}`, {
    method: "PUT",
    body: JSON.stringify({ text: String(formData.get("text") ?? "") }),
  });
  if (!res.ok) return { error: await parseError(res, "Не удалось сохранить вопрос") };
  redirect(`/admin/tests/${testId}`);
}

export async function deleteQuestionAction(testId: string, questionId: string) {
  await adminFetch(`/admin/questions/${questionId}`, { method: "DELETE" });
  redirect(`/admin/tests/${testId}`);
}

export async function createAnswerAction(
  testId: string,
  questionId: string,
  _prevState: FormState,
  formData: FormData,
): Promise<FormState> {
  const res = await adminFetch(`/admin/questions/${questionId}/answers`, {
    method: "POST",
    body: JSON.stringify({
      text: String(formData.get("text") ?? ""),
      is_correct: formData.get("is_correct") === "on",
    }),
  });
  if (!res.ok) return { error: await parseError(res, "Не удалось создать ответ") };
  redirect(`/admin/tests/${testId}`);
}

export async function setCorrectAnswerAction(testId: string, answerId: string, text: string) {
  await adminFetch(`/admin/answers/${answerId}`, {
    method: "PUT",
    body: JSON.stringify({ text, is_correct: true }),
  });
  redirect(`/admin/tests/${testId}`);
}

export async function deleteAnswerAction(testId: string, answerId: string) {
  await adminFetch(`/admin/answers/${answerId}`, { method: "DELETE" });
  redirect(`/admin/tests/${testId}`);
}

// --- Plans ---
// There is deliberately no deletePlanAction: a plan already referenced by a
// subscription must never disappear from under it. "Deactivate" (published
// via the same update form's Active checkbox) is the only supported way to
// retire a plan — see backend internal/subscriptions/handler_admin.go.

function planBody(formData: FormData) {
  return JSON.stringify({
    name: String(formData.get("name") ?? ""),
    slug: String(formData.get("slug") ?? ""),
    description: String(formData.get("description") ?? ""),
    price_amount: Math.round(Number(formData.get("price") ?? 0) * 100),
    currency: String(formData.get("currency") ?? "KZT"),
    duration_days: Number(formData.get("duration_days") ?? 30),
    active: formData.get("active") === "on",
  });
}

export async function createPlanAction(_prevState: FormState, formData: FormData): Promise<FormState> {
  const res = await adminFetch(`/admin/plans`, { method: "POST", body: planBody(formData) });
  if (!res.ok) return { error: await parseError(res, "Не удалось создать тариф") };
  redirect(`/admin/plans`);
}

export async function updatePlanAction(planId: string, _prevState: FormState, formData: FormData): Promise<FormState> {
  const res = await adminFetch(`/admin/plans/${planId}`, { method: "PUT", body: planBody(formData) });
  if (!res.ok) return { error: await parseError(res, "Не удалось сохранить тариф") };
  redirect(`/admin/plans`);
}

// --- Lesson videos ---
// Upload is deliberately its own fetch, not adminFetch — a multipart body
// must NOT have a manually-set Content-Type (fetch derives the correct
// "multipart/form-data; boundary=..." value from the FormData itself).

export async function uploadLessonVideoAction(
  courseId: string,
  lessonId: string,
  _prevState: FormState,
  formData: FormData,
): Promise<FormState> {
  const token = await getSessionToken();
  if (!token) {
    redirect("/login");
  }

  const file = formData.get("video");
  if (!(file instanceof File) || file.size === 0) {
    return { error: "Выберите видеофайл" };
  }

  const uploadForm = new FormData();
  uploadForm.append("video", file, file.name);

  const res = await fetch(`${SERVER_API_URL}/api/v1/admin/lessons/${lessonId}/video`, {
    method: "POST",
    headers: { Authorization: `Bearer ${token}` },
    body: uploadForm,
    cache: "no-store",
  });

  if (!res.ok) {
    return { error: await parseError(res, "Не удалось загрузить видео") };
  }

  redirect(`/admin/courses/${courseId}`);
}

export async function deleteLessonVideoAction(courseId: string, lessonId: string) {
  await adminFetch(`/admin/lessons/${lessonId}/video`, { method: "DELETE" });
  redirect(`/admin/courses/${courseId}`);
}

// Called from a client component on a polling timer (item 19: "Frontend
// может polling, например каждые несколько секунд") — a Server Action, not
// admin-api.ts, because that module is server-only and can't be invoked
// directly from client code, only re-exported this way.
export interface VideoProcessingStatusResult {
  status?: import("@/lib/admin-api").VideoProcessingStatus;
  error?: string;
}

export async function getVideoProcessingStatusAction(lessonId: string): Promise<VideoProcessingStatusResult> {
  const res = await adminFetch(`/admin/lessons/${lessonId}/video/status`);
  if (res.status === 404) {
    return { status: undefined };
  }
  if (!res.ok) {
    return { error: await parseError(res, "Не удалось получить статус обработки видео") };
  }
  return { status: await res.json() };
}

// --- Notifications (Stage 12) ---

export async function retryNotificationJobAction(jobId: string) {
  const res = await adminFetch(`/admin/notification-jobs/${jobId}/retry`, { method: "POST" });
  if (!res.ok) {
    const message = await parseError(res, "Не удалось перезапустить job");
    redirect(`/admin/notifications?error=${encodeURIComponent(message)}`);
  }
  redirect("/admin/notifications");
}

export async function announceCourseAction(courseId: string) {
  const res = await adminFetch(`/admin/courses/${courseId}/announce`, { method: "POST" });
  if (!res.ok) {
    const message = await parseError(res, "Не удалось отправить анонс");
    redirect(`/admin/courses/${courseId}?error=${encodeURIComponent(message)}`);
  }
  const body = await res.json();
  redirect(`/admin/courses/${courseId}?announced=${body.notified_users ?? 0}`);
}
