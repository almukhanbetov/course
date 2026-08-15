"use client";

import { useState } from "react";
import { IconClose, IconMenu } from "@/components/shell/icons";

// Collapses the pre-rendered nav links (built server-side by NavBar, since
// they depend on the session) into a toggled dropdown below ~900px — see
// .navbar-menu-btn/.navbar-links.open in globals.css. Mirrors the same
// pre-rendered-children-across-the-client-boundary trick as SidebarShell.
export function NavBarMobileMenu({ children }: { children: React.ReactNode }) {
  const [open, setOpen] = useState(false);

  return (
    <>
      <button
        type="button"
        className="navbar-menu-btn"
        onClick={() => setOpen((v) => !v)}
        aria-label={open ? "Закрыть меню" : "Открыть меню"}
        aria-expanded={open}
      >
        {open ? <IconClose size={18} /> : <IconMenu size={18} />}
      </button>
      <div className={open ? "navbar-links open" : "navbar-links"}>{children}</div>
    </>
  );
}
