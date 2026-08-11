import "server-only";
import { SERVER_API_URL, type Course, type CourseDetail, type PageResult, type Submission } from "@/lib/api";
import type { AdminLessonVideo, AdminTestDetail } from "@/lib/admin-api";

function authHeaders(token: string) {
  return { Authorization: `Bearer ${token}` };
}

export interface ApiError {
  error?: { code: string; message: string };
}

async function parseErrorMessage(res: Response): Promise<string> {
  const body: ApiError | null = await res.json().catch(() => null);
  return body?.error?.message ?? `HTTP ${res.status}`;
}

export async function instructorListCourses(
  token: string,
  params: { page?: number; limit?: number } = {},
): Promise<PageResult<Course>> {
  const qs = new URLSearchParams();
  if (params.page) qs.set("page", String(params.page));
  if (params.limit) qs.set("limit", String(params.limit));

  const res = await fetch(`${SERVER_API_URL}/api/v1/instructor/courses?${qs}`, {
    headers: authHeaders(token),
    cache: "no-store",
  });
  if (!res.ok) throw new Error(await parseErrorMessage(res));
  return res.json();
}

// Returns null when the course doesn't exist or isn't owned by this instructor (404/403).
export async function instructorGetCourse(token: string, id: string): Promise<CourseDetail | null> {
  const res = await fetch(`${SERVER_API_URL}/api/v1/instructor/courses/${id}`, {
    headers: authHeaders(token),
    cache: "no-store",
  });
  if (res.status === 404 || res.status === 403) return null;
  if (!res.ok) throw new Error(await parseErrorMessage(res));
  return res.json();
}

// Returns null when the lesson has no video yet — not an error state.
export async function instructorGetLessonVideo(token: string, lessonId: string): Promise<AdminLessonVideo | null> {
  const res = await fetch(`${SERVER_API_URL}/api/v1/instructor/lessons/${lessonId}/video`, {
    headers: authHeaders(token),
    cache: "no-store",
  });
  if (res.status === 404) return null;
  if (!res.ok) throw new Error(await parseErrorMessage(res));
  return res.json();
}

export interface InstructorTestSummary {
  id: string;
  course_id?: string;
  module_id?: string;
  lesson_id?: string;
  title: string;
  passing_score: number;
  published: boolean;
  is_final: boolean;
}

export async function instructorListCourseTests(token: string, courseId: string): Promise<InstructorTestSummary[]> {
  const res = await fetch(`${SERVER_API_URL}/api/v1/instructor/courses/${courseId}/tests`, {
    headers: authHeaders(token),
    cache: "no-store",
  });
  if (!res.ok) throw new Error(await parseErrorMessage(res));
  return res.json();
}

export async function instructorGetTest(token: string, testId: string): Promise<AdminTestDetail | null> {
  const res = await fetch(`${SERVER_API_URL}/api/v1/instructor/tests/${testId}`, {
    headers: authHeaders(token),
    cache: "no-store",
  });
  if (res.status === 404 || res.status === 403) return null;
  if (!res.ok) throw new Error(await parseErrorMessage(res));
  return res.json();
}

export interface InstructorStudentRow {
  user_id: string;
  display_name: string;
  course_id: string;
  course_title: string;
  enrolled_at: string;
  completed_at?: string;
  progress_percent: number;
  completed: boolean;
}

export async function instructorListStudents(
  token: string,
  params: { page?: number; limit?: number } = {},
): Promise<PageResult<InstructorStudentRow>> {
  const qs = new URLSearchParams();
  if (params.page) qs.set("page", String(params.page));
  if (params.limit) qs.set("limit", String(params.limit));

  const res = await fetch(`${SERVER_API_URL}/api/v1/instructor/students?${qs}`, {
    headers: authHeaders(token),
    cache: "no-store",
  });
  if (!res.ok) throw new Error(await parseErrorMessage(res));
  return res.json();
}

export interface InstructorCourseStudentRow {
  user_id: string;
  display_name: string;
  enrolled_at: string;
  completed_at?: string;
  completed_lessons: number;
  total_lessons: number;
  progress_percent: number;
  final_test_passed: boolean;
  completed: boolean;
}

