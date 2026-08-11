import type { Metadata } from "next";
import { redirect } from "next/navigation";
import { getSessionToken } from "@/lib/session";
import { adminGetStats } from "@/lib/admin-api";

export const metadata: Metadata = {
  title: "Admin Dashboard — LMS Platform",
};

export default async function AdminDashboardPage() {
  const token = await getSessionToken();
  if (!token) {
    redirect("/login");
  }

  const stats = await adminGetStats(token);

  const tiles: { label: string; value: number }[] = [
    { label: "Users", value: stats.users_count },
    { label: "Courses", value: stats.courses_count },
    { label: "Published courses", value: stats.published_courses_count },
    { label: "Specialities", value: stats.specialities_count },
    { label: "Enrollments", value: stats.enrollments_count },
    { label: "Completed courses", value: stats.completed_courses_count },
    { label: "Certificates", value: stats.certificates_count },
    { label: "Test attempts", value: stats.test_attempts_count },
    { label: "Daily active learners", value: stats.daily_active_learners },
    { label: "Weekly active learners", value: stats.weekly_active_learners },
    { label: "Lessons completed today", value: stats.lessons_completed_today },
    { label: "Courses completed this month", value: stats.courses_completed_this_month },
    { label: "Certificates this month", value: stats.certificates_this_month },
  ];

  return (
    <div>
      <h1>Dashboard</h1>
      <div className="stat-grid">
        {tiles.map((tile) => (
          <div key={tile.label} className="stat-tile">
            <span className="value">{tile.value}</span>
            <span className="label">{tile.label}</span>
          </div>
        ))}
      </div>
    </div>
  );
}
