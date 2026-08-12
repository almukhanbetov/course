import { redirect } from "next/navigation";
import { getCurrentUser } from "@/lib/session";
import { logoutAction } from "@/lib/actions";
import { SidebarShell } from "@/components/shell/SidebarShell";
import type { SidebarNavGroup } from "@/components/shell/Sidebar";
import {
  IconAward,
  IconBarChart,
  IconBell,
  IconClipboard,
  IconCourses,
  IconCreditCard,
  IconDashboard,
  IconFileText,
  IconFlag,
  IconLayers,
  IconMessageCircle,
  IconShield,
  IconStar,
  IconTag,
  IconUsers,
} from "@/components/shell/icons";

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

  const groups: SidebarNavGroup[] = [
    { items: [{ href: "/admin", label: "Dashboard", icon: <IconDashboard size={18} />, exact: true }] },
    {
      label: "Каталог",
      items: [
        { href: "/admin/courses", label: "Courses", icon: <IconCourses size={18} /> },
        { href: "/admin/course-submissions", label: "Course Submissions", icon: <IconFileText size={18} /> },
        { href: "/admin/categories", label: "Categories", icon: <IconTag size={18} /> },
        { href: "/admin/specialities", label: "Specialities", icon: <IconLayers size={18} /> },
        { href: "/admin/tests", label: "Tests", icon: <IconFileText size={18} /> },
      ],
    },
    {
      label: "Сообщество",
      items: [
        { href: "/admin/users", label: "Users", icon: <IconUsers size={18} /> },
        { href: "/admin/reviews", label: "Reviews", icon: <IconStar size={18} /> },
        { href: "/admin/questions", label: "Q&A", icon: <IconMessageCircle size={18} /> },
        { href: "/admin/reports", label: "Reports", icon: <IconFlag size={18} /> },
        { href: "/admin/audit-log", label: "Audit Log", icon: <IconClipboard size={18} /> },
        { href: "/admin/certificates", label: "Certificates", icon: <IconAward size={18} /> },
        { href: "/admin/notifications", label: "Notifications", icon: <IconBell size={18} /> },
      ],
    },
    {
      label: "Биллинг",
      items: [
        { href: "/admin/plans", label: "Plans", icon: <IconCreditCard size={18} /> },
        { href: "/admin/subscriptions", label: "Subscriptions", icon: <IconCreditCard size={18} /> },
        { href: "/admin/payments", label: "Payments", icon: <IconCreditCard size={18} /> },
        { href: "/admin/analytics", label: "Analytics", icon: <IconBarChart size={18} /> },
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
    <SidebarShell brand="LMS Admin" brandIcon={<IconShield size={18} />} groups={groups} userSlot={userSlot}>
      {children}
    </SidebarShell>
  );
}
