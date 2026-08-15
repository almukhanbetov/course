import Link from "next/link";
import type { PublicUser } from "@/lib/api";
import { logoutAction } from "@/lib/actions";
import { IconBell, IconSearch } from "@/components/shell/icons";
import { NavBarMobileMenu } from "@/components/NavBarMobileMenu";
import { ThemeToggle } from "@/components/ThemeToggle";
import { LocaleSwitcher } from "@/components/LocaleSwitcher";
import { getDictionary } from "@/lib/i18n/dictionaries";
import type { Locale } from "@/lib/i18n/locale";

export function NavBar({
  user,
  unreadCount = 0,
  locale,
}: {
  user: PublicUser | null;
  unreadCount?: number;
  locale: Locale;
}) {
  const initials = user ? `${user.first_name[0] ?? ""}${user.last_name[0] ?? ""}`.toUpperCase() : "";
  const dict = getDictionary(locale);

  return (
    <nav className="navbar">
      <div className="navbar-inner">
        <Link href="/" className="navbar-brand">
          <span className="navbar-brand-mark">LMS</span>
          LMS Platform
        </Link>
        <NavBarMobileMenu>
          <form action="/courses" method="GET" className="navbar-search">
            <IconSearch size={18} />
            <input type="search" name="q" placeholder={dict.nav.searchPlaceholder} aria-label={dict.nav.searchAriaLabel} />
          </form>
          <Link href="/courses">{dict.nav.courses}</Link>
          <Link href="/specialities">{dict.nav.specialities}</Link>
          <Link href="/pricing">{dict.nav.pricing}</Link>
          <LocaleSwitcher locale={locale} />
          <ThemeToggle />
          {user ? (
            <>
              <span className="navbar-divider" />
              {user.role === "instructor" && <Link href="/instructor">Instructor</Link>}
              {user.role === "admin" && <Link href="/admin">Admin</Link>}
              <Link href="/dashboard/notifications" className="bell-link" aria-label={dict.nav.notifications}>
                <IconBell size={19} />
                {unreadCount > 0 && <span className="bell-badge">{unreadCount > 99 ? "99+" : unreadCount}</span>}
              </Link>
              <Link href="/dashboard" className="navbar-user">
                <span className="navbar-avatar">{initials}</span>
                {user.first_name}
              </Link>
              <form action={logoutAction} className="navbar-form">
                <button type="submit" className="nav-link" style={{ marginTop: 0 }}>
                  {dict.nav.logout}
                </button>
              </form>
            </>
          ) : (
            <>
              <span className="navbar-divider" />
              <Link href="/login">{dict.nav.login}</Link>
              <Link href="/register" className="btn-primary" style={{ padding: "0.5rem 1rem", fontSize: "0.88rem" }}>
                {dict.nav.register}
              </Link>
            </>
          )}
        </NavBarMobileMenu>
      </div>
    </nav>
  );
}
