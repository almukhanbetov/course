import { redirect } from "next/navigation";
import { getCurrentUser } from "@/lib/session";
import { logoutAction } from "@/lib/actions";
import { SidebarShell } from "@/components/shell/SidebarShell";
import type { SidebarNavGroup } from "@/components/shell/Sidebar";
import {
  IconAward,
  IconBarChart,
  IconBell,
  IconCourses,
  IconCreditCard,
  IconDashboard,
  IconGraduationCap,
  IconHeart,
  IconPlayCircle,
  IconShield,
  IconSparkles,
} from "@/components/shell/icons";

// UX convenience only, not the security boundary — every /api/v1/me/* call
// is independently re-checked by RequireAuth() on the backend.
export default async function DashboardLayout({ children }: { children: React.ReactNode }) {
  const user = await getCurrentUser();

  if (!user) {
    redirect("/login");
  }

  const groups: SidebarNavGroup[] = [
    {
      label: "Обучение",
      items: [
        { href: "/dashboard", label: "Личный кабинет", icon: <IconDashboard size={18} />, exact: true },
        { href: "/dashboard/courses", label: "Мои курсы", icon: <IconCourses size={18} /> },
        {
          href: "/dashboard#continue-learning",
          label: "Продолжить обучение",
          icon: <IconPlayCircle size={18} />,
          exact: true,
        },
        { href: "/dashboard#recommendations", label: "Рекомендации", icon: <IconSparkles size={18} />, exact: true },
        { href: "/dashboard/wishlist", label: "Избранное", icon: <IconHeart size={18} /> },
      ],
    },
    {
      label: "Прогресс",
      items: [
        { href: "/dashboard/analytics", label: "Аналитика", icon: <IconBarChart size={18} /> },
        { href: "/dashboard/achievements", label: "Достижения", icon: <IconAward size={18} /> },
        { href: "/dashboard/certificates", label: "Сертификаты", icon: <IconGraduationCap size={18} /> },
      ],
    },
    {
      label: "Аккаунт",
      items: [
        { href: "/dashboard/subscription", label: "Подписка", icon: <IconCreditCard size={18} /> },
        { href: "/dashboard/notifications", label: "Уведомления", icon: <IconBell size={18} /> },
      ],
    },
  ];

  if (user.role === "admin") {
    groups.push({
      label: "Управление",
      items: [{ href: "/admin", label: "Admin CMS", icon: <IconShield size={18} /> }],
    });
  } else if (user.role === "instructor") {
    groups.push({
      label: "Управление",
      items: [{ href: "/instructor", label: "Instructor", icon: <IconGraduationCap size={18} /> }],
    });
  }

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
    <SidebarShell brand="LMS Platform" brandIcon={<IconGraduationCap size={18} />} groups={groups} userSlot={userSlot}>
      {children}
    </SidebarShell>
  );
}
