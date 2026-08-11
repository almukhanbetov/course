import Link from "next/link";
import type { Metadata } from "next";
import { redirect } from "next/navigation";
import { getCurrentUser, getSessionToken } from "@/lib/session";
import { getContinueLearning, getMyAnalytics, getMyRecommendations } from "@/lib/api";
import { logoutAction } from "@/lib/actions";
import { TimezoneSync } from "@/components/TimezoneSync";
import { ContinueLearningCard } from "@/components/ContinueLearningCard";
import { RecommendationCard } from "@/components/RecommendationCard";

export const metadata: Metadata = {
  title: "Личный кабинет — LMS Platform",
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
    <main>
      <TimezoneSync currentTimezone={user.timezone} />
      <h1>Личный кабинет</h1>
      <div className="status">
        <p>
          <strong>Имя:</strong> {user.first_name} {user.last_name}
        </p>
        <p>
          <strong>Email:</strong> {user.email}
        </p>
        <p>
          <strong>Роль:</strong> <span className="badge">{user.role}</span>
        </p>
      </div>

      {stats && (
        <div className="streak-summary">
          <span className="streak-flame">🔥</span>
          <span className="streak-count">{stats.current_streak} дней подряд</span>
          <span className="my-course-meta">
            {stats.lessons_completed} уроков завершено · {stats.courses_completed} курсов завершено
          </span>
          <Link href="/dashboard/analytics" className="btn-small">
            Подробнее
          </Link>
        </div>
      )}

      <div className="nav-row">
        <Link href="/dashboard/courses" className="nav-link">
          Мои курсы →
        </Link>
        <Link href="/dashboard/certificates" className="nav-link">
          Мои сертификаты →
        </Link>
        <Link href="/dashboard/subscription" className="nav-link">
          Моя подписка →
        </Link>
        <Link href="/dashboard/analytics" className="nav-link">
          Моя аналитика →
        </Link>
        <Link href="/dashboard/achievements" className="nav-link">
          Достижения →
        </Link>
        <Link href="/dashboard/wishlist" className="nav-link">
          Избранное →
        </Link>
      </div>

      {continueLearning.length > 0 && (
        <>
          <h2 className="mt-3">Продолжить обучение</h2>
          <div className="course-grid">
            {continueLearning.slice(0, 5).map((item) => (
              <ContinueLearningCard key={item.course_id} item={item} />
            ))}
          </div>
        </>
      )}

      {recommendations.length > 0 && (
        <>
          <h2 className="mt-3">Рекомендуем вам</h2>
          <div className="course-grid">
            {recommendations.map((rec) => (
              <RecommendationCard key={rec.course_id} rec={rec} />
            ))}
          </div>
        </>
      )}

      <form action={logoutAction}>
        <button type="submit" className="nav-link">
          Выйти
        </button>
      </form>
    </main>
  );
}
