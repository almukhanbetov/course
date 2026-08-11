import Link from "next/link";
import { notFound, redirect } from "next/navigation";
import { getSessionToken } from "@/lib/session";
import { instructorGetCourse, instructorListCourseStudents } from "@/lib/instructor-api";

export default async function InstructorCourseStudentsPage({
  params,
  searchParams,
}: {
  params: Promise<{ id: string }>;
  searchParams: Promise<{ page?: string }>;
}) {
  const { id } = await params;
  const { page: pageParam } = await searchParams;
  const page = Number(pageParam ?? "1") || 1;

  const token = await getSessionToken();
  if (!token) {
    redirect("/login");
  }

  const course = await instructorGetCourse(token, id);
  if (!course) {
    notFound();
  }

  const result = await instructorListCourseStudents(token, id, { page, limit: 20 });

  return (
    <div>
      <Link href={`/instructor/courses/${id}`}>← {course.title}</Link>
      <h1>Студенты курса «{course.title}»</h1>

      {result.items.length === 0 && <p>На этот курс пока никто не записался.</p>}

      <div className="admin-table-wrapper">
        <table className="admin-table">
          <thead>
            <tr>
              <th>Студент</th>
              <th>Записан</th>
              <th>Прогресс</th>
              <th>Финальный тест</th>
              <th>Завершён</th>
            </tr>
          </thead>
          <tbody>
            {result.items.map((s) => (
              <tr key={s.user_id}>
                <td>{s.display_name}</td>
                <td>{new Date(s.enrolled_at).toLocaleDateString("ru-RU")}</td>
                <td>
                  {s.completed_lessons}/{s.total_lessons} ({s.progress_percent}%)
                </td>
                <td>{s.final_test_passed ? "✓ сдан" : "—"}</td>
                <td>{s.completed ? `✓ ${new Date(s.completed_at!).toLocaleDateString("ru-RU")}` : "—"}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="admin-pagination">
        <span>
          Страница {result.page} / {result.total_pages || 1} ({result.total} всего)
        </span>
        {result.page > 1 && <Link href={`/instructor/courses/${id}/students?page=${result.page - 1}`}>← Назад</Link>}
        {result.page < result.total_pages && (
          <Link href={`/instructor/courses/${id}/students?page=${result.page + 1}`}>Вперёд →</Link>
        )}
      </div>
    </div>
  );
}
