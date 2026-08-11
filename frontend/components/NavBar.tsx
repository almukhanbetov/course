import Link from "next/link";
import type { PublicUser } from "@/lib/api";
import { logoutAction } from "@/lib/actions";

export function NavBar({ user, unreadCount = 0 }: { user: PublicUser | null; unreadCount?: number }) {
  return (
    <nav className="navbar">
      <div className="navbar-inner">
        <Link href="/" className="navbar-brand">
          LMS Platform
        </Link>
        <div className="navbar-links">
          <Link href="/courses">Курсы</Link>
          <Link href="/specialities">Специальности</Link>
          <Link href="/pricing">Тарифы</Link>
          {user ? (
            <>
              <Link href="/dashboard/courses">Мои курсы</Link>
              <Link href="/dashboard/certificates">Сертификаты</Link>
              <Link href="/dashboard/subscription">Подписка</Link>
              {user.role === "admin" && <Link href="/admin">Admin</Link>}
              <Link href="/dashboard/notifications" className="bell-link" aria-label="Уведомления">
                🔔
                {unreadCount > 0 && <span className="bell-badge">{unreadCount > 99 ? "99+" : unreadCount}</span>}
              </Link>
              <Link href="/dashboard">{user.first_name}</Link>
              <form action={logoutAction} className="navbar-form">
                <button type="submit" className="nav-link">
                  Выйти
                </button>
              </form>
            </>
          ) : (
            <>
              <Link href="/login">Войти</Link>
              <Link href="/register">Регистрация</Link>
            </>
          )}
        </div>
      </div>
    </nav>
  );
}
