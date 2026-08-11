import Link from "next/link";
import { redirect } from "next/navigation";
import { getCurrentUser } from "@/lib/session";
import { logoutAction } from "@/lib/actions";

// Server-side gate: a student or instructor (or anyone unauthenticated) is
// redirected away before any admin page or data fetch runs. This is a UX
// convenience, not the security boundary — every /api/v1/admin/* call is
// independently re-checked by RequireAuth()+RequireRole("admin") on the
// backend, which is the actual authority (see internal/auth/middleware.go).
export default async function AdminLayout({ children }: { children: React.ReactNode }) {
  const user = await getCurrentUser();

  if (!user) {
    redirect("/login");
  }
  if (user.role !== "admin") {
    redirect("/dashboard");
  }

  return (
    <div className="admin-shell">
      <aside className="admin-sidebar">
        <p className="admin-sidebar-brand">LMS Admin</p>
        <nav className="admin-nav">
          <Link href="/admin">Dashboard</Link>
          <Link href="/admin/users">Users</Link>
          <Link href="/admin/courses">Courses</Link>
          <Link href="/admin/course-submissions">Course Submissions</Link>
          <Link href="/admin/categories">Categories</Link>
          <Link href="/admin/reviews">Reviews</Link>
          <Link href="/admin/specialities">Specialities</Link>
          <Link href="/admin/tests">Tests</Link>
          <Link href="/admin/certificates">Certificates</Link>
          <Link href="/admin/plans">Plans</Link>
          <Link href="/admin/subscriptions">Subscriptions</Link>
          <Link href="/admin/payments">Payments</Link>
          <Link href="/admin/notifications">Notifications</Link>
        </nav>
      </aside>

      <div className="admin-main">
        <header className="admin-topbar">
          <span>
            {user.first_name} {user.last_name} <span className="badge">{user.role}</span>
          </span>
          <form action={logoutAction}>
            <button type="submit" className="nav-link">
              Выйти
            </button>
          </form>
        </header>
        <main className="admin-content">{children}</main>
      </div>
    </div>
  );
}
