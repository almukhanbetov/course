"use server";

import { redirect } from "next/navigation";
import { SERVER_API_URL } from "@/lib/api";
import { getSessionToken } from "@/lib/session";
import type { FormState } from "@/lib/actions";

async function instructorFetch(path: string, init?: RequestInit): Promise<Response> {
  const token = await getSessionToken();
  if (!token) {
    redirect("/login");
  }

  return fetch(`${SERVER_API_URL}/api/v1/instructor${path}`, {
    ...init,
    headers: { ...(init?.headers ?? {}), Authorization: `Bearer ${token}`, "Content-Type": "application/json" },
    cache: "no-store",
  });
}

async function instructorFetchJSON<T>(path: string): Promise<T> {
  const res = await instructorFetch(path);
  if (!res.ok) {
    throw new Error(await parseError(res, "request failed"));
  }
  return res.json();
}

async function parseError(res: Response, fallback: string): Promise<string> {
  const body = await res.json().catch(() => null);
  return body?.error?.message ?? fallback;
}

// --- Courses -------------------------------------------------------------
// courseBody deliberately never includes published/instructor_id/
// publication_status — the backend's InstructorCourseInput has no field for
// any of them, so there is nothing here to smuggle even if someone tried.

function courseBody(formData: FormData) {
  const categoryId = String(formData.get("category_id") ?? "");
  return JSON.stringify({
    title: String(formData.get("title") ?? ""),
    slug: String(formData.get("slug") ?? ""),
    description: String(formData.get("description") ?? ""),
    level: String(formData.get("level") ?? "beginner"),
    image_url: String(formData.get("image_url") ?? ""),
    access_type: String(formData.get("access_type") ?? "free"),
    category_id: categoryId || null,
  });
}

export async function createMyCourseAction(_prevState: FormState, formData: FormData): Promise<FormState> {
  const res = await instructorFetch(`/courses`, { method: "POST", body: courseBody(formData) });
  if (!res.ok) return { error: await parseError(res, "Не удалось создать курс") };
  const course = await res.json();
  redirect(`/instructor/courses/${course.id}`);
}

export async function updateMyCourseAction(courseId: string, _prevState: FormState, formData: FormData): Promise<FormState> {
  const res = await instructorFetch(`/courses/${courseId}`, { method: "PUT", body: courseBody(formData) });
  if (!res.ok) return { error: await parseError(res, "Не удалось сохранить курс") };
  redirect(`/instructor/courses/${courseId}`);
}

export async function submitCourseForReviewAction(courseId: string) {
  const res = await instructorFetch(`/courses/${courseId}/submit`, { method: "POST" });
  if (!res.ok) {
    const message = await parseError(res, "Не удалось отправить курс на проверку");
    redirect(`/instructor/courses/${courseId}?error=${encodeURIComponent(message)}`);
  }
  redirect(`/instructor/courses/${courseId}`);
}

// --- Modules ---------------------------------------------------------

export async function createInstructorModuleAction(
  courseId: string,
  _prevState: FormState,
  formData: FormData,
): Promise<FormState> {
  const res = await instructorFetch(`/courses/${courseId}/modules`, {
    method: "POST",
    body: JSON.stringify({ title: String(formData.get("title") ?? "") }),
  });
  if (!res.ok) return { error: await parseError(res, "Не удалось создать модуль") };
  redirect(`/instructor/courses/${courseId}`);
}

export async function updateInstructorModuleAction(
  courseId: string,
  moduleId: string,
  _prevState: FormState,
  formData: FormData,
): Promise<FormState> {
  const res = await instructorFetch(`/modules/${moduleId}`, {
    method: "PUT",
    body: JSON.stringify({ title: String(formData.get("title") ?? "") }),
  });
  if (!res.ok) return { error: await parseError(res, "Не удалось сохранить модуль") };
  redirect(`/instructor/courses/${courseId}`);
}

export async function deleteInstructorModuleAction(courseId: string, moduleId: string) {
  await instructorFetch(`/modules/${moduleId}`, { method: "DELETE" });
  redirect(`/instructor/courses/${courseId}`);
}

interface CourseModulesShape {
  modules: { id: string; position: number }[];
}

export async function moveInstructorModuleAction(courseId: string, moduleId: string, direction: "up" | "down") {
  const course = await instructorFetchJSON<CourseModulesShape>(`/courses/${courseId}`);
  const sorted = [...course.modules].sort((a, b) => a.position - b.position);
  const idx = sorted.findIndex((m) => m.id === moduleId);
  const swapWith = direction === "up" ? idx - 1 : idx + 1;

  if (idx !== -1 && swapWith >= 0 && swapWith < sorted.length) {
    const items = sorted.map((m, i) => ({ id: m.id, position: i + 1 }));
    [items[idx].position, items[swapWith].position] = [items[swapWith].position, items[idx].position];
    await instructorFetch(`/courses/${courseId}/modules/reorder`, {
      method: "PUT",
      body: JSON.stringify({ items }),
    });
  }

  redirect(`/instructor/courses/${courseId}`);
}

