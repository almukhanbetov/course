const PUBLIC_API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";
export const SERVER_API_URL = process.env.API_INTERNAL_URL ?? PUBLIC_API_URL;

function apiBaseUrl(): string {
  return typeof window === "undefined" ? SERVER_API_URL : PUBLIC_API_URL;
}

export interface HealthStatus {
  status: string;
  database: string;
}

// title/description (or name) stay the canonical, always-present Russian
// fields; these are optional per-locale overrides the backend returns only
// where a real translation exists — see lib/i18n/localize.ts for how the
// frontend picks the right one with a fallback to Russian.
export interface LocalizedTitle {
  title_kk?: string;
  title_en?: string;
}

export interface LocalizedDescription {
  description_kk?: string;
  description_en?: string;
}

export interface Course extends LocalizedTitle, LocalizedDescription {
  id: string;
  title: string;
  slug: string;
  description: string;
  level: string;
  image_url: string;
  published: boolean;
  access_type: "free" | "subscription";
  created_at: string;
  updated_at: string;
  category_id?: string;
  category_name?: string;
  category_slug?: string;
  rating_average: number;
  rating_count: number;
  instructor_id?: string;
  instructor_name?: string;
  publication_status: "draft" | "pending_review" | "published" | "rejected";
  rejection_reason?: string;
}

export interface Category {
  id: string;
  name: string;
  slug: string;
  description?: string;
  position: number;
  active: boolean;
  created_at: string;
  updated_at: string;
  name_kk?: string;
  name_en?: string;
}

export interface CourseListParams {
  q?: string;
  category?: string;
  level?: string;
  access_type?: string;
  sort?: string;
  page?: number;
  limit?: number;
}

export interface PublicReview {
  id: string;
  display_name: string;
  rating: number;
  review_text?: string;
  created_at: string;
}

export interface MyReview {
  id: string;
  user_id: string;
  course_id: string;
  rating: number;
  review_text?: string;
  published: boolean;
  created_at: string;
  updated_at: string;
}

export interface Lesson {
  id: string;
  module_id: string;
  title: string;
  slug: string;
  description: string;
  video_url: string;
  duration_seconds: number;
  position: number;
  is_free: boolean;
  published: boolean;
  created_at: string;
  updated_at: string;
}

export interface Module {
  id: string;
  course_id: string;
  title: string;
  position: number;
  created_at: string;
  updated_at: string;
  lessons: Lesson[];
}

export interface CourseDetail extends Course {
  modules: Module[];
}

export interface PublicUser {
  id: string;
  email: string;
  first_name: string;
  last_name: string;
  role: string;
  active: boolean;
  timezone: string;
  created_at: string;
  updated_at: string;
}

export interface EnrolledCourse {
  course_id: string;
  title: string;
  slug: string;
  image_url: string;
  enrolled_at: string;
  completed_at?: string;
  progress_percent: number;
  completed_lessons: number;
  total_lessons: number;
  next_lesson_id?: string;
  lessons_progress_percent: number;
  has_final_test: boolean;
  final_test_id?: string;
  final_test_passed: boolean;
  final_test_best_score?: number;
  completed: boolean;
}

export interface MyLesson {
  id: string;
  title: string;
  slug: string;
  description: string;
  video_url: string;
  duration_seconds: number;
  position: number;
  is_free: boolean;
  published: boolean;
  progress_seconds: number;
  // completed is the assignment-aware truth (Stage 15): video_completed
  // AND, if this lesson has a required published assignment, that
  // assignment is approved. Lessons with no required assignment behave
  // exactly as before (completed === video_completed).
  completed: boolean;
  // video_status is "ready" once an admin-uploaded video exists for this
  // lesson (Stage 10 object storage flow) — absent/undefined means no
  // video has been uploaded, so the player should not be shown.
  video_status?: "uploading" | "ready" | "failed";

  video_completed: boolean;
  assignment_required: boolean;
  assignment_approved: boolean;
  assignment_status?: "draft" | "submitted" | "needs_revision" | "approved";

  // Stage 16: same "one boolean doesn't decide completion" convention as
  // assignment_* above — coding_exercise_passed tracks whether ANY
  // submit-mode submission has ever passed (a later failed re-submit must
  // never un-complete the lesson), while coding_exercise_status is only the
  // latest submission's status, for display.
  coding_exercise_required: boolean;
  coding_exercise_passed: boolean;
  coding_exercise_status?: CodeSubmissionStatus;
}

export interface MyModule {
  id: string;
  title: string;
  position: number;
  lessons: MyLesson[];
}

