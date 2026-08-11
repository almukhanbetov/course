import Link from "next/link";
import type { Metadata } from "next";
import { redirect } from "next/navigation";
import { getCurrentUser, getSessionToken } from "@/lib/session";
import { getContinueLearning, getMyAnalytics, getMyRecommendations } from "@/lib/api";
import { TimezoneSync } from "@/components/TimezoneSync";
import { ContinueLearningCard } from "@/components/ContinueLearningCard";
import { RecommendationCard } from "@/components/RecommendationCard";

export const metadata: Metadata = {
  title: "Личный кабинет — LMS Platform",
};

const ROLE_LABEL: Record<string, string> = {
  student: "Студент",
  instructor: "Преподаватель",
  admin: "Администратор",
};

export default async function DashboardPage() {
  const user = await getCurrentUser();

  if (!user) {
    redirect("/login");
  }

  const token = await getSessionToken();
  const stats = token ? await getMyAnalytics(token).catch(() => null) : null;
  const continueLearning = token ? await getContinueLearning(token).catch(() => []) : [];
  const recommendations = token ? await getMyRecommendations(token).catch(() => []) : [];

  return (
    <>
      <TimezoneSync currentTimezone={user.timezone} />

      <div className="dashboard-hero">
        <div>
          <h1 className="dashboard-hero-title">С возвращением, {user.first_name}!</h1>
          <div className="dashboard-hero-meta">
            <span>{user.email}</span>
            <span>·</span>
            <span className="badge">{ROLE_LABEL[user.role] ?? user.role}</span>
          </div>
        </div>

        {stats && (
          <div className="streak-summary">
            <span className="streak-flame">🔥</span>
            <span className="streak-count">{stats.current_streak} дней подряд</span>
            <span className="my-course-meta">
              {stats.lessons_completed} уроков · {stats.courses_completed} курсов завершено
            </span>
            <Link href="/dashboard/analytics" className="btn-small">
              Подробнее
            </Link>
          </div>
        )}
      </div>

      {continueLearning.length > 0 && (
        <section id="continue-learning">
          <div className="section-header">
            <h2>Продолжить обучение</h2>
          </div>
          <div className="course-grid">
            {continueLearning.slice(0, 5).map((item) => (
              <ContinueLearningCard key={item.course_id} item={item} />
            ))}
          </div>
        </section>
      )}

      {recommendations.length > 0 && (
        <section id="recommendations">
          <div className="section-header">
            <h2>Рекомендуем вам</h2>
            <Link href="/courses" className="nav-link">
              Весь каталог →
            </Link>
          </div>
          <div className="course-grid">
            {recommendations.map((rec) => (
              <RecommendationCard key={rec.course_id} rec={rec} />
            ))}
          </div>
        </section>
      )}

      {continueLearning.length === 0 && recommendations.length === 0 && (
        <div className="empty-state mt-3">
          <p>Начните обучение, чтобы увидеть здесь свой прогресс и рекомендации.</p>
          <Link href="/courses" className="btn-primary">
            Смотреть каталог курсов
          </Link>
        </div>
      )}
    </>
  );
}