export async function instructorListCourseStudents(
  token: string,
  courseId: string,
  params: { page?: number; limit?: number } = {},
): Promise<PageResult<InstructorCourseStudentRow>> {
  const qs = new URLSearchParams();
  if (params.page) qs.set("page", String(params.page));
  if (params.limit) qs.set("limit", String(params.limit));

  const res = await fetch(`${SERVER_API_URL}/api/v1/instructor/courses/${courseId}/students?${qs}`, {
    headers: authHeaders(token),
    cache: "no-store",
  });
  if (!res.ok) throw new Error(await parseErrorMessage(res));
  return res.json();
}

export interface InstructorStats {
  courses_count: number;
  published_courses: number;
  students_count: number;
  active_enrollments: number;
  completed_enrollments: number;
  average_completion_percent: number;
  certificates_issued: number;
  submissions_awaiting_review: number;
  submissions_needs_revision: number;
}

export async function instructorGetStats(token: string): Promise<InstructorStats> {
  const res = await fetch(`${SERVER_API_URL}/api/v1/instructor/stats`, {
    headers: authHeaders(token),
    cache: "no-store",
  });
  if (!res.ok) throw new Error(await parseErrorMessage(res));
  return res.json();
}

export interface InstructorCourseStats {
  enrollments: number;
  completion_rate_percent: number;
  average_lesson_progress_percent: number;
  final_test_pass_rate_percent: number;
  average_rating: number;
  review_count: number;
  assignments_count: number;
  submitted_count: number;
  awaiting_review_count: number;
  approval_rate_percent: number;
  average_score: number;

  coding_exercises_count: number;
  code_submissions_count: number;
  code_pass_rate_percent: number;
  code_average_attempts_before_pass: number;

  active_students_last_7_days: number;
  lessons_completed_last_7_days: number;
  assignment_submissions_last_7_days: number;
  coding_submissions_last_7_days: number;
  coding_pass_rate_last_7_days_percent: number;
}

export async function instructorGetCourseStats(token: string, courseId: string): Promise<InstructorCourseStats> {
  const res = await fetch(`${SERVER_API_URL}/api/v1/instructor/courses/${courseId}/stats`, {
    headers: authHeaders(token),
    cache: "no-store",
  });
  if (!res.ok) throw new Error(await parseErrorMessage(res));
  return res.json();
}

export interface InstructorCourseReview {
  id: string;
  display_name: string;
  rating: number;
  review_text?: string;
  created_at: string;
}

export async function instructorListCourseReviews(
  token: string,
  courseId: string,
  params: { page?: number; limit?: number } = {},
): Promise<PageResult<InstructorCourseReview>> {
  const qs = new URLSearchParams();
  if (params.page) qs.set("page", String(params.page));
  if (params.limit) qs.set("limit", String(params.limit));

  const res = await fetch(`${SERVER_API_URL}/api/v1/instructor/courses/${courseId}/reviews?${qs}`, {
    headers: authHeaders(token),
    cache: "no-store",
  });
  if (!res.ok) throw new Error(await parseErrorMessage(res));
  return res.json();
}

// --- Assignments / homework (Stage 15) ---

export interface InstructorAssignment {
  id: string;
  lesson_id: string;
  title: string;
  description: string;
  instructions: string;
  required: boolean;
  max_score?: number;
  published: boolean;
  created_at: string;
  updated_at: string;
}

// Returns null when this lesson has no assignment yet.
export async function instructorGetLessonAssignment(token: string, lessonId: string): Promise<InstructorAssignment | null> {
  const res = await fetch(`${SERVER_API_URL}/api/v1/instructor/lessons/${lessonId}/assignment`, {
    headers: authHeaders(token),
    cache: "no-store",
  });
  if (res.status === 404) return null;
  if (!res.ok) throw new Error(await parseErrorMessage(res));
  return res.json();
}

