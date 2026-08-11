import Link from "next/link";
import type { Metadata } from "next";
import { redirect } from "next/navigation";
import { getSessionToken } from "@/lib/session";
import { instructorListCourses } from "@/lib/instructor-api";
import { PublicationBadge } from "@/components/instructor/PublicationBadge";

export const metadata: Metadata = {
  title: "My Courses — Instructor",
};

export default async function InstructorCoursesPage({
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

  const result = await instructorListCourses(token, { page, limit: 20 });

  return (
    <div>
      <div className="admin-header">
        <h1>Мои курсы</h1>
        <Link href="/instructor/courses/new" className="btn-primary">
          Новый курс
        </Link>
      </div>

      {result.items.length === 0 && <p>У вас пока нет курсов.</p>}

      <div className="admin-table-wrapper">
        <table className="admin-table">
          <thead>
            <tr>
              <th>Название</th>
              <th>Уровень</th>
              <th>Статус</th>
              <th>Рейтинг</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {result.items.map((course) => (
              <tr key={course.id}>
                <td>{course.title}</td>
                <td>{course.level}</td>
                <td>
                  <PublicationBadge status={course.publication_status} />
                </td>
                <td>
                  {course.rating_count > 0 ? `★ ${course.rating_average.toFixed(1)} (${course.rating_count})` : "—"}
                </td>
                <td>
                  <Link href={`/instructor/courses/${course.id}`} className="btn-small">
                    Управлять
                  </Link>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="admin-pagination">
        <span>
          Страница {result.page} / {result.total_pages || 1} ({result.total} всего)
        </span>
        {result.page > 1 && <Link href={`/instructor/courses?page=${result.page - 1}`}>← Назад</Link>}
        {result.page < result.total_pages && <Link href={`/instructor/courses?page=${result.page + 1}`}>Вперёд →</Link>}
      </div>
    </div>
  );
}
