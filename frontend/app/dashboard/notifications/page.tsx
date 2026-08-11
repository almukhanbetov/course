import Link from "next/link";
import type { Metadata } from "next";
import { redirect } from "next/navigation";
import { getSessionToken } from "@/lib/session";
import { getMyNotifications, notificationActionLink } from "@/lib/api";
import { markNotificationReadAction, markAllNotificationsReadAction } from "@/lib/actions";

export const metadata: Metadata = {
  title: "Уведомления — LMS Platform",
};

export default async function NotificationsPage({
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

  const result = await getMyNotifications(token, page, 20);

  return (
    <main>
      <div className="admin-header">
        <h1>Уведомления</h1>
        <form action={markAllNotificationsReadAction}>
          <button type="submit" className="btn-secondary">
            Отметить всё прочитанным
          </button>
        </form>
      </div>

      {result.items.length === 0 && <p>У вас пока нет уведомлений.</p>}

      <div className="notification-list">
        {result.items.map((n) => {
          const link = notificationActionLink(n);
          const isUnread = !n.read_at;
          return (
            <div key={n.id} className={`notification-item${isUnread ? " unread" : ""}`}>
              <div>
                <strong>{n.title}</strong>
                <p className="my-course-meta">{n.message}</p>
                <p className="my-course-meta">{new Date(n.created_at).toLocaleString("ru-RU")}</p>
              </div>
              <div className="admin-inline-actions">
                {link && (
                  <Link href={link} className="btn-small">
                    Открыть
                  </Link>
                )}
                {isUnread && (
                  <form action={markNotificationReadAction.bind(null, n.id)}>
                    <button type="submit" className="btn-small">
                      Прочитано
                    </button>
                  </form>
                )}
              </div>
            </div>
          );
        })}
      </div>

      <div className="admin-pagination">
        <span>
          Страница {result.page} / {result.total_pages || 1} ({result.total} всего)
        </span>
        {result.page > 1 && <Link href={`/dashboard/notifications?page=${result.page - 1}`}>← Назад</Link>}
        {result.page < result.total_pages && (
          <Link href={`/dashboard/notifications?page=${result.page + 1}`}>Далее →</Link>
        )}
      </div>
    </main>
  );
}
