"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

export interface SidebarNavItem {
  href: string;
  label: string;
  // A pre-rendered element (e.g. <IconDashboard size={18} />), not a
  // function reference — plain functions can't cross the Server->Client
  // boundary (these groups are built in a Server Component layout and
  // handed to this "use client" component), but React elements can.
  icon: React.ReactNode;
  exact?: boolean;
}

export interface SidebarNavGroup {
  label?: string;
  items: SidebarNavItem[];
}

// Active-state matches by path prefix so nested routes (e.g. /admin/users/5)
// still highlight their section's nav item; `exact` opts a root-level item
// (e.g. "/admin" itself) out of that so it doesn't stay lit for every child
// route too. Hash-fragment items (dashboard's #continue-learning anchors)
// are in-page jump links, not distinct routes — never highlighted, since
// without scroll-spying a "current section" indicator would be a lie.
export function Sidebar({ groups, onNavigate }: { groups: SidebarNavGroup[]; onNavigate?: () => void }) {
  const pathname = usePathname();

  function isActive(item: SidebarNavItem): boolean {
    if (item.href.includes("#")) return false;
    if (item.exact) return pathname === item.href;
    return pathname === item.href || pathname.startsWith(item.href + "/");
  }

  return (
    <nav className="admin-nav">
      {groups.map((group, i) => (
        <div key={group.label ?? i}>
          {group.label && <p className="admin-nav-group-label">{group.label}</p>}
          {group.items.map((item) => (
            <Link key={item.href} href={item.href} className={isActive(item) ? "active" : undefined} onClick={onNavigate}>
              {item.icon}
              {item.label}
            </Link>
          ))}
        </div>
      ))}
    </nav>
  );
}