export interface MyCourseDetail {
  course_id: string;
  title: string;
  slug: string;
  description: string;
  level: string;
  image_url: string;
  enrolled_at: string;
  completed_at?: string;
  progress_percent: number;
  completed_lessons: number;
  total_lessons: number;
  modules: MyModule[];
  lessons_progress_percent: number;
  has_final_test: boolean;
  final_test_id?: string;
  final_test_passed: boolean;
  completed: boolean;
}

export interface LessonProgress {
  lesson_id: string;
  progress_seconds: number;
  completed: boolean;
  completed_at?: string;
}

export interface Speciality extends LocalizedTitle, LocalizedDescription {
  id: string;
  title: string;
  slug: string;
  description: string;
  image_url: string;
  published: boolean;
  created_at: string;
  updated_at: string;
}

export interface SpecialityCourse extends LocalizedTitle, LocalizedDescription {
  id: string;
  title: string;
  slug: string;
  description: string;
  level: string;
  position: number;
  required: boolean;
}

export interface SpecialityDetail extends Speciality {
  courses: SpecialityCourse[];
}

export interface RoadmapCourseProgress {
  course_id: string;
  title: string;
  slug: string;
  position: number;
  required: boolean;
  progress_percent: number;
  completed: boolean;
}

export interface MyRoadmap {
  speciality_id: string;
  title: string;
  slug: string;
  description: string;
  progress_percent: number;
  completed: boolean;
  courses: RoadmapCourseProgress[];
}

export async function getHealth(): Promise<HealthStatus> {
  const res = await fetch(`${apiBaseUrl()}/api/v1/health`, {
    cache: "no-store",
  });

  if (!res.ok) {
    throw new Error(`Health check failed with status ${res.status}`);
  }

  return res.json();
}

export async function getCourses(params: CourseListParams = {}): Promise<PageResult<Course>> {
  const query = new URLSearchParams();
  if (params.q) query.set("q", params.q);
  if (params.category) query.set("category", params.category);
  if (params.level) query.set("level", params.level);
  if (params.access_type) query.set("access_type", params.access_type);
  if (params.sort) query.set("sort", params.sort);
  if (params.page) query.set("page", String(params.page));
  if (params.limit) query.set("limit", String(params.limit));

  const qs = query.toString();
  const res = await fetch(`${apiBaseUrl()}/api/v1/courses${qs ? `?${qs}` : ""}`, {
    cache: "no-store",
  });

  if (!res.ok) {
    throw new Error(`Failed to load courses: ${res.status}`);
  }

  return res.json();
}

// CourseSuggestion mirrors backend/internal/courses/model.go's
// CourseSuggestion exactly (Stage 22A1) — the narrow shape
// GET /search/suggestions returns: just enough to render and link a
// suggestion, not the full Course shape getCourses returns.
export interface CourseSuggestion {
  id: string;
  title: string;
  slug: string;
  category_name?: string;
}

// getCourseSuggestions backs search-as-you-type (Stage 22B1). Public,
// unauthenticated, same as getCourses/getCategories above — callable from
// both server and client code since apiBaseUrl() resolves to the right
// base URL either way. Accepts an optional AbortSignal so the autocomplete
// component can cancel a stale in-flight request when the user keeps
// typing, rather than risking an older response overwriting a newer one.
export async function getCourseSuggestions(query: string, signal?: AbortSignal): Promise<CourseSuggestion[]> {
  const res = await fetch(`${apiBaseUrl()}/api/v1/search/suggestions?q=${encodeURIComponent(query)}`, {
    cache: "no-store",
    signal,
  });

  if (!res.ok) {
    throw new Error(`Failed to load suggestions: ${res.status}`);
  }

  return res.json();
}

export async function getCategories(): Promise<Category[]> {
  const res = await fetch(`${apiBaseUrl()}/api/v1/categories`, {
    cache: "no-store",
  });

  if (!res.ok) {
    throw new Error(`Failed to load categories: ${res.status}`);
  }

  return res.json();
}

export async function getCourseReviews(courseId: string, page = 1, limit = 20): Promise<PageResult<PublicReview>> {
  const res = await fetch(`${apiBaseUrl()}/api/v1/courses/${courseId}/reviews?page=${page}&limit=${limit}`, {
    cache: "no-store",
  });

  if (!res.ok) {
    throw new Error(`Failed to load reviews: ${res.status}`);
  }

  return res.json();
}

