import "server-only";
import {
  SERVER_API_URL,
  type Category,
  type Course,
  type CourseDetail,
  type Plan,
  type PublicUser,
  type Speciality,
  type SpecialityDetail,
} from "@/lib/api";

function authHeaders(token: string) {
  return { Authorization: `Bearer ${token}` };
}

export interface AdminLessonVideo {
  id: string;
  lesson_id: string;
  storage_provider: string;
  bucket: string;
  original_filename: string;
  mime_type: string;
  size_bytes: number;
  duration_seconds: number | null;
  status: "uploading" | "ready" | "failed";
  created_at: string;
  updated_at: string;
  processing_status: "uploaded" | "processing" | "ready" | "failed";
  processing_error?: string;
  processed_at?: string;
  is_active: boolean;
}

// Returns null when the lesson has no video yet — not an error state.
export async function adminGetLessonVideo(token: string, lessonId: string): Promise<AdminLessonVideo | null> {
  const res = await fetch(`${SERVER_API_URL}/api/v1/admin/lessons/${lessonId}/video`, {
    headers: authHeaders(token),
    cache: "no-store",
  });
  if (res.status === 404) return null;
  if (!res.ok) throw new Error(await parseErrorMessage(res));
  return res.json();
}

export interface RenditionStatus {
  quality: string;
  status: "pending" | "processing" | "ready" | "failed";
}

export interface AttemptStatus {
  status: "uploaded" | "processing" | "ready" | "failed";
  error?: string;
  renditions: RenditionStatus[];
}

export interface VideoProcessingStatus {
  active?: AttemptStatus;
  pending?: AttemptStatus;
}

// Returns null when the lesson has no video at all yet.
export async function adminGetVideoProcessingStatus(token: string, lessonId: string): Promise<VideoProcessingStatus | null> {
  const res = await fetch(`${SERVER_API_URL}/api/v1/admin/lessons/${lessonId}/video/status`, {
    headers: authHeaders(token),
    cache: "no-store",
  });
  if (res.status === 404) return null;
  if (!res.ok) throw new Error(await parseErrorMessage(res));
  return res.json();
}

export interface PageResult<T> {
  items: T[];
  page: number;
  limit: number;
  total: number;
  total_pages: number;
}

export interface AdminStats {
  users_count: number;
  courses_count: number;
  published_courses_count: number;
  specialities_count: number;
  enrollments_count: number;
  completed_courses_count: number;
  certificates_count: number;
  test_attempts_count: number;
  daily_active_learners: number;
  weekly_active_learners: number;
  lessons_completed_today: number;
  courses_completed_this_month: number;
  certificates_this_month: number;
}

export interface AdminTestSummary {
  id: string;
  course_id?: string;
  module_id?: string;
  lesson_id?: string;
  title: string;
  description: string;
  passing_score: number;
  published: boolean;
  is_final: boolean;
  created_at: string;
  updated_at: string;
  course_title?: string;
}

export interface AdminAnswer {
  id: string;
  question_id: string;
  text: string;
  is_correct: boolean;
  position: number;
  created_at: string;
}

export interface AdminQuestion {
  id: string;
  test_id: string;
  text: string;
  position: number;
  answers: AdminAnswer[];
}

export interface AdminTestDetail extends AdminTestSummary {
  questions: AdminQuestion[];
}

export interface AdminCertificate {
  id: string;
  certificate_number: string;
  student_name: string;
  student_email: string;
  course_title: string;
  issued_at: string;
}

export interface ApiError {
  error?: { code: string; message: string };
}

async function parseErrorMessage(res: Response): Promise<string> {
  const body: ApiError | null = await res.json().catch(() => null);
  return body?.error?.message ?? `HTTP ${res.status}`;
}

export async function adminGetStats(token: string): Promise<AdminStats> {
  const res = await fetch(`${SERVER_API_URL}/api/v1/admin/stats`, {
    headers: authHeaders(token),
    cache: "no-store",
  });
  if (!res.ok) throw new Error(await parseErrorMessage(res));
  return res.json();
}

