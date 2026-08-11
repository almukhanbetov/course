import Link from "next/link";
import type { Metadata } from "next";
import { redirect } from "next/navigation";
import { getSessionToken } from "@/lib/session";
import { getMyCourses, getMyCertificates } from "@/lib/api";

export const metadata: Metadata = {
  title: "Мои курсы — LMS Platform",
};

export default async function MyCoursesPage() {
  const token = await getSessionToken();
  if (!token) {
    redirect("/login");
  }

  let courses: Awaited<ReturnType<typeof getMyCourses>> = [];
  let error: string | null = null;

  try {
    courses = await getMyCourses(token);
  } catch (err) {
    error = err instanceof Error ? err.message : "Unknown error";
  }

  // Fetching this also lazily issues a certificate for any course that just
  // became completed but doesn't have one yet — see lib/api.ts.
  let certificateByCourseId = new Map<string, string>();
  try {
    const certificates = await getMyCertificates(token);
    certificateByCourseId = new Map(certificates.map((c) => [c.course_id, c.id]));
  } catch {
    // Non-fatal for this page — the course list itself still renders.
  }

  return (
    <main>
      <h1>Мои курсы</h1>

      {error && <p role="alert">Не удалось загрузить курсы: {error}</p>}

      {!error && courses.length === 0 && (
        <p>
          Вы пока не записаны ни на один курс. <Link href="/courses">Посмотреть каталог</Link>
        </p>
      )}

      <div className="course-grid">
        {courses.map((course) => {
          const lessonsDone = course.lessons_progress_percent === 100;
          const needsFinalTest = course.has_final_test && lessonsDone && !course.final_test_passed;
          const certificateId = certificateByCourseId.get(course.course_id);

          let continueHref = `/courses/${course.course_id}`;
          if (needsFinalTest && course.final_test_id) {
            continueHref = `/tests/${course.final_test_id}`;
          } else if (course.next_lesson_id) {
            continueHref = `/learn/${course.course_id}/${course.next_lesson_id}`;
          }

          return (
            <div key={course.course_id} className="my-course-card">
              <h2>{course.title}</h2>

              <p className="my-course-meta">
                Уроки: {course.lessons_progress_percent}% ({course.completed_lessons} / {course.total_lessons})
              </p>
              <div className="progress-bar">
                <div className="progress-bar-fill" style={{ width: `${course.lessons_progress_percent}%` }} />
              </div>

              {course.has_final_test && (
                <p className="my-course-meta">
                  Итоговый тест:{" "}
                  {course.final_test_passed
                    ? `${course.final_test_best_score}% — пройден`
                    : course.final_test_best_score !== undefined
                      ? `${course.final_test_best_score}% — не пройден`
                      : "не пройден"}
                </p>
              )}

              {course.completed && (
                <p className="my-course-meta">
                  <strong>Курс завершён</strong>
                </p>
              )}

              {course.completed && certificateId && (
                <p className="my-course-meta">
                  <Link href={`/dashboard/certificates/${certificateId}`}>Сертификат получен →</Link>
                </p>
              )}

              <Link href={continueHref} className="btn-primary">
                {needsFinalTest ? "Пройти итоговый тест" : "Продолжить обучение"}
              </Link>
            </div>
          );
        })}
      </div>
    </main>
  );
}