// Returns null when the user hasn't reviewed this course yet (404).
export async function getMyReview(token: string, courseId: string): Promise<MyReview | null> {
  const res = await fetch(`${apiBaseUrl()}/api/v1/courses/${courseId}/reviews/me`, {
    headers: { Authorization: `Bearer ${token}` },
    cache: "no-store",
  });

  if (res.status === 404) {
    return null;
  }

  if (!res.ok) {
    throw new Error(`Failed to load your review: ${res.status}`);
  }

  return res.json();
}

export async function getCourse(id: string): Promise<CourseDetail | null> {
  const res = await fetch(`${apiBaseUrl()}/api/v1/courses/${id}`, {
    cache: "no-store",
  });

  if (res.status === 404) {
    return null;
  }

  if (!res.ok) {
    throw new Error(`Failed to load course: ${res.status}`);
  }

  return res.json();
}

export async function getMyCourses(token: string): Promise<EnrolledCourse[]> {
  const res = await fetch(`${apiBaseUrl()}/api/v1/me/courses`, {
    headers: { Authorization: `Bearer ${token}` },
    cache: "no-store",
  });

  if (!res.ok) {
    throw new Error(`Failed to load my courses: ${res.status}`);
  }

  return res.json();
}

// Returns null when the user is not enrolled (403) or the course doesn't exist (404).
export async function getMyCourseDetail(token: string, courseId: string): Promise<MyCourseDetail | null> {
  const res = await fetch(`${apiBaseUrl()}/api/v1/me/courses/${courseId}`, {
    headers: { Authorization: `Bearer ${token}` },
    cache: "no-store",
  });

  if (res.status === 403 || res.status === 404) {
    return null;
  }

  if (!res.ok) {
    throw new Error(`Failed to load course: ${res.status}`);
  }

  return res.json();
}

export async function getSpecialities(): Promise<Speciality[]> {
  const res = await fetch(`${apiBaseUrl()}/api/v1/specialities`, {
    cache: "no-store",
  });

  if (!res.ok) {
    throw new Error(`Failed to load specialities: ${res.status}`);
  }

  return res.json();
}

export async function getSpeciality(id: string): Promise<SpecialityDetail | null> {
  const res = await fetch(`${apiBaseUrl()}/api/v1/specialities/${id}`, {
    cache: "no-store",
  });

  if (res.status === 404) {
    return null;
  }

  if (!res.ok) {
    throw new Error(`Failed to load speciality: ${res.status}`);
  }

  return res.json();
}

// PublicAnswer/PublicQuestion/PublicTest never carry a correctness flag —
// the backend response they mirror doesn't select is_correct at all.
export interface PublicAnswer {
  id: string;
  text: string;
  position: number;
}

export interface PublicQuestion {
  id: string;
  text: string;
  position: number;
  answers: PublicAnswer[];
}

export interface PublicTest {
  id: string;
  title: string;
  description: string;
  passing_score: number;
  is_final: boolean;
  questions: PublicQuestion[];
}

export interface AttemptAnswerReview {
  question_id: string;
  question_text: string;
  selected_answer_id: string;
  selected_answer_text: string;
  correct_answer_id: string;
  correct_answer_text: string;
  correct: boolean;
}

export interface AttemptDetail {
  id: string;
  test_id: string;
  user_id: string;
  score: number;
  passed: boolean;
  started_at: string;
  completed_at: string;
  created_at: string;
  test_title: string;
  passing_score: number;
  total_questions: number;
  answers: AttemptAnswerReview[];
}

export type TestAccessResult =
  | { kind: "ok"; test: PublicTest }
  | { kind: "not_enrolled" }
  | { kind: "lessons_not_completed" }
  | { kind: "not_found" }
  | { kind: "error"; message: string };

export async function getTest(token: string, testId: string): Promise<TestAccessResult> {
  const res = await fetch(`${apiBaseUrl()}/api/v1/tests/${testId}`, {
    headers: { Authorization: `Bearer ${token}` },
    cache: "no-store",
  });

  if (res.ok) {
    return { kind: "ok", test: await res.json() };
  }

  const body = await res.json().catch(() => null);
  const code = body?.error?.code;

  if (code === "NOT_ENROLLED") return { kind: "not_enrolled" };
  if (code === "LESSONS_NOT_COMPLETED") return { kind: "lessons_not_completed" };
  if (code === "TEST_NOT_FOUND") return { kind: "not_found" };
  return { kind: "error", message: body?.error?.message ?? `HTTP ${res.status}` };
}

// --- Assignments / homework (Stage 15) ---

