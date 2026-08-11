import Link from "next/link";
import { notFound, redirect } from "next/navigation";
import { getLessonAssignment, getLessonCodingExercise, getMyCourseDetail, getMySubmission } from "@/lib/api";
import { getSessionToken } from "@/lib/session";
import { completeLessonAction, listCodeAttemptsAction } from "@/lib/actions";
import { LessonVideoPlayer } from "@/components/LessonVideoPlayer";
import { AssignmentSection } from "@/components/AssignmentSection";
import { CodingExerciseSection } from "@/components/CodingExerciseSection";

export default async function LessonPage({
  params,
}: {
  params: Promise<{ courseId: string; lessonId: string }>;
}) {
  const { courseId, lessonId } = await params;

  const token = await getSessionToken();
  if (!token) {
    redirect("/login");
  }

  const course = await getMyCourseDetail(token, courseId);
  if (!course) {
    return (
      <main>
        <p role="alert">Вы не записаны на этот курс.</p>
        <Link href={`/courses/${courseId}`}>Перейти к курсу</Link>
      </main>
    );
  }

  const flatLessons = course.modules.flatMap((module) =>
    module.lessons.map((lesson) => ({ module, lesson })),
  );
  const currentIndex = flatLessons.findIndex((entry) => entry.lesson.id === lessonId);

  if (currentIndex === -1) {
    notFound();
  }

  const { module: currentModule, lesson: currentLesson } = flatLessons[currentIndex];
  const prevEntry = currentIndex > 0 ? flatLessons[currentIndex - 1] : null;
  const nextEntry = currentIndex < flatLessons.length - 1 ? flatLessons[currentIndex + 1] : null;

  const assignment = await getLessonAssignment(token, lessonId);
  const submission = assignment ? await getMySubmission(token, assignment.id) : null;

  const codingExerciseView = await getLessonCodingExercise(token, lessonId);
  const codingAttempts = codingExerciseView
    ? (await listCodeAttemptsAction(codingExerciseView.exercise.id)).attempts ?? []
    : [];

  return (
    <main>
      <Link href="/dashboard/courses">← Мои курсы</Link>
      <p className="subtitle">{course.title}</p>
      <h1>{currentLesson.title}</h1>
      <p className="my-course-meta">Модуль: {currentModule.title}</p>

      <div className="lesson-layout">
        <div>
          {currentLesson.video_status === "ready" ? (
            <LessonVideoPlayer lessonId={lessonId} initialProgressSeconds={currentLesson.progress_seconds} />
          ) : (
            <div className="video-placeholder">Видео для этого урока ещё не загружено.</div>
          )}

          {currentLesson.description && <p>{currentLesson.description}</p>}

          <div className="progress-bar">
            <div className="progress-bar-fill" style={{ width: `${course.progress_percent}%` }} />
          </div>
          <p className="my-course-meta">
            Прогресс курса: {course.completed_lessons} / {course.total_lessons} уроков ({course.progress_percent}%)
          </p>

          <div className="lesson-actions">
            {currentLesson.video_completed ? (
              <span className="badge badge-free">
                {currentLesson.completed ? "Урок завершён ✓" : "Видео просмотрено ✓"}
              </span>
            ) : (
              <form action={completeLessonAction.bind(null, courseId, lessonId, currentLesson.duration_seconds)}>
                <button type="submit" className="btn-primary">
                  Завершить урок
                </button>
              </form>
            )}
            {prevEntry && (
              <Link href={`/learn/${courseId}/${prevEntry.lesson.id}`} className="btn-secondary">
                ← Предыдущий
              </Link>
            )}
            {nextEntry && (
              <Link href={`/learn/${courseId}/${nextEntry.lesson.id}`} className="btn-secondary">
                Следующий →
              </Link>
            )}
          </div>

          {assignment && (
            <AssignmentSection courseId={courseId} lessonId={lessonId} assignment={assignment} submission={submission} />
          )}

          {codingExerciseView && (
            <CodingExerciseSection
              exercise={codingExerciseView.exercise}
              examples={codingExerciseView.examples}
              initialAttempts={codingAttempts}
            />
          )}
        </div>

        <aside className="lesson-sidebar">
          {course.modules.map((module) => (
            <div key={module.id}>
              <p className="module-title">{module.title}</p>
              <ul className="lesson-nav-list">
                {module.lessons.map((lesson) => (
                  <li key={lesson.id}>
                    <Link
                      href={`/learn/${courseId}/${lesson.id}`}
                      className={`lesson-nav-item${lesson.id === lessonId ? " active" : ""}`}
                    >
                      {lesson.completed && <span className="check">✓</span>}
                      {lesson.title}
                    </Link>
                  </li>
                ))}
              </ul>
            </div>
          ))}

          {course.has_final_test && course.final_test_id && (
            <div>
              <p className="module-title">Итоговый тест</p>
              <ul className="lesson-nav-list">
                <li>
                  {course.lessons_progress_percent === 100 ? (
                    <Link href={`/tests/${course.final_test_id}`} className="lesson-nav-item">
                      {course.final_test_passed && <span className="check">✓</span>}
                      Итоговый тест
                    </Link>
                  ) : (
                    <span className="lesson-nav-item locked">🔒 Итоговый тест (завершите уроки)</span>
                  )}
                </li>
              </ul>
            </div>
          )}
        </aside>
      </div>
    </main>
  );
}