// --- Lessons -----------------------------------------------------------

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

export async function createInstructorLessonAction(
  courseId: string,
  moduleId: string,
  _prevState: FormState,
  formData: FormData,
): Promise<FormState> {
  const res = await instructorFetch(`/modules/${moduleId}/lessons`, { method: "POST", body: lessonBody(formData) });
  if (!res.ok) return { error: await parseError(res, "Не удалось создать урок") };
  redirect(`/instructor/courses/${courseId}`);
}

export async function updateInstructorLessonAction(
  courseId: string,
  lessonId: string,
  _prevState: FormState,
  formData: FormData,
): Promise<FormState> {
  const res = await instructorFetch(`/lessons/${lessonId}`, { method: "PUT", body: lessonBody(formData) });
  if (!res.ok) return { error: await parseError(res, "Не удалось сохранить урок") };
  redirect(`/instructor/courses/${courseId}`);
}

export async function deleteInstructorLessonAction(courseId: string, lessonId: string) {
  await instructorFetch(`/lessons/${lessonId}`, { method: "DELETE" });
  redirect(`/instructor/courses/${courseId}`);
}

interface ModuleLessonsShape {
  modules: { id: string; lessons: { id: string; position: number }[] }[];
}

export async function moveInstructorLessonAction(courseId: string, moduleId: string, lessonId: string, direction: "up" | "down") {
  const course = await instructorFetchJSON<ModuleLessonsShape>(`/courses/${courseId}`);
  const targetModule = course.modules.find((m) => m.id === moduleId);

  if (targetModule) {
    const sorted = [...targetModule.lessons].sort((a, b) => a.position - b.position);
    const idx = sorted.findIndex((l) => l.id === lessonId);
    const swapWith = direction === "up" ? idx - 1 : idx + 1;

    if (idx !== -1 && swapWith >= 0 && swapWith < sorted.length) {
      const items = sorted.map((l, i) => ({ id: l.id, position: i + 1 }));
      [items[idx].position, items[swapWith].position] = [items[swapWith].position, items[idx].position];
      await instructorFetch(`/modules/${moduleId}/lessons/reorder`, {
        method: "PUT",
        body: JSON.stringify({ items }),
      });
    }
  }

  redirect(`/instructor/courses/${courseId}`);
}

// --- Lesson video --------------------------------------------------------
// Upload is its own fetch (not instructorFetch) — a multipart body must NOT
// have a manually-set Content-Type, same reasoning as the admin upload action.

export async function uploadInstructorLessonVideoAction(
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

  const res = await fetch(`${SERVER_API_URL}/api/v1/instructor/lessons/${lessonId}/video`, {
    method: "POST",
    headers: { Authorization: `Bearer ${token}` },
    body: uploadForm,
    cache: "no-store",
  });

  if (!res.ok) {
    return { error: await parseError(res, "Не удалось загрузить видео") };
  }

  redirect(`/instructor/courses/${courseId}`);
}

export async function deleteInstructorLessonVideoAction(courseId: string, lessonId: string) {
  await instructorFetch(`/lessons/${lessonId}/video`, { method: "DELETE" });
  redirect(`/instructor/courses/${courseId}`);
}

// Called from a client component on a polling timer — see
// InstructorVideoProcessingStatusPanel (mirrors admin's equivalent).
export interface VideoProcessingStatusResult {
  status?: import("@/lib/admin-api").VideoProcessingStatus;
  error?: string;
}

export async function getInstructorVideoProcessingStatusAction(lessonId: string): Promise<VideoProcessingStatusResult> {
  const res = await instructorFetch(`/lessons/${lessonId}/video/status`);
  if (res.status === 404) {
    return { status: undefined };
  }
  if (!res.ok) {
    return { error: await parseError(res, "Не удалось получить статус обработки видео") };
  }
  return { status: await res.json() };
}

// --- Tests -----------------------------------------------------------

function testBody(formData: FormData, parent: { courseId?: string; moduleId?: string; lessonId?: string }) {
  return JSON.stringify({
    course_id: parent.courseId ?? null,
    module_id: parent.moduleId ?? null,
    lesson_id: parent.lessonId ?? null,
    title: String(formData.get("title") ?? ""),
    description: String(formData.get("description") ?? ""),
    passing_score: Number(formData.get("passing_score") ?? 70),
    published: formData.get("published") === "on",
    is_final: formData.get("is_final") === "on",
  });
}