export async function adminListUsers(
  token: string,
  params: { page?: number; limit?: number; search?: string },
): Promise<PageResult<PublicUser>> {
  const qs = new URLSearchParams();
  if (params.page) qs.set("page", String(params.page));
  if (params.limit) qs.set("limit", String(params.limit));
  if (params.search) qs.set("search", params.search);

  const res = await fetch(`${SERVER_API_URL}/api/v1/admin/users?${qs}`, {
    headers: authHeaders(token),
    cache: "no-store",
  });
  if (!res.ok) throw new Error(await parseErrorMessage(res));
  return res.json();
}

// adminListInstructors backs the instructor selector in the admin course
// form. There is no role filter on GET /admin/users — the catalog of
// instructors is small enough that fetching one large page and filtering
// here is simpler than adding a backend query param for it.
export async function adminListInstructors(token: string): Promise<PublicUser[]> {
  const result = await adminListUsers(token, { limit: 100 });
  return result.items.filter((u) => u.role === "instructor");
}

export async function adminGetUser(token: string, id: string): Promise<PublicUser | null> {
  const res = await fetch(`${SERVER_API_URL}/api/v1/admin/users/${id}`, {
    headers: authHeaders(token),
    cache: "no-store",
  });
  if (res.status === 404) return null;
  if (!res.ok) throw new Error(await parseErrorMessage(res));
  return res.json();
}

export async function adminListCourses(
  token: string,
  params: { page?: number; limit?: number; search?: string },
): Promise<PageResult<Course>> {
  const qs = new URLSearchParams();
  if (params.page) qs.set("page", String(params.page));
  if (params.limit) qs.set("limit", String(params.limit));
  if (params.search) qs.set("search", params.search);

  const res = await fetch(`${SERVER_API_URL}/api/v1/admin/courses?${qs}`, {
    headers: authHeaders(token),
    cache: "no-store",
  });
  if (!res.ok) throw new Error(await parseErrorMessage(res));
  return res.json();
}

export async function adminGetCourse(token: string, id: string): Promise<CourseDetail | null> {
  const res = await fetch(`${SERVER_API_URL}/api/v1/admin/courses/${id}`, {
    headers: authHeaders(token),
    cache: "no-store",
  });
  if (res.status === 404) return null;
  if (!res.ok) throw new Error(await parseErrorMessage(res));
  return res.json();
}

export async function adminListCategories(token: string): Promise<Category[]> {
  const res = await fetch(`${SERVER_API_URL}/api/v1/admin/categories`, {
    headers: authHeaders(token),
    cache: "no-store",
  });
  if (!res.ok) throw new Error(await parseErrorMessage(res));
  return res.json();
}

export async function adminGetCategory(token: string, id: string): Promise<Category | null> {
  const categories = await adminListCategories(token);
  return categories.find((c) => c.id === id) ?? null;
}

export interface AdminReview {
  id: string;
  user_id: string;
  user_name: string;
  course_id: string;
  course_title: string;
  rating: number;
  review_text?: string;
  published: boolean;
  created_at: string;
  updated_at: string;
}

export async function adminListReviews(
  token: string,
  params: { page?: number; limit?: number; course_id?: string; rating?: number },
): Promise<PageResult<AdminReview>> {
  const qs = new URLSearchParams();
  if (params.page) qs.set("page", String(params.page));
  if (params.limit) qs.set("limit", String(params.limit));
  if (params.course_id) qs.set("course_id", params.course_id);
  if (params.rating) qs.set("rating", String(params.rating));

  const res = await fetch(`${SERVER_API_URL}/api/v1/admin/reviews?${qs}`, {
    headers: authHeaders(token),
    cache: "no-store",
  });
  if (!res.ok) throw new Error(await parseErrorMessage(res));
  return res.json();
}

