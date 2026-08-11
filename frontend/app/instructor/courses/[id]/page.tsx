import Link from "next/link";
import { notFound, redirect } from "next/navigation";
import { getSessionToken } from "@/lib/session";
import { getCategories } from "@/lib/api";
import {
  instructorGetCourse,
  instructorGetLessonAssignment,
  instructorGetLessonCodingExercise,
  instructorGetLessonVideo,
  instructorListCourseTests,
  instructorListTestCases,
  instructorGetCourseStats,
  type InstructorAssignment,
  type InstructorCodingExercise,
  type InstructorTestCase,
  type InstructorTestSummary,
} from "@/lib/instructor-api";
import type { AdminLessonVideo } from "@/lib/admin-api";
import { InstructorCourseForm } from "@/components/instructor/InstructorCourseForm";
import { InstructorModuleForm } from "@/components/instructor/InstructorModuleForm";
import { InstructorLessonForm } from "@/components/instructor/InstructorLessonForm";
import { InstructorLessonVideoUpload } from "@/components/instructor/InstructorLessonVideoUpload";
import { InstructorAssignmentEditor } from "@/components/instructor/InstructorAssignmentEditor";
import { InstructorCodingExerciseEditor } from "@/components/instructor/InstructorCodingExerciseEditor";
import { PublicationPanel } from "@/components/instructor/PublicationPanel";
import { ConfirmButton } from "@/components/ConfirmButton";
import {
  deleteInstructorModuleAction,
  deleteInstructorLessonAction,
  moveInstructorModuleAction,
  moveInstructorLessonAction,
} from "@/lib/instructor-actions";

