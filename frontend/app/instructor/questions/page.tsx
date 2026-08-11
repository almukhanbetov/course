import type { Metadata } from "next";
import { redirect } from "next/navigation";
import { getCurrentUser, getSessionToken } from "@/lib/session";
import { instructorGetCourse, instructorListCourses } from "@/lib/instructor-api";
import { getLessonQuestions } from "@/lib/api";
import { QAModerationSection, type ModerationGroup } from "@/components/QAModerationSection";

export const metadata: Metadata = {
  title: "Q&A — Instructor",
};

// Reuses the same GET /lessons/:id/questions endpoint the student-facing
// lesson page uses (Stage 20A never gated that read behind ownership or
// enrollment — only auth), composed here across every lesson of every
// course this instructor owns. There is no dedicated "list Q&A for my
// courses" backend endpoint (see STAGE20_PROGRESS.md's Stage 20B2 section
// for why that was deliberately not added this session), so this page
// fetches course -> lesson -> questions in application code instead of a
// single query — acceptable at this project's demo-data scale (a handful
// of courses/lessons), not something this session claims scales further.
export default async function InstructorQuestionsPage() {
  const token = await getSessionToken();
  if (!token) {
    redirect("/login");
  }
  const user = await getCurrentUser();
  if (!user) {
    redirect("/login");
  }

  let groups: ModerationGroup[] = [];
  let loadError: string | null = null;

  try {
    const coursesPage = await instructorListCourses(token, { limit: 100 });
    const courseDetails = await Promise.all(
      coursesPage.items.map((c) => instructorGetCourse(token, c.id).catch(() => null)),
    );

    const lessonRefs = courseDetails
      .filter((detail): detail is NonNullable<typeof detail> => detail !== null)
      .flatMap((detail) =>
        detail.modules.flatMap((module) =>
          module.lessons.map((lesson) => ({
            courseId: detail.id,
            courseTitle: detail.title,
            lessonId: lesson.id,
            lessonTitle: lesson.title,
          })),
        ),
      );

    const rawGroups = await Promise.all(
      lessonRefs.map(async (ref) => {
        const page = await getLessonQuestions(token, ref.lessonId).catch(() => null);
        if (!page || page.items.length === 0) return null;
        return { ...ref, questions: page.items };
      }),
    );
    groups = rawGroups.filter((g): g is ModerationGroup => g !== null);
  } catch (err) {
    loadError = err instanceof Error ? err.message : "Unknown error";
  }

  return (
    <div>
      <div className="admin-header">
        <h1>Вопросы студентов</h1>
      </div>
      <p className="subtitle">
        Вопросы и ответы по урокам ваших курсов. Отвечать может преподаватель-владелец курса или администратор;
        удалить можно только собственный вопрос или ответ.
      </p>

      {loadError ? (
        <p role="alert">Не удалось загрузить вопросы: {loadError}</p>
      ) : (
        <QAModerationSection
          groups={groups}
          currentUserId={user.id}
          currentUserName={`${user.first_name} ${user.last_name}`.trim()}
        />
      )}
    </div>
  );
}
