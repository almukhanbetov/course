"use client";

import { useState } from "react";
import { Sidebar, type SidebarNavGroup } from "./Sidebar";
import { IconClose, IconMenu } from "./icons";

interface SidebarShellProps {
  brand: string;
  brandIcon?: React.ReactNode;
  sidebarClassName?: string;
  brandClassName?: string;
  groups: SidebarNavGroup[];
  // Pre-rendered on the server by the caller (name/role badge + a
  // server-action logout <form>) — a Server Component layout can pass
  // already-rendered JSX like this into a Client Component just fine, so
  // the logout action never needs to cross the client boundary itself.
  userSlot: React.ReactNode;
  children: React.ReactNode;
}

// Self-contained: owns its own mobile-drawer open/close state, independent
// of the global TopBar (NavBar). Used identically by /dashboard, /admin and
// /instructor layouts with different brand/groups — the only three places
// in the app that need a left sidebar.
export function SidebarShell({
  brand,
  brandIcon,
  sidebarClassName,
  brandClassName,
  groups,
  userSlot,
  children,
}: SidebarShellProps) {
  const [open, setOpen] = useState(false);

  return (
    <div className="admin-shell">
      <aside className={["admin-sidebar", sidebarClassName, open ? "open" : ""].filter(Boolean).join(" ")}>
        <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
          <p className={["admin-sidebar-brand", brandClassName].filter(Boolean).join(" ")}>
            {brandIcon}
            {brand}
          </p>
          <button type="button" className="sidebar-close-btn" onClick={() => setOpen(false)} aria-label="Закрыть меню">
            <IconClose size={16} />
          </button>
        </div>
        <Sidebar groups={groups} onNavigate={() => setOpen(false)} />
      </aside>

      <div className={["sidebar-overlay", open ? "open" : ""].filter(Boolean).join(" ")} onClick={() => setOpen(false)} />

      <div className="admin-main">
        <header className="admin-topbar">
          <div>
            <button type="button" className="navbar-menu-btn" onClick={() => setOpen(true)} aria-label="Открыть меню">
              <IconMenu size={18} />
            </button>
          </div>
          <div className="admin-topbar-user">{userSlot}</div>
        </header>
        <main className="admin-content">{children}</main>
      </div>
    </div>
  );
}
