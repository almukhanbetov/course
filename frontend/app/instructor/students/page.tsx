import Link from "next/link";
import type { Metadata } from "next";
import { redirect } from "next/navigation";
import { getSessionToken } from "@/lib/session";
import { instructorListStudents } from "@/lib/instructor-api";

export const metadata: Metadata = {
  title: "Students — Instructor",
};

export default async function InstructorStudentsPage({
  searchParams,
}: {
  searchParams: Promise<{ page?: string }>;
}) {
  const token = await getSessionToken();
  if (!token) {
    redirect("/login");
  }

  const { page: pageParam } = await searchParams;
  const page = Number(pageParam ?? "1") || 1;

  const result = await instructorListStudents(token, { page, limit: 20 });

  return (
    <div>
      <h1>Студенты</h1>
      <p className="subtitle">Все студенты, записанные хотя бы на один ваш курс.</p>

      {result.items.length === 0 && <p>Пока нет студентов ни на одном из ваших курсов.</p>}

      <div className="admin-table-wrapper">
        <table className="admin-table">
          <thead>
            <tr>
              <th>Студент</th>
              <th>Курс</th>
              <th>Записан</th>
              <th>Прогресс</th>
              <th>Завершён</th>
            </tr>
          </thead>
          <tbody>
            {result.items.map((s) => (
              <tr key={`${s.user_id}-${s.course_id}`}>
                <td>{s.display_name}</td>
                <td>{s.course_title}</td>
                <td>{new Date(s.enrolled_at).toLocaleDateString("ru-RU")}</td>
                <td>{s.progress_percent}%</td>
                <td>{s.completed ? "✓" : "—"}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="admin-pagination">
        <span>
          Страница {result.page} / {result.total_pages || 1} ({result.total} всего)
        </span>
        {result.page > 1 && <Link href={`/instructor/students?page=${result.page - 1}`}>← Назад</Link>}
        {result.page < result.total_pages && <Link href={`/instructor/students?page=${result.page + 1}`}>Вперёд →</Link>}
      </div>
    </div>
  );
}