export interface StudentAssignment {
  id: string;
  lesson_id: string;
  title: string;
  description: string;
  instructions: string;
  required: boolean;
  max_score?: number;
}

export interface SubmissionFile {
  id: string;
  original_filename: string;
  mime_type: string;
  size_bytes: number;
  created_at: string;
}

export interface Submission {
  id: string;
  assignment_id: string;
  user_id: string;
  text_content?: string;
  status: "draft" | "submitted" | "needs_revision" | "approved";
  score?: number;
  instructor_feedback?: string;
  submitted_at?: string;
  reviewed_at?: string;
  reviewed_by?: string;
  created_at: string;
  updated_at: string;
  files: SubmissionFile[];
}

// Returns null when this lesson has no (published) assignment.
export async function getLessonAssignment(token: string, lessonId: string): Promise<StudentAssignment | null> {
  const res = await fetch(`${apiBaseUrl()}/api/v1/lessons/${lessonId}/assignment`, {
    headers: { Authorization: `Bearer ${token}` },
    cache: "no-store",
  });
  if (res.status === 404 || res.status === 403) return null;
  if (!res.ok) throw new Error(`Failed to load assignment: ${res.status}`);
  return res.json();
}

// Returns null when the student hasn't started this assignment yet.
export async function getMySubmission(token: string, assignmentId: string): Promise<Submission | null> {
  const res = await fetch(`${apiBaseUrl()}/api/v1/assignments/${assignmentId}/submission`, {
    headers: { Authorization: `Bearer ${token}` },
    cache: "no-store",
  });
  if (res.status === 404) return null;
  if (!res.ok) throw new Error(`Failed to load submission: ${res.status}`);
  return res.json();
}

// Returns null when the caller doesn't own the file's submission and isn't
// authorized to view it (the backend answers 404, never 403, for that case).
export async function getSubmissionFileDownloadUrl(token: string, fileId: string): Promise<{ url: string; filename: string } | null> {
  const res = await fetch(`${apiBaseUrl()}/api/v1/submission-files/${fileId}/download`, {
    headers: { Authorization: `Bearer ${token}` },
    cache: "no-store",
  });
  if (res.status === 404) return null;
  if (!res.ok) throw new Error(`Failed to authorize download: ${res.status}`);
  return res.json();
}

// --- Coding exercises / secure code runner (Stage 16) ---

export type CodeLanguage = "go" | "python" | "javascript";

export type CodeSubmissionStatus =
  | "queued"
  | "running"
  | "passed"
  | "failed"
  | "compile_error"
  | "runtime_error"
  | "timeout"
  | "internal_error";

// TERMINAL_SUBMISSION_STATUSES mirrors backend/internal/coding/model.go's
// TerminalStatuses — the frontend poll loop stops once a submission reaches
// one of these, exactly like VideoProcessingStatusPanel's isInFlight check.
export const TERMINAL_SUBMISSION_STATUSES: ReadonlySet<CodeSubmissionStatus> = new Set([
  "passed",
  "failed",
  "compile_error",
  "runtime_error",
  "timeout",
  "internal_error",
]);

export interface StudentCodingExercise {
  id: string;
  lesson_id: string;
  title: string;
  description: string;
  language: CodeLanguage;
  starter_code: string;
  required: boolean;
  time_limit_ms: number;
  memory_limit_mb: number;
}

export interface StudentTestCaseExample {
  id: string;
  input?: string;
  expected_output: string;
  position: number;
}

export interface StudentCodingExerciseView {
  exercise: StudentCodingExercise;
  examples: StudentTestCaseExample[];
}

export interface CodeSubmission {
  id: string;
  exercise_id: string;
  user_id: string;
  language: CodeLanguage;
  source_code: string;
  mode: "run" | "submit";
  status: CodeSubmissionStatus;
  passed_tests: number;
  total_tests: number;
  execution_time_ms?: number;
  memory_used_kb?: number;
  stdout?: string;
  compile_output?: string;
  created_at: string;
  finished_at?: string;
}

// Returns null when this lesson has no (published) coding exercise.
export async function getLessonCodingExercise(token: string, lessonId: string): Promise<StudentCodingExerciseView | null> {
  const res = await fetch(`${apiBaseUrl()}/api/v1/lessons/${lessonId}/coding-exercise`, {
    headers: { Authorization: `Bearer ${token}` },
    cache: "no-store",
  });
  if (res.status === 404 || res.status === 403) return null;
  if (!res.ok) throw new Error(`Failed to load coding exercise: ${res.status}`);
  return res.json();
}

