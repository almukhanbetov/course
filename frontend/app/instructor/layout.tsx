import Link from "next/link";
import { redirect } from "next/navigation";
import { getCurrentUser } from "@/lib/session";
import { logoutAction } from "@/lib/actions";

// Server-side gate mirroring admin/layout.tsx — a UX convenience, not the
// security boundary. Every /api/v1/instructor/* call is independently
// re-checked by RequireAuth + RequireAnyRole("instructor", "admin") plus
// the per-course ownership.Service check on the backend, which is the
// actual authority.
export default async function InstructorLayout({ children }: { children: React.ReactNode }) {
  const user = await getCurrentUser();

  if (!user) {
    redirect("/login");
  }
  if (user.role !== "instructor" && user.role !== "admin") {
    redirect("/dashboard");
  }

  return (
    <div className="admin-shell">
      <aside className="admin-sidebar instructor-sidebar">
        <p className="admin-sidebar-brand instructor-sidebar-brand">LMS Instructor</p>
        <nav className="admin-nav">
          <Link href="/instructor">Dashboard</Link>
          <Link href="/instructor/courses">My Courses</Link>
          <Link href="/instructor/submissions">Submissions</Link>
          <Link href="/instructor/students">Students</Link>
          <Link href="/instructor/analytics">Analytics</Link>
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
