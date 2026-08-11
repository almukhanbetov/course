import Link from "next/link";
import { notFound, redirect } from "next/navigation";
import { getSessionToken } from "@/lib/session";
import {
  adminGetCourse,
  adminGetLessonVideo,
  adminListCategories,
  adminListInstructors,
  type AdminLessonVideo,
} from "@/lib/admin-api";
import { CourseForm } from "@/components/admin/CourseForm";
import { ModuleForm } from "@/components/admin/ModuleForm";
import { LessonForm } from "@/components/admin/LessonForm";
import { LessonVideoUpload } from "@/components/admin/LessonVideoUpload";
import { ConfirmButton } from "@/components/ConfirmButton";
import {
  deleteCourseAction,
  deleteModuleAction,
  deleteLessonAction,
  moveModuleAction,
  moveLessonAction,
  announceCourseAction,
} from "@/lib/admin-actions";

export default async function AdminCourseDetailPage({
  params,
  searchParams,
}: {
  params: Promise<{ id: string }>;
  searchParams: Promise<{ error?: string; announced?: string }>;
}) {
  const { id } = await params;
  const { error, announced } = await searchParams;

  const token = await getSessionToken();
  if (!token) {
    redirect("/login");
  }

  const course = await adminGetCourse(token, id);
  if (!course) {
    notFound();
  }

  const [categories, instructors] = await Promise.all([adminListCategories(token), adminListInstructors(token)]);
  const modules = [...course.modules].sort((a, b) => a.position - b.position);

  const allLessonIds = modules.flatMap((m) => m.lessons.map((l) => l.id));
  const videoEntries = await Promise.all(
    allLessonIds.map(async (lessonId) => [lessonId, await adminGetLessonVideo(token, lessonId)] as const),
  );
  const videosByLessonId = new Map<string, AdminLessonVideo | null>(videoEntries);

  return (
    <div>
      <Link href="/admin/courses">← All courses</Link>
      <h1>{course.title}</h1>

      {error && <p role="alert">{decodeURIComponent(error)}</p>}
      {announced && <p role="status">Announcement sent to {announced} student(s).</p>}

      <div className="admin-card">
        <h2>Course details</h2>
        <CourseForm course={course} categories={categories} instructors={instructors} />
        <div className="admin-inline-actions mt-3">
          <form action={deleteCourseAction.bind(null, course.id)}>
            <ConfirmButton
              className="btn-danger"
              confirmMessage="Delete this course? This is only possible if it has no student enrollments."
            >
              Delete course
            </ConfirmButton>
          </form>
          {course.published && (
            <form action={announceCourseAction.bind(null, course.id)}>
              <ConfirmButton
                className="btn-secondary"
                confirmMessage="Notify all active students about this course?"
              >
                Announce to students
              </ConfirmButton>
            </form>
          )}
        </div>
      </div>

      <h2>Modules</h2>
      {modules.length === 0 && <p>No modules yet.</p>}

      {modules.map((module, moduleIdx) => {
        const lessons = [...module.lessons].sort((a, b) => a.position - b.position);

        return (
          <div key={module.id} className="admin-card">
            <div className="admin-inline-actions">
              <strong>
                {module.position}. {module.title}
              </strong>
              <form action={moveModuleAction.bind(null, course.id, module.id, "up")}>
                <button type="submit" className="btn-small" disabled={moduleIdx === 0}>
                  ↑
                </button>
              </form>
              <form action={moveModuleAction.bind(null, course.id, module.id, "down")}>
                <button type="submit" className="btn-small" disabled={moduleIdx === modules.length - 1}>
                  ↓
                </button>
              </form>
              <form action={deleteModuleAction.bind(null, course.id, module.id)}>
                <ConfirmButton className="btn-danger" confirmMessage="Delete this module and all its lessons?">
                  Delete
                </ConfirmButton>
              </form>
            </div>

            <details>
              <summary>Rename module</summary>
              <ModuleForm courseId={course.id} moduleId={module.id} title={module.title} />
            </details>

            <ul className="lesson-list">
              {lessons.map((lesson, lessonIdx) => (
                <li key={lesson.id}>
                  <details>
                    <summary>
                      {lesson.position}. {lesson.title}
                      {lesson.is_free && <span className="badge badge-free">free</span>}
                      {!lesson.published && <span className="badge">draft</span>}
                    </summary>
                    <div className="admin-inline-actions my-2">
                      <form action={moveLessonAction.bind(null, course.id, module.id, lesson.id, "up")}>
                        <button type="submit" className="btn-small" disabled={lessonIdx === 0}>
                          ↑
                        </button>
                      </form>
                      <form action={moveLessonAction.bind(null, course.id, module.id, lesson.id, "down")}>
                        <button type="submit" className="btn-small" disabled={lessonIdx === lessons.length - 1}>
                          ↓
                        </button>
                      </form>
                      <form action={deleteLessonAction.bind(null, course.id, lesson.id)}>
                        <ConfirmButton className="btn-danger" confirmMessage="Delete this lesson?">
                          Delete
                        </ConfirmButton>
                      </form>
                    </div>
                    <LessonForm courseId={course.id} moduleId={module.id} lesson={lesson} />
                    <LessonVideoUpload
                      courseId={course.id}
                      lessonId={lesson.id}
                      video={videosByLessonId.get(lesson.id) ?? null}
                    />
                  </details>
                </li>
              ))}
            </ul>

            <details>
              <summary>Add lesson to this module</summary>
              <LessonForm courseId={course.id} moduleId={module.id} />
            </details>
          </div>
        );
      })}

      <div className="admin-card">
        <h3>Add module</h3>
        <ModuleForm courseId={course.id} />
      </div>
    </div>
  );
}