// Returns null when the attempt doesn't exist or doesn't belong to this user.
export async function getMyAttemptDetail(token: string, attemptId: string): Promise<AttemptDetail | null> {
  const res = await fetch(`${apiBaseUrl()}/api/v1/me/test-attempts/${attemptId}`, {
    headers: { Authorization: `Bearer ${token}` },
    cache: "no-store",
  });

  if (res.status === 403 || res.status === 404) {
    return null;
  }

  if (!res.ok) {
    throw new Error(`Failed to load attempt: ${res.status}`);
  }

  return res.json();
}

export async function getMyRoadmap(token: string, specialityId: string): Promise<MyRoadmap | null> {
  const res = await fetch(`${apiBaseUrl()}/api/v1/me/specialities/${specialityId}`, {
    headers: { Authorization: `Bearer ${token}` },
    cache: "no-store",
  });

  if (res.status === 404) {
    return null;
  }

  if (!res.ok) {
    throw new Error(`Failed to load roadmap: ${res.status}`);
  }

  return res.json();
}

export interface CertificateSummary {
  id: string;
  certificate_number: string;
  course_id: string;
  course_title: string;
  issued_at: string;
}

export interface CertificateDetail {
  id: string;
  certificate_number: string;
  course_id: string;
  course_title: string;
  student_name: string;
  issued_at: string;
}

export interface PublicVerification {
  valid: boolean;
  certificate_number?: string;
  student_name?: string;
  course_title?: string;
  issued_at?: string;
}

// Calling this also lazily issues certificates for any course the user has
// completed but doesn't have one for yet — see the backend service docs.
export async function getMyCertificates(token: string): Promise<CertificateSummary[]> {
  const res = await fetch(`${apiBaseUrl()}/api/v1/me/certificates`, {
    headers: { Authorization: `Bearer ${token}` },
    cache: "no-store",
  });

  if (!res.ok) {
    throw new Error(`Failed to load certificates: ${res.status}`);
  }

  return res.json();
}

// Returns null when the certificate doesn't exist or doesn't belong to this user.
export async function getMyCertificateDetail(token: string, certificateId: string): Promise<CertificateDetail | null> {
  const res = await fetch(`${apiBaseUrl()}/api/v1/me/certificates/${certificateId}`, {
    headers: { Authorization: `Bearer ${token}` },
    cache: "no-store",
  });

  if (res.status === 403 || res.status === 404) {
    return null;
  }

  if (!res.ok) {
    throw new Error(`Failed to load certificate: ${res.status}`);
  }

  return res.json();
}

// Plan.price_amount is in minor currency units (1/100 of the major unit) —
// e.g. 990000 == 9900.00 in a 2-decimal-digit currency such as KZT. Never
// treat this as the literal display amount; always divide by 100 first.
export interface Plan {
  id: string;
  name: string;
  slug: string;
  description: string;
  price_amount: number;
  currency: string;
  duration_days: number;
  active: boolean;
  created_at: string;
  updated_at: string;
}

export function formatPrice(plan: Pick<Plan, "price_amount" | "currency">): string {
  return `${(plan.price_amount / 100).toLocaleString("ru-RU", { minimumFractionDigits: 2 })} ${plan.currency}`;
}

export interface Subscription {
  id: string;
  user_id: string;
  plan_id: string;
  status: string;
  starts_at: string | null;
  expires_at: string | null;
  canceled_at: string | null;
  created_at: string;
  updated_at: string;
}

export interface Payment {
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
}

export interface CreateSubscriptionResult {
  subscription: Subscription;
  payment: Payment;
  plan: Plan;
}

export interface MySubscription {
  active: boolean;
  plan?: Plan;
  status?: string;
  starts_at?: string;
  expires_at?: string;
}

export async function getPlans(): Promise<Plan[]> {
  const res = await fetch(`${apiBaseUrl()}/api/v1/plans`, { cache: "no-store" });
  if (!res.ok) {
    throw new Error(`Failed to load plans: ${res.status}`);
  }
  return res.json();
}

export async function getPlan(id: string): Promise<Plan | null> {
  const plans = await getPlans();
  return plans.find((p) => p.id === id) ?? null;
}

export async function getMySubscription(token: string): Promise<MySubscription> {
  const res = await fetch(`${apiBaseUrl()}/api/v1/me/subscription`, {
    headers: { Authorization: `Bearer ${token}` },
    cache: "no-store",
  });
  if (!res.ok) {
    throw new Error(`Failed to load subscription: ${res.status}`);
  }
  return res.json();
}