// adminListCourseSubmissions is the instructor-authoring moderation queue —
// courses currently pending_review. Deliberately distinct from student
// course reviews (see internal/reviews) — never call this "reviews" in the
// UI, to avoid confusing the two.
export async function adminListCourseSubmissions(
  token: string,
  params: { page?: number; limit?: number },
): Promise<PageResult<Course>> {
  const qs = new URLSearchParams();
  if (params.page) qs.set("page", String(params.page));
  if (params.limit) qs.set("limit", String(params.limit));

  const res = await fetch(`${SERVER_API_URL}/api/v1/admin/course-submissions?${qs}`, {
    headers: authHeaders(token),
    cache: "no-store",
  });
  if (!res.ok) throw new Error(await parseErrorMessage(res));
  return res.json();
}

export async function adminListSpecialities(token: string): Promise<Speciality[]> {
  const res = await fetch(`${SERVER_API_URL}/api/v1/admin/specialities`, {
    headers: authHeaders(token),
    cache: "no-store",
  });
  if (!res.ok) throw new Error(await parseErrorMessage(res));
  return res.json();
}

export async function adminGetSpeciality(token: string, id: string): Promise<SpecialityDetail | null> {
  const res = await fetch(`${SERVER_API_URL}/api/v1/specialities/${id}`, {
    headers: authHeaders(token),
    cache: "no-store",
  });
  if (res.status === 404) return null;
  if (!res.ok) throw new Error(await parseErrorMessage(res));
  return res.json();
}

export async function adminListTests(token: string): Promise<AdminTestSummary[]> {
  const res = await fetch(`${SERVER_API_URL}/api/v1/admin/tests`, {
    headers: authHeaders(token),
    cache: "no-store",
  });
  if (!res.ok) throw new Error(await parseErrorMessage(res));
  return res.json();
}

export async function adminGetTest(token: string, id: string): Promise<AdminTestDetail | null> {
  const res = await fetch(`${SERVER_API_URL}/api/v1/admin/tests/${id}`, {
    headers: authHeaders(token),
    cache: "no-store",
  });
  if (res.status === 404) return null;
  if (!res.ok) throw new Error(await parseErrorMessage(res));
  return res.json();
}

export interface AdminSubscription {
  id: string;
  user_id: string;
  plan_id: string;
  status: string;
  starts_at: string | null;
  expires_at: string | null;
  canceled_at: string | null;
  created_at: string;
  updated_at: string;
  user_email: string;
  user_name: string;
  plan_name: string;
}

export interface AdminPayment {
  id: string;
  user_id: string;
  subscription_id: string | null;
  provider: string;
  provider_payment_id: string | null;
  amount: number;
  currency: string;
  status: string;
  idempotency_key: string;
  created_at: string;
  paid_at: string | null;
  failed_at: string | null;
  user_email: string;
  user_name: string;
}

export async function adminListPlans(token: string): Promise<Plan[]> {
  const res = await fetch(`${SERVER_API_URL}/api/v1/admin/plans`, {
    headers: authHeaders(token),
    cache: "no-store",
  });
  if (!res.ok) throw new Error(await parseErrorMessage(res));
  return res.json();
}

export async function adminGetPlan(token: string, id: string): Promise<Plan | null> {
  const plans = await adminListPlans(token);
  return plans.find((p) => p.id === id) ?? null;
}

export async function adminListSubscriptions(
  token: string,
  params: { page?: number; limit?: number; status?: string },
): Promise<PageResult<AdminSubscription>> {
  const qs = new URLSearchParams();
  if (params.page) qs.set("page", String(params.page));
  if (params.limit) qs.set("limit", String(params.limit));
  if (params.status) qs.set("status", params.status);

  const res = await fetch(`${SERVER_API_URL}/api/v1/admin/subscriptions?${qs}`, {
    headers: authHeaders(token),
    cache: "no-store",
  });
  if (!res.ok) throw new Error(await parseErrorMessage(res));
  return res.json();
}

export async function adminListPayments(
  token: string,
  params: { page?: number; limit?: number; status?: string },
): Promise<PageResult<AdminPayment>> {
  const qs = new URLSearchParams();
  if (params.page) qs.set("page", String(params.page));
  if (params.limit) qs.set("limit", String(params.limit));
  if (params.status) qs.set("status", params.status);

  const res = await fetch(`${SERVER_API_URL}/api/v1/admin/payments?${qs}`, {
    headers: authHeaders(token),
    cache: "no-store",
  });
  if (!res.ok) throw new Error(await parseErrorMessage(res));
  return res.json();
}

