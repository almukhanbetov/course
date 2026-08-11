import { redirect } from "next/navigation";
import { getCurrentUser } from "@/lib/session";
import { logoutAction } from "@/lib/actions";
import { SidebarShell } from "@/components/shell/SidebarShell";
import type { SidebarNavGroup } from "@/components/shell/Sidebar";
import { IconBarChart, IconClipboard, IconCourses, IconDashboard, IconGraduationCap, IconUsers } from "@/components/shell/icons";

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

  const groups: SidebarNavGroup[] = [
    {
      items: [
        { href: "/instructor", label: "Dashboard", icon: <IconDashboard size={18} />, exact: true },
        { href: "/instructor/courses", label: "My Courses", icon: <IconCourses size={18} /> },
        { href: "/instructor/submissions", label: "Submissions", icon: <IconClipboard size={18} /> },
        { href: "/instructor/students", label: "Students", icon: <IconUsers size={18} /> },
        { href: "/instructor/analytics", label: "Analytics", icon: <IconBarChart size={18} /> },
      ],
    },
  ];

  const userSlot = (
    <>
      <span>
        {user.first_name} {user.last_name} <span className="badge">{user.role}</span>
      </span>
      <form action={logoutAction}>
        <button type="submit" className="nav-link">
          Выйти
        </button>
      </form>
    </>
  );

  return (
    <SidebarShell
      brand="LMS Instructor"
      brandIcon={<IconGraduationCap size={18} />}
      sidebarClassName="instructor-sidebar"
      brandClassName="instructor-sidebar-brand"
      groups={groups}
      userSlot={userSlot}
    >
      {children}
    </SidebarShell>
  );
}