export interface AppNotification {
  id: string;
  user_id: string;
  type: string;
  title: string;
  message: string;
  data?: Record<string, unknown>;
  read_at?: string;
  created_at: string;
}

export interface PageResult<T> {
  items: T[];
  page: number;
  limit: number;
  total: number;
  total_pages: number;
}

export async function getMyNotifications(token: string, page = 1, limit = 20): Promise<PageResult<AppNotification>> {
  const res = await fetch(`${apiBaseUrl()}/api/v1/me/notifications?page=${page}&limit=${limit}`, {
    headers: { Authorization: `Bearer ${token}` },
    cache: "no-store",
  });
  if (!res.ok) {
    throw new Error(`Failed to load notifications: ${res.status}`);
  }
  return res.json();
}

export async function getUnreadNotificationCount(token: string): Promise<number> {
  const res = await fetch(`${apiBaseUrl()}/api/v1/me/notifications/unread-count`, {
    headers: { Authorization: `Bearer ${token}` },
    cache: "no-store",
  });
  if (!res.ok) {
    return 0;
  }
  const data = await res.json();
  return data.count ?? 0;
}

// notificationActionLink derives an internal, allow-listed destination from
// a notification's type + data — never from a raw URL in the payload (the
// backend never sends one, and this function wouldn't trust it if it did).
// Unknown types simply get no action link.
export function notificationActionLink(n: AppNotification): string | null {
  const data = n.data ?? {};
  switch (n.type) {
    case "enrolled":
    case "course_completed":
      return "/dashboard/courses";
    case "certificate_issued":
      return typeof data.certificate_id === "string" ? `/dashboard/certificates/${data.certificate_id}` : "/dashboard/certificates";
    case "subscription_activated":
    case "subscription_expiring":
    case "subscription_expired":
      return "/dashboard/subscription";
    case "course_announcement":
      return typeof data.course_id === "string" ? `/courses/${data.course_id}` : "/courses";
    case "question_answered":
      return typeof data.course_id === "string" && typeof data.lesson_id === "string"
        ? `/learn/${data.course_id}/${data.lesson_id}`
        : null;
    default:
      return null;
  }
}

export async function verifyCertificate(certificateNumber: string): Promise<PublicVerification> {
  const res = await fetch(`${apiBaseUrl()}/api/v1/certificates/verify/${encodeURIComponent(certificateNumber)}`, {
    cache: "no-store",
  });

  if (!res.ok) {
    throw new Error(`Failed to verify certificate: ${res.status}`);
  }

  return res.json();
}

// --- Learning analytics / achievements / streaks (Stage 17) ---

export interface AnalyticsStats {
  courses_enrolled: number;
  courses_completed: number;
  lessons_completed: number;
  assignments_approved: number;
  coding_exercises_passed: number;
  tests_passed: number;
  certificates: number;
  current_streak: number;
  longest_streak: number;
}

export interface ActivityDayCount {
  date: string;
  activity_count: number;
}

export interface RecentActivityEntry {
  id: string;
  activity_type:
    | "lesson_completed"
    | "assignment_submitted"
    | "assignment_approved"
    | "coding_exercise_passed"
    | "test_passed"
    | "course_completed"
    | "certificate_issued";
  entity_type?: string;
  entity_id?: string;
  occurred_at: string;
  metadata?: Record<string, unknown>;
}

export async function getMyAnalytics(token: string): Promise<AnalyticsStats> {
  const res = await fetch(`${apiBaseUrl()}/api/v1/me/analytics`, {
    headers: { Authorization: `Bearer ${token}` },
    cache: "no-store",
  });
  if (!res.ok) throw new Error(`Failed to load analytics: ${res.status}`);
  return res.json();
}

export async function getMyActivityCalendar(token: string, from?: string, to?: string): Promise<ActivityDayCount[]> {
  const qs = new URLSearchParams();
  if (from) qs.set("from", from);
  if (to) qs.set("to", to);
  const res = await fetch(`${apiBaseUrl()}/api/v1/me/activity?${qs}`, {
    headers: { Authorization: `Bearer ${token}` },
    cache: "no-store",
  });
  if (!res.ok) throw new Error(`Failed to load activity calendar: ${res.status}`);
  return res.json();
}

export async function getMyRecentActivity(token: string): Promise<RecentActivityEntry[]> {
  const res = await fetch(`${apiBaseUrl()}/api/v1/me/activity/recent`, {
    headers: { Authorization: `Bearer ${token}` },
    cache: "no-store",
  });
  if (!res.ok) throw new Error(`Failed to load recent activity: ${res.status}`);
  return res.json();
}

