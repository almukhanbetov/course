import Link from "next/link";
import type { Metadata } from "next";
import { redirect } from "next/navigation";
import { getMySubscription, formatPrice } from "@/lib/api";
import { getSessionToken } from "@/lib/session";

export const metadata: Metadata = {
  title: "Моя подписка — LMS Platform",
};

function daysRemaining(expiresAt: string): number {
  const ms = new Date(expiresAt).getTime() - Date.now();
  return Math.max(0, Math.ceil(ms / (1000 * 60 * 60 * 24)));
}

export default async function SubscriptionDashboardPage() {
  const token = await getSessionToken();
  if (!token) {
    redirect("/login");
  }

  const subscription = await getMySubscription(token);

  return (
    <>
      <h1>Моя подписка</h1>

      {subscription.active && subscription.plan ? (
        <div className="admin-card">
          <p>
            Статус: <span className="badge badge-free">Активна</span>
          </p>
          <p>
            Тариф: <strong>{subscription.plan.name}</strong> ({formatPrice(subscription.plan)})
          </p>
          {subscription.starts_at && (
            <p>Начало: {new Date(subscription.starts_at).toLocaleDateString("ru-RU")}</p>
          )}
          {subscription.expires_at && (
            <>
              <p>Окончание: {new Date(subscription.expires_at).toLocaleDateString("ru-RU")}</p>
              <p className="my-course-meta">Осталось дней: {daysRemaining(subscription.expires_at)}</p>
            </>
          )}
        </div>
      ) : (
        <div className="admin-card">
          <p>
            Статус: <span className="badge">Нет активной подписки</span>
          </p>
          <p>Оформите подписку, чтобы получить доступ к курсам с пометкой «По подписке».</p>
          <Link href="/pricing" className="btn-primary">
            Выбрать тариф
          </Link>
        </div>
      )}
    </>
  );
}
