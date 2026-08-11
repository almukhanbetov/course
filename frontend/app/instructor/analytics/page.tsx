import Link from "next/link";
import type { Metadata } from "next";
import { redirect } from "next/navigation";
import { getSessionToken } from "@/lib/session";
import { instructorGetStats, instructorListCourses } from "@/lib/instructor-api";
import { PublicationBadge } from "@/components/instructor/PublicationBadge";

export const metadata: Metadata = {
  title: "Analytics — Instructor",
};

export default async function InstructorAnalyticsPage() {
  const token = await getSessionToken();
  if (!token) {
    redirect("/login");
  }

  const [stats, courses] = await Promise.all([instructorGetStats(token), instructorListCourses(token, { limit: 100 })]);

  return (
    <div>
      <h1>Аналитика</h1>

      <div className="stat-grid">
        <div className="stat-tile">
          <div className="value">{stats.courses_count}</div>
          <div className="label">Мои курсы</div>
        </div>
        <div className="stat-tile">
          <div className="value">{stats.published_courses}</div>
          <div className="label">Опубликовано</div>
        </div>
        <div className="stat-tile">
          <div className="value">{stats.students_count}</div>
          <div className="label">Студентов</div>
        </div>
        <div className="stat-tile">
          <div className="value">{stats.active_enrollments}</div>
          <div className="label">Активных зачислений</div>
        </div>
        <div className="stat-tile">
          <div className="value">{stats.completed_enrollments}</div>
          <div className="label">Завершили курс</div>
        </div>
        <div className="stat-tile">
          <div className="value">{stats.average_completion_percent.toFixed(0)}%</div>
          <div className="label">Средний прогресс</div>
        </div>
        <div className="stat-tile">
          <div className="value">{stats.certificates_issued}</div>
          <div className="label">Сертификатов выдано</div>
        </div>
      </div>

      <h2 className="mt-3">По курсам</h2>
      <div className="admin-table-wrapper">
        <table className="admin-table">
          <thead>
            <tr>
              <th>Курс</th>
              <th>Статус</th>
              <th>Рейтинг</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {courses.items.map((course) => (
              <tr key={course.id}>
                <td>{course.title}</td>
                <td>
                  <PublicationBadge status={course.publication_status} />
                </td>
                <td>{course.rating_count > 0 ? `★ ${course.rating_average.toFixed(1)} (${course.rating_count})` : "—"}</td>
                <td>
                  <Link href={`/instructor/courses/${course.id}`} className="btn-small">
                    Подробнее
                  </Link>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