export interface EarnedAchievement {
  code: string;
  title: string;
  description: string;
  icon: string;
  earned_at: string;
}

export interface LockedAchievement {
  code: string;
  title: string;
  description: string;
  icon: string;
}

export interface AchievementsView {
  earned: EarnedAchievement[];
  locked: LockedAchievement[];
}

export async function getMyAchievements(token: string): Promise<AchievementsView> {
  const res = await fetch(`${apiBaseUrl()}/api/v1/me/achievements`, {
    headers: { Authorization: `Bearer ${token}` },
    cache: "no-store",
  });
  if (!res.ok) throw new Error(`Failed to load achievements: ${res.status}`);
  return res.json();
}

// activityDisplayText builds a safe, structured-data-driven display string
// (item 17: "Не хранить presentation text как единственный источник
// истины. Хранить structured event + формировать display безопасно") —
// entity titles are never embedded server-side into the activity row, so
// there is nothing here to escape/inject; this is plain string formatting
// over known enum values and a JSON metadata object the backend itself
// controls the shape of.
export function activityDisplayText(entry: RecentActivityEntry): string {
  const title = typeof entry.metadata?.title === "string" ? entry.metadata.title : undefined;
  switch (entry.activity_type) {
    case "lesson_completed":
      return title ? `Завершён урок «${title}»` : "Завершён урок";
    case "assignment_submitted":
      return "Отправлено домашнее задание";
    case "assignment_approved":
      return "Домашнее задание принято";
    case "coding_exercise_passed":
      return title ? `Пройдено упражнение по коду «${title}»` : "Пройдено упражнение по коду";
    case "test_passed":
      return title ? `Пройден тест «${title}»` : "Пройден тест";
    case "course_completed":
      return title ? `Завершён курс «${title}»` : "Завершён курс";
    case "certificate_issued":
      return title ? `Получен сертификат «${title}»` : "Получен сертификат";
    default:
      return entry.activity_type;
  }
}

// --- Wishlist / Continue Learning / Recommendations (Stage 18) ---

export interface WishlistItem {
  course_id: string;
  title: string;
  slug: string;
  image_url: string;
  access_type: "free" | "subscription";
  category_name?: string;
  rating_average: number;
  rating_count: number;
  added_at: string;
}

// Returns [] rather than throwing on a cold/anonymous call site — every
// caller of this already guards on `token` being present, so a non-OK
// response here means something is actually wrong, not "not logged in".
export async function getMyWishlist(token: string): Promise<WishlistItem[]> {
  const res = await fetch(`${apiBaseUrl()}/api/v1/me/wishlist`, {
    headers: { Authorization: `Bearer ${token}` },
    cache: "no-store",
  });
  if (!res.ok) throw new Error(`Failed to load wishlist: ${res.status}`);
  return res.json();
}

// Backs the "user-aware enrichment" pattern (Stage 18 item 3) — the public
// course listing/detail endpoints are never touched; this is a second,
// lightweight authenticated call the page makes only when a session
// exists, to know which course ids to mark in_wishlist.
export async function getMyWishlistCourseIds(token: string): Promise<string[]> {
  const res = await fetch(`${apiBaseUrl()}/api/v1/me/wishlist/course-ids`, {
    headers: { Authorization: `Bearer ${token}` },
    cache: "no-store",
  });
  if (!res.ok) throw new Error(`Failed to load wishlist ids: ${res.status}`);
  return res.json();
}

export interface ContinueLearningItem {
  course_id: string;
  title: string;
  image_url: string;
  progress_percent: number;
  next_lesson_id?: string;
  next_lesson_title?: string;
  last_activity_at?: string;
}

export async function getContinueLearning(token: string): Promise<ContinueLearningItem[]> {
  const res = await fetch(`${apiBaseUrl()}/api/v1/me/continue-learning`, {
    headers: { Authorization: `Bearer ${token}` },
    cache: "no-store",
  });
  if (!res.ok) throw new Error(`Failed to load continue-learning list: ${res.status}`);
  return res.json();
}

// Reason codes are the backend's exact vocabulary (never pre-rendered text
// — see internal/recommendations' model.go). ReasonLabel below is the only
// place they're turned into Russian display strings.
export type RecommendationReason =
  | "same_category"
  | "in_learning_path"
  | "in_wishlist"
  | "high_rating"
  | "popular"
  | "new_course"
  | "speciality_overlap";

