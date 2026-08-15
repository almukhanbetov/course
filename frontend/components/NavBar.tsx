import Link from "next/link";
import type { PublicUser } from "@/lib/api";
import { logoutAction } from "@/lib/actions";
import { IconBell, IconSearch } from "@/components/shell/icons";
import { NavBarMobileMenu } from "@/components/NavBarMobileMenu";

export function NavBar({ user, unreadCount = 0 }: { user: PublicUser | null; unreadCount?: number }) {
  const initials = user ? `${user.first_name[0] ?? ""}${user.last_name[0] ?? ""}`.toUpperCase() : "";

  return (
    <nav className="navbar">
      <div className="navbar-inner">
        <Link href="/" className="navbar-brand">
          <span className="navbar-brand-mark">LMS</span>
          LMS Platform
        </Link>
        <NavBarMobileMenu>
          <form action="/courses" method="GET" className="navbar-search">
            <IconSearch size={16} />
            <input type="search" name="q" placeholder="Поиск курсов..." aria-label="Поиск курсов" />
          </form>
          <Link href="/courses">Курсы</Link>
          <Link href="/specialities">Специальности</Link>
          <Link href="/pricing">Тарифы</Link>
          {user ? (
            <>
              <span className="navbar-divider" />
              {user.role === "instructor" && <Link href="/instructor">Instructor</Link>}
              {user.role === "admin" && <Link href="/admin">Admin</Link>}
              <Link href="/dashboard/notifications" className="bell-link" aria-label="Уведомления">
                <IconBell size={19} />
                {unreadCount > 0 && <span className="bell-badge">{unreadCount > 99 ? "99+" : unreadCount}</span>}
              </Link>
              <Link href="/dashboard" className="navbar-user">
                <span className="navbar-avatar">{initials}</span>
                {user.first_name}
              </Link>
              <form action={logoutAction} className="navbar-form">
                <button type="submit" className="nav-link" style={{ marginTop: 0 }}>
                  Выйти
                </button>
              </form>
            </>
          ) : (
            <>
              <span className="navbar-divider" />
              <Link href="/login">Войти</Link>
              <Link href="/register" className="btn-primary" style={{ padding: "0.5rem 1rem", fontSize: "0.88rem" }}>
                Регистрация
              </Link>
            </>
          )}
        </NavBarMobileMenu>
      </div>
    </nav>
  );
}