export async function createInstructorTestAction(
  courseId: string,
  _prevState: FormState,
  formData: FormData,
): Promise<FormState> {
  const res = await instructorFetch(`/tests`, { method: "POST", body: testBody(formData, { courseId }) });
  if (!res.ok) return { error: await parseError(res, "Не удалось создать тест") };
  redirect(`/instructor/courses/${courseId}`);
}

export async function updateInstructorTestAction(
  courseId: string,
  testId: string,
  _prevState: FormState,
  formData: FormData,
): Promise<FormState> {
  const res = await instructorFetch(`/tests/${testId}`, { method: "PUT", body: testBody(formData, { courseId }) });
  if (!res.ok) return { error: await parseError(res, "Не удалось сохранить тест") };
  redirect(`/instructor/courses/${courseId}/tests/${testId}`);
}

export async function deleteInstructorTestAction(courseId: string, testId: string) {
  await instructorFetch(`/tests/${testId}`, { method: "DELETE" });
  redirect(`/instructor/courses/${courseId}`);
}

export async function createInstructorQuestionAction(
  courseId: string,
  testId: string,
  _prevState: FormState,
  formData: FormData,
): Promise<FormState> {
  const res = await instructorFetch(`/tests/${testId}/questions`, {
    method: "POST",
    body: JSON.stringify({ text: String(formData.get("text") ?? "") }),
  });
  if (!res.ok) return { error: await parseError(res, "Не удалось создать вопрос") };
  redirect(`/instructor/courses/${courseId}/tests/${testId}`);
}

export async function updateInstructorQuestionAction(
  courseId: string,
  testId: string,
  questionId: string,
  _prevState: FormState,
  formData: FormData,
): Promise<FormState> {
  const res = await instructorFetch(`/questions/${questionId}`, {
    method: "PUT",
    body: JSON.stringify({ text: String(formData.get("text") ?? "") }),
  });
  if (!res.ok) return { error: await parseError(res, "Не удалось сохранить вопрос") };
  redirect(`/instructor/courses/${courseId}/tests/${testId}`);
}

export async function deleteInstructorQuestionAction(courseId: string, testId: string, questionId: string) {
  await instructorFetch(`/questions/${questionId}`, { method: "DELETE" });
  redirect(`/instructor/courses/${courseId}/tests/${testId}`);
}

export async function createInstructorAnswerAction(
  courseId: string,
  testId: string,
  questionId: string,
  _prevState: FormState,
  formData: FormData,
): Promise<FormState> {
  const res = await instructorFetch(`/questions/${questionId}/answers`, {
    method: "POST",
    body: JSON.stringify({
      text: String(formData.get("text") ?? ""),
      is_correct: formData.get("is_correct") === "on",
    }),
  });
  if (!res.ok) return { error: await parseError(res, "Не удалось создать ответ") };
  redirect(`/instructor/courses/${courseId}/tests/${testId}`);
}

export async function setInstructorCorrectAnswerAction(courseId: string, testId: string, answerId: string, text: string) {
  await instructorFetch(`/answers/${answerId}`, {
    method: "PUT",
    body: JSON.stringify({ text, is_correct: true }),
  });
  redirect(`/instructor/courses/${courseId}/tests/${testId}`);
}

export async function deleteInstructorAnswerAction(courseId: string, testId: string, answerId: string) {
  await instructorFetch(`/answers/${answerId}`, { method: "DELETE" });
  redirect(`/instructor/courses/${courseId}/tests/${testId}`);
}

// --- Assignments / homework (Stage 15) ---

function assignmentBody(formData: FormData) {
  const maxScoreRaw = String(formData.get("max_score") ?? "").trim();
  return JSON.stringify({
    title: String(formData.get("title") ?? ""),
    description: String(formData.get("description") ?? ""),
    instructions: String(formData.get("instructions") ?? ""),
    required: formData.get("required") === "on",
    max_score: maxScoreRaw ? Number(maxScoreRaw) : null,
    published: formData.get("published") === "on",
  });
}

export async function createInstructorAssignmentAction(
  courseId: string,
  lessonId: string,
  _prevState: FormState,
  formData: FormData,
): Promise<FormState> {
  const res = await instructorFetch(`/lessons/${lessonId}/assignment`, { method: "POST", body: assignmentBody(formData) });
  if (!res.ok) return { error: await parseError(res, "Не удалось создать задание") };
  redirect(`/instructor/courses/${courseId}`);
}

export async function updateInstructorAssignmentAction(
  courseId: string,
  assignmentId: string,
  _prevState: FormState,
  formData: FormData,
): Promise<FormState> {
  const res = await instructorFetch(`/assignments/${assignmentId}`, { method: "PUT", body: assignmentBody(formData) });
  if (!res.ok) return { error: await parseError(res, "Не удалось сохранить задание") };
  redirect(`/instructor/courses/${courseId}`);
}