export interface Recommendation {
  course_id: string;
  title: string;
  slug: string;
  image_url: string;
  access_type: "free" | "subscription";
  category_name?: string;
  rating_average: number;
  rating_count: number;
  score: number;
  reasons: RecommendationReason[];
}

export async function getMyRecommendations(token: string): Promise<Recommendation[]> {
  const res = await fetch(`${apiBaseUrl()}/api/v1/me/recommendations`, {
    headers: { Authorization: `Bearer ${token}` },
    cache: "no-store",
  });
  if (!res.ok) throw new Error(`Failed to load recommendations: ${res.status}`);
  return res.json();
}

// Public — no auth header, no personalization (item 18).
export async function getSimilarCourses(courseId: string): Promise<Recommendation[]> {
  const res = await fetch(`${apiBaseUrl()}/api/v1/courses/${courseId}/similar`, { cache: "no-store" });
  if (!res.ok) throw new Error(`Failed to load similar courses: ${res.status}`);
  return res.json();
}

const REASON_LABELS: Record<RecommendationReason, string> = {
  same_category: "Похоже на то, что вы изучаете",
  in_learning_path: "Продолжение вашей специальности",
  in_wishlist: "Вы добавили курс в избранное",
  high_rating: "Высокая оценка студентов",
  popular: "Популярный курс",
  new_course: "Новый курс",
  speciality_overlap: "Похожая специальность",
};

// reasonLabel never exposes the numeric score/weights that produced a
// recommendation (item 11) — only this fixed, backend-independent mapping
// from a reason code to a short Russian phrase. categoryName, when given,
// personalizes the same_category phrasing (e.g. "Похоже на то, что вы
// изучаете Go") without the backend ever sending pre-rendered text.
export function reasonLabel(reason: RecommendationReason, categoryName?: string): string {
  if (reason === "same_category" && categoryName) {
    return `Похоже на то, что вы изучаете ${categoryName}`;
  }
  return REASON_LABELS[reason] ?? reason;
}

// --- Lesson Q&A (Stage 20A backend, Stage 20B1 frontend) -------------------
// Mirrors backend/internal/qa/model.go exactly. POST responses from the
// backend return the bare Question/Answer shape (no display_name/answers) —
// see lib/actions.ts's askQuestionAction/answerQuestionAction, which
// synthesize the missing display_name client-side from the already-known
// current user rather than refetching.

export interface QAAnswer {
  id: string;
  question_id: string;
  user_id: string;
  body: string;
  is_instructor_answer: boolean;
  published: boolean;
  created_at: string;
  updated_at: string;
}

export interface QAAnswerView extends QAAnswer {
  display_name: string;
}

export interface QAQuestion {
  id: string;
  lesson_id: string;
  course_id: string;
  user_id: string;
  body: string;
  published: boolean;
  created_at: string;
  updated_at: string;
}

export interface QAQuestionView extends QAQuestion {
  display_name: string;
  answers: QAAnswerView[];
}

export async function getLessonQuestions(
  token: string,
  lessonId: string,
  page = 1,
  limit = 20,
): Promise<PageResult<QAQuestionView>> {
  const res = await fetch(`${apiBaseUrl()}/api/v1/lessons/${lessonId}/questions?page=${page}&limit=${limit}`, {
    headers: { Authorization: `Bearer ${token}` },
    cache: "no-store",
  });
  if (!res.ok) {
    throw new Error(`Failed to load questions: ${res.status}`);
  }
  return res.json();
}

// getLessonQuestionsModeration is getLessonQuestions' moderation
// counterpart (Stage 21C): hits the instructor/admin-authorized endpoint
// that includes hidden (published: false) content, so the moderation pages
// can actually find and un-hide what they previously hid — the plain
// public endpoint above filters hidden content out, which made a hidden
// question disappear from the moderator's own view too (confirmed live
// this session). Calls the "/instructor" path for both instructor and
// admin callers, same reasoning as setQuestionPublishedAction/
// setAnswerPublishedAction in lib/actions.ts: that route group accepts
// both roles, and the backend re-derives real authorization per request.
export async function getLessonQuestionsModeration(
  token: string,
  lessonId: string,
  page = 1,
  limit = 20,
): Promise<PageResult<QAQuestionView>> {
  const res = await fetch(`${apiBaseUrl()}/api/v1/instructor/qa/lessons/${lessonId}/questions?page=${page}&limit=${limit}`, {
    headers: { Authorization: `Bearer ${token}` },
    cache: "no-store",
  });
  if (!res.ok) {
    throw new Error(`Failed to load questions: ${res.status}`);
  }
  return res.json();
}