export interface InstructorSubmissionRow {
  submission_id: string;
  assignment_id: string;
  assignment_title: string;
  lesson_id: string;
  lesson_title: string;
  course_id: string;
  course_title: string;
  student_id: string;
  student_name: string;
  status: "submitted" | "needs_revision" | "approved";
  score?: number;
  submitted_at?: string;
}

export async function instructorListAssignmentSubmissions(
  token: string,
  assignmentId: string,
  params: { status?: string; page?: number; limit?: number } = {},
): Promise<PageResult<InstructorSubmissionRow>> {
  const qs = new URLSearchParams();
  if (params.status) qs.set("status", params.status);
  if (params.page) qs.set("page", String(params.page));
  if (params.limit) qs.set("limit", String(params.limit));

  const res = await fetch(`${SERVER_API_URL}/api/v1/instructor/assignments/${assignmentId}/submissions?${qs}`, {
    headers: authHeaders(token),
    cache: "no-store",
  });
  if (!res.ok) throw new Error(await parseErrorMessage(res));
  return res.json();
}

export async function instructorListSubmissions(
  token: string,
  params: { course_id?: string; status?: string; page?: number; limit?: number } = {},
): Promise<PageResult<InstructorSubmissionRow>> {
  const qs = new URLSearchParams();
  if (params.course_id) qs.set("course_id", params.course_id);
  if (params.status) qs.set("status", params.status);
  if (params.page) qs.set("page", String(params.page));
  if (params.limit) qs.set("limit", String(params.limit));

  const res = await fetch(`${SERVER_API_URL}/api/v1/instructor/submissions?${qs}`, {
    headers: authHeaders(token),
    cache: "no-store",
  });
  if (!res.ok) throw new Error(await parseErrorMessage(res));
  return res.json();
}

export interface SubmissionReview {
  id: string;
  submission_id: string;
  reviewer_id: string;
  status: "approved" | "needs_revision";
  score?: number;
  feedback: string;
  created_at: string;
}

export interface SubmissionDetail extends Submission {
  assignment_title: string;
  max_score?: number;
  lesson_id: string;
  lesson_title: string;
  course_id: string;
  course_title: string;
  student_name: string;
  reviews: SubmissionReview[];
}

// Returns null when the submission doesn't exist or isn't in a course this
// instructor manages (the backend answers 404 for both — see ownership).
export async function instructorGetSubmissionDetail(token: string, id: string): Promise<SubmissionDetail | null> {
  const res = await fetch(`${SERVER_API_URL}/api/v1/instructor/submissions/${id}`, {
    headers: authHeaders(token),
    cache: "no-store",
  });
  if (res.status === 404) return null;
  if (!res.ok) throw new Error(await parseErrorMessage(res));
  return res.json();
}

// --- Coding exercises / secure code runner (Stage 16) ---

export interface InstructorCodingExercise {
  id: string;
  lesson_id: string;
  title: string;
  description: string;
  language: "go" | "python" | "javascript";
  starter_code: string;
  solution_code?: string;
  time_limit_ms: number;
  memory_limit_mb: number;
  published: boolean;
  required: boolean;
  created_at: string;
  updated_at: string;
}

export interface InstructorTestCase {
  id: string;
  coding_exercise_id: string;
  input?: string;
  expected_output: string;
  position: number;
  hidden: boolean;
  created_at: string;
}

// Returns null when this lesson has no coding exercise yet.
export async function instructorGetLessonCodingExercise(token: string, lessonId: string): Promise<InstructorCodingExercise | null> {
  const res = await fetch(`${SERVER_API_URL}/api/v1/instructor/lessons/${lessonId}/coding-exercise`, {
    headers: authHeaders(token),
    cache: "no-store",
  });
  if (res.status === 404) return null;
  if (!res.ok) throw new Error(await parseErrorMessage(res));
  return res.json();
}

export async function instructorListTestCases(token: string, exerciseId: string): Promise<InstructorTestCase[]> {
  const res = await fetch(`${SERVER_API_URL}/api/v1/instructor/coding-exercises/${exerciseId}/test-cases`, {
    headers: authHeaders(token),
    cache: "no-store",
  });
  if (!res.ok) throw new Error(await parseErrorMessage(res));
  return res.json();
}