export async function deleteInstructorAssignmentAction(courseId: string, assignmentId: string) {
  const res = await instructorFetch(`/assignments/${assignmentId}`, { method: "DELETE" });
  if (!res.ok) {
    const message = await parseError(res, "Не удалось удалить задание");
    redirect(`/instructor/courses/${courseId}?error=${encodeURIComponent(message)}`);
  }
  redirect(`/instructor/courses/${courseId}`);
}

// --- Coding exercises / secure code runner (Stage 16) ---

function codingExerciseBody(formData: FormData) {
  return JSON.stringify({
    title: String(formData.get("title") ?? ""),
    description: String(formData.get("description") ?? ""),
    language: String(formData.get("language") ?? "python"),
    starter_code: String(formData.get("starter_code") ?? ""),
    solution_code: String(formData.get("solution_code") ?? "") || null,
    time_limit_ms: Number(formData.get("time_limit_ms") ?? 5000),
    memory_limit_mb: Number(formData.get("memory_limit_mb") ?? 128),
    published: formData.get("published") === "on",
    required: formData.get("required") === "on",
  });
}

export async function createInstructorCodingExerciseAction(
  courseId: string,
  lessonId: string,
  _prevState: FormState,
  formData: FormData,
): Promise<FormState> {
  const res = await instructorFetch(`/lessons/${lessonId}/coding-exercise`, { method: "POST", body: codingExerciseBody(formData) });
  if (!res.ok) return { error: await parseError(res, "Не удалось создать упражнение") };
  redirect(`/instructor/courses/${courseId}`);
}

export async function updateInstructorCodingExerciseAction(
  courseId: string,
  exerciseId: string,
  _prevState: FormState,
  formData: FormData,
): Promise<FormState> {
  const res = await instructorFetch(`/coding-exercises/${exerciseId}`, { method: "PUT", body: codingExerciseBody(formData) });
  if (!res.ok) return { error: await parseError(res, "Не удалось сохранить упражнение") };
  redirect(`/instructor/courses/${courseId}`);
}

export async function deleteInstructorCodingExerciseAction(courseId: string, exerciseId: string) {
  const res = await instructorFetch(`/coding-exercises/${exerciseId}`, { method: "DELETE" });
  if (!res.ok) {
    const message = await parseError(res, "Не удалось удалить упражнение");
    redirect(`/instructor/courses/${courseId}?error=${encodeURIComponent(message)}`);
  }
  redirect(`/instructor/courses/${courseId}`);
}

function testCaseBody(formData: FormData) {
  const input = String(formData.get("input") ?? "");
  return JSON.stringify({
    input: input || null,
    expected_output: String(formData.get("expected_output") ?? ""),
    position: Number(formData.get("position") ?? 1),
    hidden: formData.get("hidden") === "on",
  });
}

export async function createInstructorTestCaseAction(
  courseId: string,
  exerciseId: string,
  _prevState: FormState,
  formData: FormData,
): Promise<FormState> {
  const res = await instructorFetch(`/coding-exercises/${exerciseId}/test-cases`, { method: "POST", body: testCaseBody(formData) });
  if (!res.ok) return { error: await parseError(res, "Не удалось добавить тест") };
  redirect(`/instructor/courses/${courseId}`);
}

export async function updateInstructorTestCaseAction(
  courseId: string,
  testCaseId: string,
  _prevState: FormState,
  formData: FormData,
): Promise<FormState> {
  const res = await instructorFetch(`/coding-test-cases/${testCaseId}`, { method: "PUT", body: testCaseBody(formData) });
  if (!res.ok) return { error: await parseError(res, "Не удалось сохранить тест") };
  redirect(`/instructor/courses/${courseId}`);
}

export async function deleteInstructorTestCaseAction(courseId: string, testCaseId: string) {
  const res = await instructorFetch(`/coding-test-cases/${testCaseId}`, { method: "DELETE" });
  if (!res.ok) {
    const message = await parseError(res, "Не удалось удалить тест");
    redirect(`/instructor/courses/${courseId}?error=${encodeURIComponent(message)}`);
  }
  redirect(`/instructor/courses/${courseId}`);
}

export async function reviewSubmissionAction(
  submissionId: string,
  _prevState: FormState,
  formData: FormData,
): Promise<FormState> {
  const scoreRaw = String(formData.get("score") ?? "").trim();
  const res = await instructorFetch(`/submissions/${submissionId}/review`, {
    method: "POST",
    body: JSON.stringify({
      status: String(formData.get("status") ?? ""),
      score: scoreRaw ? Number(scoreRaw) : null,
      feedback: String(formData.get("feedback") ?? ""),
    }),
  });
  if (!res.ok) return { error: await parseError(res, "Не удалось сохранить проверку") };
  redirect(`/instructor/submissions/${submissionId}`);
}
