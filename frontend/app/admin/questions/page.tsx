import type { Metadata } from "next";
import { redirect } from "next/navigation";
import { getCurrentUser, getSessionToken } from "@/lib/session";
import { adminGetCourse, adminListCourses } from "@/lib/admin-api";
import { getLessonQuestions } from "@/lib/api";
import { QAModerationSection, type ModerationGroup } from "@/components/QAModerationSection";

export const metadata: Metadata = {
  title: "Q&A — Admin",
};

// Same composition as app/instructor/questions/page.tsx, scoped to every
// course platform-wide instead of one instructor's own — see that page's
// doc comment for why this is application-level composition of the
// existing GET /lessons/:id/questions rather than a dedicated backend
// endpoint (none exists yet for Stage 20B2).
export default async function AdminQuestionsPage() {
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
    const coursesPage = await adminListCourses(token, { limit: 100 });
    const courseDetails = await Promise.all(
      coursesPage.items.map((c) => adminGetCourse(token, c.id).catch(() => null)),
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
        Вопросы и ответы по урокам всех курсов платформы. Отвечать может владелец курса или администратор; удалить
        можно только собственный вопрос или ответ.
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