export async function adminListCertificates(
  token: string,
  params: { page?: number; limit?: number; search?: string },
): Promise<PageResult<AdminCertificate>> {
  const qs = new URLSearchParams();
  if (params.page) qs.set("page", String(params.page));
  if (params.limit) qs.set("limit", String(params.limit));
  if (params.search) qs.set("search", params.search);

  const res = await fetch(`${SERVER_API_URL}/api/v1/admin/certificates?${qs}`, {
    headers: authHeaders(token),
    cache: "no-store",
  });
  if (!res.ok) throw new Error(await parseErrorMessage(res));
  return res.json();
}

export interface AdminNotificationJob {
  id: string;
  user_id: string;
  user_email: string;
  type: string;
  payload?: Record<string, unknown>;
  channel: "in_app" | "email";
  status: "pending" | "processing" | "completed" | "failed";
  attempts: number;
  available_at: string;
  started_at?: string;
  finished_at?: string;
  last_error?: string;
  created_at: string;
}

// CurrencyRevenue mirrors backend/internal/subscriptions/model.go exactly.
// Every field is scoped to a single currency — this shape is never reduced
// to one combined total across currencies, on the backend or here.
export interface CurrencyRevenue {
  currency: string;
  paid_amount: number;
  paid_count: number;
  refunded_amount: number;
  refunded_count: number;
  new_subscriptions: number;
  canceled_subscriptions: number;
  active_subscriptions: number;
  mrr: number;
}

// PlanRevenue mirrors backend/internal/subscriptions/model.go (Stage 19C).
// Like CurrencyRevenue, each row carries its own currency and is never
// combined with a row of a different currency.
export interface PlanRevenue {
  plan_id: string;
  plan_name: string;
  currency: string;
  active_subscriptions: number;
  new_subscriptions: number;
  canceled_subscriptions: number;
  paid_amount: number;
  paid_count: number;
}

export interface AdminRevenueAnalytics {
  from: string;
  to: string;
  by_currency: CurrencyRevenue[];
  plan_breakdown: PlanRevenue[];
}

// adminGetRevenueAnalytics throws Error("UNAUTHORIZED")/Error("FORBIDDEN") on
// 401/403 specifically (rather than falling through to the generic
// parseErrorMessage path) so the page can render a distinct message for
// "your session expired" vs. "you don't have access" vs. any other failure —
// see app/admin/analytics/page.tsx.
export async function adminGetRevenueAnalytics(
  token: string,
  params: { from?: string; to?: string },
): Promise<AdminRevenueAnalytics> {
  const qs = new URLSearchParams();
  if (params.from) qs.set("from", params.from);
  if (params.to) qs.set("to", params.to);

  const res = await fetch(`${SERVER_API_URL}/api/v1/admin/analytics/revenue?${qs}`, {
    headers: authHeaders(token),
    cache: "no-store",
  });
  if (res.status === 401) throw new Error("UNAUTHORIZED");
  if (res.status === 403) throw new Error("FORBIDDEN");
  if (!res.ok) throw new Error(await parseErrorMessage(res));
  return res.json();
}

export async function adminListNotificationJobs(
  token: string,
  params: { page?: number; limit?: number; status?: string; channel?: string },
): Promise<PageResult<AdminNotificationJob>> {
  const qs = new URLSearchParams();
  if (params.page) qs.set("page", String(params.page));
  if (params.limit) qs.set("limit", String(params.limit));
  if (params.status) qs.set("status", params.status);
  if (params.channel) qs.set("channel", params.channel);

  const res = await fetch(`${SERVER_API_URL}/api/v1/admin/notification-jobs?${qs}`, {
    headers: authHeaders(token),
    cache: "no-store",
  });
  if (!res.ok) throw new Error(await parseErrorMessage(res));
  return res.json();
}