export default async function InstructorCourseDetailPage({
  params,
  searchParams,
}: {
  params: Promise<{ id: string }>;
  searchParams: Promise<{ error?: string }>;
}) {
  const { id } = await params;
  const { error } = await searchParams;

  const token = await getSessionToken();
  if (!token) {
    redirect("/login");
  }

  const course = await instructorGetCourse(token, id);
  if (!course) {
    notFound();
  }

  const [categories, tests, stats] = await Promise.all([
    getCategories(),
    instructorListCourseTests(token, id),
    instructorGetCourseStats(token, id),
  ]);

  const modules = [...course.modules].sort((a, b) => a.position - b.position);

  const allLessonIds = modules.flatMap((m) => m.lessons.map((l) => l.id));
  const videoEntries = await Promise.all(
    allLessonIds.map(async (lessonId) => [lessonId, await instructorGetLessonVideo(token, lessonId)] as const),
  );
  const videosByLessonId = new Map<string, AdminLessonVideo | null>(videoEntries);

  const assignmentEntries = await Promise.all(
    allLessonIds.map(async (lessonId) => [lessonId, await instructorGetLessonAssignment(token, lessonId)] as const),
  );
  const assignmentsByLessonId = new Map<string, InstructorAssignment | null>(assignmentEntries);

  const codingExerciseEntries = await Promise.all(
    allLessonIds.map(async (lessonId) => [lessonId, await instructorGetLessonCodingExercise(token, lessonId)] as const),
  );
  const codingExercisesByLessonId = new Map<string, InstructorCodingExercise | null>(codingExerciseEntries);
  const testCasesByExerciseId = new Map<string, InstructorTestCase[]>(
    await Promise.all(
      codingExerciseEntries
        .filter(([, exercise]) => exercise !== null)
        .map(async ([, exercise]) => [exercise!.id, await instructorListTestCases(token, exercise!.id)] as const),
    ),
  );

  return (
    <div>
      <Link href="/instructor/courses">← Мои курсы</Link>
      <h1>{course.title}</h1>

      {error && <p role="alert">{decodeURIComponent(error)}</p>}

      <div className="admin-card">
        <h2>Основная информация и категория</h2>
        <InstructorCourseForm course={course} categories={categories} />
      </div>

      <PublicationPanel course={course} />

      <div className="admin-card">
        <h2>Статистика курса</h2>
        <div className="stat-grid">
          <div className="stat-tile">
            <div className="value">{stats.enrollments}</div>
            <div className="label">Зачислений</div>
          </div>
          <div className="stat-tile">
            <div className="value">{stats.completion_rate_percent.toFixed(0)}%</div>
            <div className="label">Завершили курс</div>
          </div>
          <div className="stat-tile">
            <div className="value">{stats.average_lesson_progress_percent.toFixed(0)}%</div>
            <div className="label">Средний прогресс</div>
          </div>
          <div className="stat-tile">
            <div className="value">{stats.final_test_pass_rate_percent.toFixed(0)}%</div>
            <div className="label">Сдали финальный тест</div>
          </div>
          <div className="stat-tile">
            <div className="value">{stats.average_rating.toFixed(1)}</div>
            <div className="label">Средний рейтинг</div>
          </div>
          <div className="stat-tile">
            <div className="value">{stats.review_count}</div>
            <div className="label">Отзывов</div>
          </div>
          <div className="stat-tile">
            <div className="value">{stats.assignments_count}</div>
            <div className="label">Заданий</div>
          </div>
          <div className="stat-tile">
            <div className="value">{stats.submitted_count}</div>
            <div className="label">Отправлено решений</div>
          </div>
          <div className="stat-tile">
            <div className="value">{stats.awaiting_review_count}</div>
            <div className="label">Ожидают проверки</div>
          </div>
          <div className="stat-tile">
            <div className="value">{stats.approval_rate_percent.toFixed(0)}%</div>
            <div className="label">Доля принятых</div>
          </div>
          <div className="stat-tile">
            <div className="value">{stats.average_score.toFixed(0)}</div>
            <div className="label">Средний балл</div>
          </div>
          <div className="stat-tile">
            <div className="value">{stats.coding_exercises_count}</div>
            <div className="label">Упражнений по коду</div>
          </div>
          <div className="stat-tile">
            <div className="value">{stats.code_submissions_count}</div>
            <div className="label">Отправлено решений кода</div>
          </div>
          <div className="stat-tile">
            <div className="value">{stats.code_pass_rate_percent.toFixed(0)}%</div>
            <div className="label">Доля успешных решений</div>
          </div>
          <div className="stat-tile">
            <div className="value">{stats.code_average_attempts_before_pass.toFixed(1)}</div>
            <div className="label">Попыток до успеха (в среднем)</div>
          </div>
        </div>

        <h3 className="mt-3">Активность за последние 7 дней</h3>
        <div className="stat-grid">
          <div className="stat-tile">
            <div className="value">{stats.active_students_last_7_days}</div>
            <div className="label">Активных студентов</div>
          </div>
          <div className="stat-tile">
            <div className="value">{stats.lessons_completed_last_7_days}</div>
            <div className="label">Уроков завершено</div>
          </div>
          <div className="stat-tile">
            <div className="value">{stats.assignment_submissions_last_7_days}</div>
            <div className="label">Отправлено заданий</div>
          </div>
          <div className="stat-tile">
            <div className="value">{stats.coding_submissions_last_7_days}</div>
            <div className="label">Отправлено решений кода</div>
          </div>
          <div className="stat-tile">
            <div className="value">{stats.coding_pass_rate_last_7_days_percent.toFixed(0)}%</div>
            <div className="label">Доля успешных решений</div>
          </div>
        </div>
        <Link href={`/instructor/courses/${course.id}/students`} className="btn-secondary mt-3">
          Студенты курса
        </Link>
      </div>

      <h2>Модули и уроки</h2>
      {modules.length === 0 && <p>Модулей пока нет.</p>}

      {modules.map((module, moduleIdx) => {
        const lessons = [...module.lessons].sort((a, b) => a.position - b.position);

        return (
          <div key={module.id} className="admin-card">
            <div className="admin-inline-actions">
              <strong>
                {module.position}. {module.title}
              </strong>
              <form action={moveInstructorModuleAction.bind(null, course.id, module.id, "up")}>
                <button type="submit" className="btn-small" disabled={moduleIdx === 0}>
                  ↑
                </button>
              </form>
              <form action={moveInstructorModuleAction.bind(null, course.id, module.id, "down")}>
                <button type="submit" className="btn-small" disabled={moduleIdx === modules.length - 1}>
                  ↓
                </button>
              </form>
              <form action={deleteInstructorModuleAction.bind(null, course.id, module.id)}>
                <ConfirmButton className="btn-danger" confirmMessage="Удалить модуль и все его уроки?">
                  Удалить
                </ConfirmButton>
              </form>
            </div>

            <details>
              <summary>Переименовать модуль</summary>
              <InstructorModuleForm courseId={course.id} moduleId={module.id} title={module.title} />
            </details>

            <ul className="lesson-list">
              {lessons.map((lesson, lessonIdx) => (
                <li key={lesson.id}>
                  <details>
                    <summary>
                      {lesson.position}. {lesson.title}
                      {lesson.is_free && <span className="badge badge-free">бесплатно</span>}
                      {!lesson.published && <span className="badge">черновик</span>}
                    </summary>
                    <div className="admin-inline-actions my-2">
                      <form action={moveInstructorLessonAction.bind(null, course.id, module.id, lesson.id, "up")}>
                        <button type="submit" className="btn-small" disabled={lessonIdx === 0}>
                          ↑
                        </button>
                      </form>
                      <form action={moveInstructorLessonAction.bind(null, course.id, module.id, lesson.id, "down")}>
                        <button type="submit" className="btn-small" disabled={lessonIdx === lessons.length - 1}>
                          ↓
                        </button>
                      </form>
                      <form action={deleteInstructorLessonAction.bind(null, course.id, lesson.id)}>
                        <ConfirmButton className="btn-danger" confirmMessage="Удалить этот урок?">
                          Удалить
                        </ConfirmButton>
                      </form>
                    </div>
                    <InstructorLessonForm courseId={course.id} moduleId={module.id} lesson={lesson} />
                    <InstructorLessonVideoUpload
                      courseId={course.id}
                      lessonId={lesson.id}
                      video={videosByLessonId.get(lesson.id) ?? null}
                    />
                    <details>
                      <summary>Домашнее задание{assignmentsByLessonId.get(lesson.id) ? "" : " (нет)"}</summary>
                      <InstructorAssignmentEditor
                        courseId={course.id}
                        lessonId={lesson.id}
                        assignment={assignmentsByLessonId.get(lesson.id) ?? null}
                      />
                      {assignmentsByLessonId.get(lesson.id) && (
                        <Link
                          href={`/instructor/assignments/${assignmentsByLessonId.get(lesson.id)!.id}/submissions`}
                          className="btn-small"
                        >
                          Решения студентов
                        </Link>
                      )}
                    </details>
                    <details>
                      <summary>Практика кода{codingExercisesByLessonId.get(lesson.id) ? "" : " (нет)"}</summary>
                      <InstructorCodingExerciseEditor
                        courseId={course.id}
                        lessonId={lesson.id}
                        exercise={codingExercisesByLessonId.get(lesson.id) ?? null}
                        testCases={
                          codingExercisesByLessonId.get(lesson.id)
                            ? testCasesByExerciseId.get(codingExercisesByLessonId.get(lesson.id)!.id) ?? []
                            : []
                        }
                      />
                    </details>
                  </details>
                </li>
              ))}
            </ul>

            <details>
              <summary>Добавить урок в этот модуль</summary>
              <InstructorLessonForm courseId={course.id} moduleId={module.id} />
            </details>
          </div>
        );
      })}

      <div className="admin-card">
        <h3>Добавить модуль</h3>
        <InstructorModuleForm courseId={course.id} />
      </div>

      <h2>Тесты</h2>
      {tests.length === 0 && <p>Тестов пока нет.</p>}
      <ul className="lesson-list">
        {tests.map((test: InstructorTestSummary) => (
          <li key={test.id} className="admin-inline-actions">
            <span>
              {test.title} {test.is_final && <span className="badge">финальный</span>}
              {!test.published && <span className="badge">черновик</span>}
            </span>
            <Link href={`/instructor/courses/${course.id}/tests/${test.id}`} className="btn-small">
              Управлять
            </Link>
          </li>
        ))}
      </ul>
      <Link href={`/instructor/courses/${course.id}/tests/new`} className="btn-secondary mt-3">
        Новый тест
      </Link>
    </div>
  );
}
