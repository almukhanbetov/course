import Link from "next/link";
import type { Metadata } from "next";
import { redirect } from "next/navigation";
import { getSessionToken } from "@/lib/session";
import { getMyAchievements } from "@/lib/api";

export const metadata: Metadata = {
  title: "Мои достижения — LMS Platform",
};

export default async function AchievementsPage() {
  const token = await getSessionToken();
  if (!token) {
    redirect("/login");
  }

  const view = await getMyAchievements(token);

  return (
    <>
      <Link href="/dashboard/analytics">← Моя аналитика</Link>
      <h1>Мои достижения</h1>

      <h2>Получено ({view.earned.length})</h2>
      {view.earned.length === 0 && <p className="my-course-meta">Пока нет полученных достижений — начните с первого урока!</p>}
      <div className="achievement-grid">
        {view.earned.map((a) => (
          <div key={a.code} className="achievement-card">
            <span className="achievement-icon">{a.icon}</span>
            <div>
              <h4>{a.title}</h4>
              <p>{a.description}</p>
              <p className="activity-date">{new Date(a.earned_at).toLocaleDateString("ru-RU")}</p>
            </div>
          </div>
        ))}
      </div>

      <h2 className="mt-3">Ещё не получено ({view.locked.length})</h2>
      <div className="achievement-grid">
        {view.locked.map((a) => (
          <div key={a.code} className="achievement-card locked">
            <span className="achievement-icon">{a.icon}</span>
            <div>
              <h4>{a.title}</h4>
              <p>{a.description}</p>
            </div>
          </div>
        ))}
      </div>
    </>
  );
}
