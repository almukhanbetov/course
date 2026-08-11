import Link from "next/link";
import type { Metadata } from "next";
import { redirect } from "next/navigation";
import { getSessionToken } from "@/lib/session";
import { instructorGetStats, instructorListCourses } from "@/lib/instructor-api";
import { PublicationBadge } from "@/components/instructor/PublicationBadge";

export const metadata: Metadata = {
  title: "Instructor Dashboard",
};

export default async function InstructorDashboardPage() {
  const token = await getSessionToken();
  if (!token) {
    redirect("/login");
  }

  const [stats, courses] = await Promise.all([
    instructorGetStats(token),
    instructorListCourses(token, { limit: 5 }),
  ]);

  return (
    <div>
      <div className="admin-header">
        <h1>Dashboard</h1>
        <Link href="/instructor/courses/new" className="btn-primary">
          Новый курс
        </Link>
      </div>

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

      <h2 className="mt-3">Домашние задания</h2>
      <div className="stat-grid">
        <Link href="/instructor/submissions?status=submitted" className="stat-tile">
          <div className="value">{stats.submissions_awaiting_review}</div>
          <div className="label">Ожидают проверки</div>
        </Link>
        <Link href="/instructor/submissions?status=needs_revision" className="stat-tile">
          <div className="value">{stats.submissions_needs_revision}</div>
          <div className="label">На доработке у студентов</div>
        </Link>
      </div>

      <h2 className="mt-3">Последние курсы</h2>
      {courses.items.length === 0 && <p>У вас пока нет курсов.</p>}
      {courses.items.map((course) => (
        <div key={course.id} className="instructor-course-card">
          <div className="admin-inline-actions">
            <h3>{course.title}</h3>
            <PublicationBadge status={course.publication_status} />
          </div>
          <Link href={`/instructor/courses/${course.id}`} className="btn-small">
            Управлять
          </Link>
        </div>
      ))}
    </div>
  );
}
