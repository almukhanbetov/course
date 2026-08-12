"use server";

import { cookies } from "next/headers";
import { redirect } from "next/navigation";
import { SERVER_API_URL, type CodeSubmission, type PageResult, type QAAnswer, type QAQuestion, type QAQuestionView } from "@/lib/api";
import { SESSION_COOKIE, SESSION_MAX_AGE, getSessionToken } from "@/lib/session";

export interface FormState {
  error: string | null;
}

export async function loginAction(_prevState: FormState, formData: FormData): Promise<FormState> {
  const email = String(formData.get("email") ?? "");
  const password = String(formData.get("password") ?? "");

  const res = await fetch(`${SERVER_API_URL}/api/v1/auth/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email, password }),
    cache: "no-store",
  });

  if (!res.ok) {
    const body = await res.json().catch(() => null);
    return { error: body?.error?.message ?? "Не удалось войти" };
  }

  const data = await res.json();

  (await cookies()).set(SESSION_COOKIE, data.token, {
    httpOnly: true,
    secure: process.env.NODE_ENV === "production",
    sameSite: "lax",
    path: "/",
    maxAge: SESSION_MAX_AGE,
  });

  redirect("/dashboard");
}

export async function registerAction(_prevState: FormState, formData: FormData): Promise<FormState> {
  const payload = {
    email: String(formData.get("email") ?? ""),
    password: String(formData.get("password") ?? ""),
    first_name: String(formData.get("first_name") ?? ""),
    last_name: String(formData.get("last_name") ?? ""),
  };

  const res = await fetch(`${SERVER_API_URL}/api/v1/auth/register`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
    cache: "no-store",
  });

  if (!res.ok) {
    const body = await res.json().catch(() => null);
    return { error: body?.error?.message ?? "Не удалось зарегистрироваться" };
  }

  redirect("/login?registered=1");
}

export async function logoutAction() {
  (await cookies()).delete(SESSION_COOKIE);
  redirect("/login");
}

export async function enrollAction(courseId: string) {
  const token = await getSessionToken();
  if (!token) {
    redirect("/login");
  }

  const res = await fetch(`${SERVER_API_URL}/api/v1/courses/${courseId}/enroll`, {
    method: "POST",
    headers: { Authorization: `Bearer ${token}` },
    cache: "no-store",
  });

  if (res.status === 403) {
    const body = await res.json().catch(() => null);
    if (body?.error?.code === "COURSE_ACCESS_REQUIRED") {
      redirect("/pricing");
    }
  }

  redirect("/dashboard/courses");
}

export async function completeLessonAction(courseId: string, lessonId: string, durationSeconds: number) {
  const token = await getSessionToken();
  if (!token) {
    redirect("/login");
  }

  await fetch(`${SERVER_API_URL}/api/v1/lessons/${lessonId}/progress`, {
    method: "PUT",
    headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json" },
    body: JSON.stringify({ progress_seconds: durationSeconds, completed: true }),
    cache: "no-store",
  });

  redirect(`/learn/${courseId}/${lessonId}`);
}

// createSubscriptionAction only ever sends plan_id — the backend quotes the
// price from the plan row itself, so nothing the browser sends can change
// what a subscription costs.
export async function createSubscriptionAction(planId: string) {
  const token = await getSessionToken();
  if (!token) {
    redirect("/login");
  }

  const res = await fetch(`${SERVER_API_URL}/api/v1/subscriptions`, {
    method: "POST",
    headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json" },
    body: JSON.stringify({ plan_id: planId }),
    cache: "no-store",
  });

  if (!res.ok) {
    redirect(`/pricing?error=${encodeURIComponent("Не удалось начать оформление подписки")}`);
  }

  const result = await res.json();
  redirect(`/checkout/${planId}?payment=${result.payment.id}`);
}

export async function mockConfirmPaymentAction(planId: string, paymentId: string) {
  const token = await getSessionToken();
  if (!token) {
    redirect("/login");
  }

  const res = await fetch(`${SERVER_API_URL}/api/v1/payments/${paymentId}/mock-confirm`, {
    method: "POST",
    headers: { Authorization: `Bearer ${token}` },
    cache: "no-store",
  });

  if (!res.ok) {
    const body = await res.json().catch(() => null);
    const message = encodeURIComponent(body?.error?.message ?? "Не удалось подтвердить платёж");
    redirect(`/checkout/${planId}?payment=${paymentId}&error=${message}`);
  }

  redirect("/dashboard/subscription");
}

export async function submitTestAction(testId: string, formData: FormData) {
  const token = await getSessionToken();
  if (!token) {
    redirect("/login");
  }

  const questionIds = new Set<string>();
  for (const key of formData.keys()) {
    if (key.startsWith("q-")) {
      questionIds.add(key.slice(2));
    }
  }

  const answers = Array.from(questionIds).map((questionId) => ({
    question_id: questionId,
    answer_id: String(formData.get(`q-${questionId}`) ?? ""),
  }));

  const res = await fetch(`${SERVER_API_URL}/api/v1/tests/${testId}/submit`, {
    method: "POST",
    headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json" },
    body: JSON.stringify({ answers }),
    cache: "no-store",
  });

  const body = await res.json().catch(() => null);

  if (!res.ok) {
    const message = encodeURIComponent(body?.error?.message ?? "Не удалось отправить тест");
    redirect(`/tests/${testId}?error=${message}`);
  }

  redirect(`/tests/${testId}?attempt=${body.attempt_id}`);
}

// --- Lesson video player (Stage 10) ---
// These two, unlike every action above, are called directly from a Client
// Component (LessonVideoPlayer) on a timer/event, not from a <form> submit —
// so they return data instead of redirecting.

export interface VideoUrlResult {
  status?: "processing" | "ready" | "failed";
  streamType?: "hls" | "mp4";
  url?: string;
  expiresIn?: number;
  error?: string;
  errorCode?: string;
}

export async function getLessonVideoUrlAction(lessonId: string): Promise<VideoUrlResult> {
  const token = await getSessionToken();
  if (!token) {
    return { error: "Не авторизован" };
  }

  const res = await fetch(`${SERVER_API_URL}/api/v1/lessons/${lessonId}/video`, {
    headers: { Authorization: `Bearer ${token}` },
    cache: "no-store",
  });

  if (!res.ok) {
    const body = await res.json().catch(() => null);
    return { error: body?.error?.message ?? "Не удалось получить видео", errorCode: body?.error?.code };
  }

  const data = await res.json();
  return {
    status: data.status,
    streamType: data.stream_type,
    url: data.url,
    expiresIn: data.expires_in,
  };
}

export interface SaveProgressResult {
  completed?: boolean;
  error?: string;
}

// The backend re-applies the completion threshold rule regardless of what
// this sends — see internal/learning enforceCompletionThreshold — so a
// hand-crafted call from devtools with completed=true still can't finish a
// lesson early.
export async function saveLessonProgressAction(
  lessonId: string,
  progressSeconds: number,
  completed: boolean,
): Promise<SaveProgressResult> {
  const token = await getSessionToken();
  if (!token) {
    return { error: "Не авторизован" };
  }

  const res = await fetch(`${SERVER_API_URL}/api/v1/lessons/${lessonId}/progress`, {
    method: "PUT",
    headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json" },
    body: JSON.stringify({ progress_seconds: Math.floor(progressSeconds), completed }),
    cache: "no-store",
  });

  if (!res.ok) {
    const body = await res.json().catch(() => null);
    return { error: body?.error?.message ?? "Не удалось сохранить прогресс" };
  }

  const data = await res.json();
  return { completed: data.completed };
}

// --- Course reviews (Stage 13) ---
// Rating/eligibility are re-validated server-side on every call (see
// internal/reviews) — these actions just forward the form fields and
// surface whatever error the backend returns (401/403/409/400).

function reviewBody(formData: FormData) {
  return JSON.stringify({
    rating: Number(formData.get("rating") ?? 0),
    review_text: String(formData.get("review_text") ?? ""),
  });
}

export async function createReviewAction(courseId: string, _prevState: FormState, formData: FormData): Promise<FormState> {
  const token = await getSessionToken();
  if (!token) {
    redirect("/login");
  }

  const res = await fetch(`${SERVER_API_URL}/api/v1/courses/${courseId}/reviews`, {
    method: "POST",
    headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json" },
    body: reviewBody(formData),
    cache: "no-store",
  });

  if (!res.ok) {
    const body = await res.json().catch(() => null);
    return { error: body?.error?.message ?? "Не удалось оставить отзыв" };
  }

  redirect(`/courses/${courseId}`);
}

export async function updateReviewAction(courseId: string, _prevState: FormState, formData: FormData): Promise<FormState> {
  const token = await getSessionToken();
  if (!token) {
    redirect("/login");
  }

  const res = await fetch(`${SERVER_API_URL}/api/v1/courses/${courseId}/reviews/me`, {
    method: "PUT",
    headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json" },
    body: reviewBody(formData),
    cache: "no-store",
  });

  if (!res.ok) {
    const body = await res.json().catch(() => null);
    return { error: body?.error?.message ?? "Не удалось сохранить отзыв" };
  }

  redirect(`/courses/${courseId}`);
}

export async function deleteReviewAction(courseId: string) {
  const token = await getSessionToken();
  if (!token) {
    redirect("/login");
  }

  await fetch(`${SERVER_API_URL}/api/v1/courses/${courseId}/reviews/me`, {
    method: "DELETE",
    headers: { Authorization: `Bearer ${token}` },
    cache: "no-store",
  });

  redirect(`/courses/${courseId}`);
}

// --- Notifications (Stage 12) ---

export async function markNotificationReadAction(notificationId: string) {
  const token = await getSessionToken();
  if (!token) {
    redirect("/login");
  }

  await fetch(`${SERVER_API_URL}/api/v1/me/notifications/${notificationId}/read`, {
    method: "PUT",
    headers: { Authorization: `Bearer ${token}` },
    cache: "no-store",
  });

  redirect("/dashboard/notifications");
}

export async function markAllNotificationsReadAction() {
  const token = await getSessionToken();
  if (!token) {
    redirect("/login");
  }

  await fetch(`${SERVER_API_URL}/api/v1/me/notifications/read-all`, {
    method: "PUT",
    headers: { Authorization: `Bearer ${token}` },
    cache: "no-store",
  });

  redirect("/dashboard/notifications");
}

// --- Assignments / homework (Stage 15) ---
// The backend re-validates enrollment/access/editability/emptiness on every
// call regardless of what the frontend sends — these actions just forward
// the form fields and surface whatever error comes back.

async function parseAssignmentError(res: Response, fallback: string): Promise<string> {
  const body = await res.json().catch(() => null);
  return body?.error?.message ?? fallback;
}

export async function saveAssignmentDraftAction(
  courseId: string,
  lessonId: string,
  assignmentId: string,
  _prevState: FormState,
  formData: FormData,
): Promise<FormState> {
  const token = await getSessionToken();
  if (!token) {
    redirect("/login");
  }

  const res = await fetch(`${SERVER_API_URL}/api/v1/assignments/${assignmentId}/submission`, {
    method: "PUT",
    headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json" },
    body: JSON.stringify({ text_content: String(formData.get("text_content") ?? "") }),
    cache: "no-store",
  });

  if (!res.ok) {
    return { error: await parseAssignmentError(res, "Не удалось сохранить черновик") };
  }

  redirect(`/learn/${courseId}/${lessonId}`);
}

export async function uploadAssignmentFileAction(
  courseId: string,
  lessonId: string,
  assignmentId: string,
  _prevState: FormState,
  formData: FormData,
): Promise<FormState> {
  const token = await getSessionToken();
  if (!token) {
    redirect("/login");
  }

  const file = formData.get("file");
  if (!(file instanceof File) || file.size === 0) {
    return { error: "Выберите файл" };
  }

  const uploadForm = new FormData();
  uploadForm.append("file", file, file.name);

  const res = await fetch(`${SERVER_API_URL}/api/v1/assignments/${assignmentId}/submission/files`, {
    method: "POST",
    headers: { Authorization: `Bearer ${token}` },
    body: uploadForm,
    cache: "no-store",
  });

  if (!res.ok) {
    return { error: await parseAssignmentError(res, "Не удалось загрузить файл") };
  }

  redirect(`/learn/${courseId}/${lessonId}`);
}

export interface DownloadUrlResult {
  url?: string;
  filename?: string;
  error?: string;
}

// Called from a client component on click — the backend re-checks
// ownership (own submission, or an instructor/admin who manages the
// course) on every call, never trusting anything the client asserts.
export async function getSubmissionFileDownloadUrlAction(fileId: string): Promise<DownloadUrlResult> {
  const token = await getSessionToken();
  if (!token) {
    return { error: "Не авторизован" };
  }

  const res = await fetch(`${SERVER_API_URL}/api/v1/submission-files/${fileId}/download`, {
    headers: { Authorization: `Bearer ${token}` },
    cache: "no-store",
  });

  if (!res.ok) {
    const body = await res.json().catch(() => null);
    return { error: body?.error?.message ?? "Не удалось получить файл" };
  }

  const data = await res.json();
  return { url: data.url, filename: data.filename };
}

export async function submitAssignmentAction(
  courseId: string,
  lessonId: string,
  assignmentId: string,
  _prevState: FormState,
): Promise<FormState> {
  const token = await getSessionToken();
  if (!token) {
    redirect("/login");
  }

  const res = await fetch(`${SERVER_API_URL}/api/v1/assignments/${assignmentId}/submission/submit`, {
    method: "POST",
    headers: { Authorization: `Bearer ${token}` },
    cache: "no-store",
  });

  if (!res.ok) {
    return { error: await parseAssignmentError(res, "Не удалось отправить задание") };
  }

  redirect(`/learn/${courseId}/${lessonId}`);
}

// --- Coding exercises / secure code runner (Stage 16) ---
// Called directly from a "use client" component (button onClick, poll
// interval) rather than via <form action=...> — there is no page
// navigation involved (item: Run/Submit must never reload or leave the
// page), so these are plain async functions returning data, the same shape
// getVideoProcessingStatusAction uses for VideoProcessingStatusPanel's poll
// loop, not the FormState/redirect shape the assignment actions above use.

export interface CodeSubmissionResult {
  submission?: CodeSubmission;
  error?: string;
}

async function parseCodeError(res: Response, fallback: string): Promise<string> {
  const body = await res.json().catch(() => null);
  if (body?.error?.code === "RATE_LIMITED") return "Слишком много попыток — подождите немного и попробуйте снова";
  if (body?.error?.code === "TOO_MANY_EXECUTIONS") return "У вас уже есть решение в очереди — дождитесь результата";
  return body?.error?.message ?? fallback;
}

export async function runCodeAction(exerciseId: string, sourceCode: string): Promise<CodeSubmissionResult> {
  const token = await getSessionToken();
  if (!token) {
    return { error: "Не авторизован" };
  }

  const res = await fetch(`${SERVER_API_URL}/api/v1/coding-exercises/${exerciseId}/run`, {
    method: "POST",
    headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json" },
    body: JSON.stringify({ source_code: sourceCode }),
    cache: "no-store",
  });

  if (!res.ok) {
    return { error: await parseCodeError(res, "Не удалось запустить код") };
  }
  return { submission: await res.json() };
}

export async function submitCodeAction(exerciseId: string, sourceCode: string): Promise<CodeSubmissionResult> {
  const token = await getSessionToken();
  if (!token) {
    return { error: "Не авторизован" };
  }

  const res = await fetch(`${SERVER_API_URL}/api/v1/coding-exercises/${exerciseId}/submit`, {
    method: "POST",
    headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json" },
    body: JSON.stringify({ source_code: sourceCode }),
    cache: "no-store",
  });

  if (!res.ok) {
    return { error: await parseCodeError(res, "Не удалось отправить решение") };
  }
  return { submission: await res.json() };
}

// Polled every 1-2s by the client from the moment Run/Submit returns until
// the submission reaches a terminal status (see TERMINAL_SUBMISSION_STATUSES) —
// no WebSocket, same interval-poll shape as VideoProcessingStatusPanel.
export async function getCodeSubmissionAction(submissionId: string): Promise<CodeSubmissionResult> {
  const token = await getSessionToken();
  if (!token) {
    return { error: "Не авторизован" };
  }

  const res = await fetch(`${SERVER_API_URL}/api/v1/code-submissions/${submissionId}`, {
    headers: { Authorization: `Bearer ${token}` },
    cache: "no-store",
  });

  if (!res.ok) {
    return { error: await parseCodeError(res, "Не удалось получить статус решения") };
  }
  return { submission: await res.json() };
}

export interface CodeAttemptsResult {
  attempts?: CodeSubmission[];
  error?: string;
}

export async function listCodeAttemptsAction(exerciseId: string): Promise<CodeAttemptsResult> {
  const token = await getSessionToken();
  if (!token) {
    return { error: "Не авторизован" };
  }

  const res = await fetch(`${SERVER_API_URL}/api/v1/coding-exercises/${exerciseId}/attempts`, {
    headers: { Authorization: `Bearer ${token}` },
    cache: "no-store",
  });

  if (!res.ok) {
    return { error: await parseCodeError(res, "Не удалось загрузить историю попыток") };
  }
  return { attempts: await res.json() };
}

// --- Timezone / profile (Stage 17 item 7) ---
// The backend re-validates the timezone as a real IANA zone name
// (time.LoadLocation) regardless of what's sent here — this action just
// forwards the browser's own Intl.DateTimeFormat().resolvedOptions().timeZone
// value (see TimezoneSync client component) and surfaces whatever error
// comes back.

export async function updateTimezoneAction(timezone: string): Promise<FormState> {
  const token = await getSessionToken();
  if (!token) {
    return { error: "Не авторизован" };
  }

  const res = await fetch(`${SERVER_API_URL}/api/v1/me/timezone`, {
    method: "PUT",
    headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json" },
    body: JSON.stringify({ timezone }),
    cache: "no-store",
  });

  if (!res.ok) {
    const body = await res.json().catch(() => null);
    return { error: body?.error?.message ?? "Не удалось обновить часовой пояс" };
  }
  return { error: null };
}

// --- Wishlist (Stage 18) ---
// Called directly from a "use client" component's onClick (WishlistButton),
// not via <form action=...> — no page navigation, just an optimistic toggle
// that reconciles with whatever the backend actually persisted. Add is
// idempotent on the backend, so these never need to distinguish "already
// there" from "just added".

export interface WishlistActionResult {
  in_wishlist?: boolean;
  error?: string;
}

async function parseWishlistError(res: Response, fallback: string): Promise<string> {
  const body = await res.json().catch(() => null);
  return body?.error?.message ?? fallback;
}

export async function addToWishlistAction(courseId: string): Promise<WishlistActionResult> {
  const token = await getSessionToken();
  if (!token) {
    return { error: "Не авторизован" };
  }

  const res = await fetch(`${SERVER_API_URL}/api/v1/courses/${courseId}/wishlist`, {
    method: "POST",
    headers: { Authorization: `Bearer ${token}` },
    cache: "no-store",
  });
  if (!res.ok) {
    return { error: await parseWishlistError(res, "Не удалось добавить в избранное") };
  }
  const data = await res.json();
  return { in_wishlist: Boolean(data.in_wishlist) };
}

export async function removeFromWishlistAction(courseId: string): Promise<WishlistActionResult> {
  const token = await getSessionToken();
  if (!token) {
    return { error: "Не авторизован" };
  }

  const res = await fetch(`${SERVER_API_URL}/api/v1/courses/${courseId}/wishlist`, {
    method: "DELETE",
    headers: { Authorization: `Bearer ${token}` },
    cache: "no-store",
  });
  if (!res.ok) {
    return { error: await parseWishlistError(res, "Не удалось убрать из избранного") };
  }
  const data = await res.json();
  return { in_wishlist: Boolean(data.in_wishlist) };
}

// --- Recommendation feedback (Stage 23A2 backend, Stage 23B1 frontend) -----

export interface RecommendationFeedbackResult {
  ok: boolean;
  error?: string;
}

async function parseRecommendationFeedbackError(res: Response, fallback: string): Promise<string> {
  const body = await res.json().catch(() => null);
  return body?.error?.message ?? fallback;
}

// submitRecommendationFeedbackAction backs the "Скрыть"/"Не интересно"
// buttons on personalized recommendation cards. Same shape as
// addToWishlistAction/removeFromWishlistAction above: no client-supplied
// user id anywhere (the backend reads it from the verified JWT), and the
// call is idempotent/upsert-safe on the backend side (Stage 23A1's
// UpsertFeedback), so retrying or switching action for the same course is
// always safe.
export async function submitRecommendationFeedbackAction(
  courseId: string,
  action: "dismiss" | "not_interested",
): Promise<RecommendationFeedbackResult> {
  const token = await getSessionToken();
  if (!token) {
    return { ok: false, error: "Не авторизован" };
  }

  const res = await fetch(`${SERVER_API_URL}/api/v1/recommendations/${courseId}/feedback`, {
    method: "POST",
    headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json" },
    body: JSON.stringify({ action }),
    cache: "no-store",
  });
  if (!res.ok) {
    return { ok: false, error: await parseRecommendationFeedbackError(res, "Не удалось сохранить отметку") };
  }
  return { ok: true };
}

// --- Lesson Q&A (Stage 20B1) -------------------------------------------
// Same shape as the wishlist actions above (return a result object, never
// redirect) since the lesson page's Q&A section needs to update in place —
// asking/answering/deleting must not navigate the student away from the
// video/progress/assignment they're in the middle of.

async function parseQAError(res: Response, fallback: string): Promise<string> {
  const body = await res.json().catch(() => null);
  return body?.error?.message ?? fallback;
}

export interface AskQuestionResult {
  data?: QAQuestion;
  error?: string;
}

export async function askQuestionAction(lessonId: string, body: string): Promise<AskQuestionResult> {
  const token = await getSessionToken();
  if (!token) {
    return { error: "Не авторизован" };
  }

  const res = await fetch(`${SERVER_API_URL}/api/v1/lessons/${lessonId}/questions`, {
    method: "POST",
    headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json" },
    body: JSON.stringify({ body }),
    cache: "no-store",
  });
  if (!res.ok) {
    return { error: await parseQAError(res, "Не удалось задать вопрос") };
  }
  return { data: await res.json() };
}

export interface AnswerQuestionResult {
  data?: QAAnswer;
  error?: string;
}

export async function answerQuestionAction(questionId: string, body: string): Promise<AnswerQuestionResult> {
  const token = await getSessionToken();
  if (!token) {
    return { error: "Не авторизован" };
  }

  const res = await fetch(`${SERVER_API_URL}/api/v1/questions/${questionId}/answers`, {
    method: "POST",
    headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json" },
    body: JSON.stringify({ body }),
    cache: "no-store",
  });
  if (!res.ok) {
    return { error: await parseQAError(res, "Не удалось отправить ответ") };
  }
  return { data: await res.json() };
}

export interface QAMutationResult {
  ok: boolean;
  error?: string;
}

export async function deleteQuestionAction(questionId: string): Promise<QAMutationResult> {
  const token = await getSessionToken();
  if (!token) {
    return { ok: false, error: "Не авторизован" };
  }

  const res = await fetch(`${SERVER_API_URL}/api/v1/questions/${questionId}`, {
    method: "DELETE",
    headers: { Authorization: `Bearer ${token}` },
    cache: "no-store",
  });
  if (!res.ok) {
    return { ok: false, error: await parseQAError(res, "Не удалось удалить вопрос") };
  }
  return { ok: true };
}

export async function deleteAnswerAction(answerId: string): Promise<QAMutationResult> {
  const token = await getSessionToken();
  if (!token) {
    return { ok: false, error: "Не авторизован" };
  }

  const res = await fetch(`${SERVER_API_URL}/api/v1/answers/${answerId}`, {
    method: "DELETE",
    headers: { Authorization: `Bearer ${token}` },
    cache: "no-store",
  });
  if (!res.ok) {
    return { ok: false, error: await parseQAError(res, "Не удалось удалить ответ") };
  }
  return { ok: true };
}

export interface SetQuestionPublishedResult {
  data?: QAQuestion;
  error?: string;
}

// setQuestionPublishedAction backs the moderation hide/show toggle (Stage
// 21B1's backend). Deliberately calls the "/instructor" route, not
// "/admin" — that group's RequireAnyRole("instructor", "admin") accepts
// both callers, and the backend's own ownership.CanManageCourse check
// (admin: any course; instructor: only courses they own) is what actually
// decides authorization, exactly like askQuestionAction/answerQuestionAction
// above already share one action across both moderation pages instead of
// needing a role-specific variant.
export async function setQuestionPublishedAction(questionId: string, published: boolean): Promise<SetQuestionPublishedResult> {
  const token = await getSessionToken();
  if (!token) {
    return { error: "Не авторизован" };
  }

  const res = await fetch(`${SERVER_API_URL}/api/v1/instructor/qa/questions/${questionId}`, {
    method: "PUT",
    headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json" },
    body: JSON.stringify({ published }),
    cache: "no-store",
  });
  if (!res.ok) {
    return { error: await parseQAError(res, "Не удалось изменить статус вопроса") };
  }
  return { data: await res.json() };
}

export interface SetAnswerPublishedResult {
  data?: QAAnswer;
  error?: string;
}

export async function setAnswerPublishedAction(answerId: string, published: boolean): Promise<SetAnswerPublishedResult> {
  const token = await getSessionToken();
  if (!token) {
    return { error: "Не авторизован" };
  }

  const res = await fetch(`${SERVER_API_URL}/api/v1/instructor/qa/answers/${answerId}`, {
    method: "PUT",
    headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json" },
    body: JSON.stringify({ published }),
    cache: "no-store",
  });
  if (!res.ok) {
    return { error: await parseQAError(res, "Не удалось изменить статус ответа") };
  }
  return { data: await res.json() };
}

export interface LoadMoreQuestionsResult {
  data?: PageResult<QAQuestionView>;
  error?: string;
}

// loadMoreQuestionsAction backs the "Показать ещё вопросы" button — the
// lesson page's initial page of questions is fetched server-side (see
// app/learn/[courseId]/[lessonId]/page.tsx), this only ever runs for
// subsequent pages a student explicitly asks for.
export async function loadMoreQuestionsAction(lessonId: string, page: number): Promise<LoadMoreQuestionsResult> {
  const token = await getSessionToken();
  if (!token) {
    return { error: "Не авторизован" };
  }

  const res = await fetch(`${SERVER_API_URL}/api/v1/lessons/${lessonId}/questions?page=${page}&limit=20`, {
    headers: { Authorization: `Bearer ${token}` },
    cache: "no-store",
  });
  if (!res.ok) {
    return { error: await parseQAError(res, "Не удалось загрузить вопросы") };
  }
  return { data: await res.json() };
}
