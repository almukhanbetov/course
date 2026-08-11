import Link from "next/link";
import type { Metadata } from "next";
import { redirect } from "next/navigation";
import { getSessionToken, getCurrentUser } from "@/lib/session";
import { activityDisplayText, getMyActivityCalendar, getMyAnalytics, getMyRecentActivity } from "@/lib/api";
import { ActivityCalendar } from "@/components/ActivityCalendar";
import { TimezoneSync } from "@/components/TimezoneSync";

export const metadata: Metadata = {
  title: "Моя аналитика — LMS Platform",
};

export default async function AnalyticsPage() {
  const token = await getSessionToken();
  if (!token) {
    redirect("/login");
  }
  const user = await getCurrentUser();

  const [stats, calendar, recent] = await Promise.all([
    getMyAnalytics(token),
    getMyActivityCalendar(token),
    getMyRecentActivity(token),
  ]);

  return (
    <>
      {user && <TimezoneSync currentTimezone={user.timezone} />}
      <Link href="/dashboard">← Личный кабинет</Link>
      <h1>Моя аналитика</h1>

      <div className="streak-summary">
        <span className="streak-flame">🔥</span>
        <span className="streak-count">{stats.current_streak} дней подряд</span>
        <span className="my-course-meta">Рекорд: {stats.longest_streak} дней</span>
      </div>

      <div className="stat-grid">
        <div className="stat-tile">
          <div className="value">{stats.lessons_completed}</div>
          <div className="label">Уроков завершено</div>
        </div>
        <div className="stat-tile">
          <div className="value">{stats.courses_completed}</div>
          <div className="label">Курсов завершено</div>
        </div>
        <div className="stat-tile">
          <div className="value">{stats.courses_enrolled}</div>
          <div className="label">Курсов записано</div>
        </div>
        <div className="stat-tile">
          <div className="value">{stats.assignments_approved}</div>
          <div className="label">Заданий принято</div>
        </div>
        <div className="stat-tile">
          <div className="value">{stats.coding_exercises_passed}</div>
          <div className="label">Упражнений по коду</div>
        </div>
        <div className="stat-tile">
          <div className="value">{stats.tests_passed}</div>
          <div className="label">Тестов пройдено</div>
        </div>
        <div className="stat-tile">
          <div className="value">{stats.certificates}</div>
          <div className="label">Сертификатов</div>
        </div>
      </div>

      <h2 className="mt-3">Календарь активности</h2>
      <ActivityCalendar days={calendar} />

      <h2 className="mt-3">Последние события</h2>
      {recent.length === 0 && <p className="my-course-meta">Пока нет активности.</p>}
      <div className="recent-activity-list">
        {recent.map((entry) => (
          <div key={entry.id} className="recent-activity-item">
            <div>{activityDisplayText(entry)}</div>
            <div className="activity-date">{new Date(entry.occurred_at).toLocaleString("ru-RU")}</div>
          </div>
        ))}
      </div>

      <p className="mt-3">
        <Link href="/dashboard/achievements">Мои достижения →</Link>
      </p>
    </>
  );
}
