import Link from "next/link";
import { getSpecialities } from "@/lib/api";
import { SidebarAccordion } from "@/components/shell/SidebarAccordion";
import { IconGraduationCap } from "@/components/shell/icons";
import { getDictionary } from "@/lib/i18n/dictionaries";
import { localizeTitle } from "@/lib/i18n/localize";
import type { Locale } from "@/lib/i18n/locale";

// Shared wide layout for public marketing/catalog pages (/, /courses,
// /specialities): a left sidebar of specialities plus a main content column.
// Authenticated areas (dashboard/instructor/admin) use their own SidebarShell.
export async function PublicShell({ children, locale }: { children: React.ReactNode; locale: Locale }) {
  let specialities: Awaited<ReturnType<typeof getSpecialities>> = [];
  try {
    specialities = await getSpecialities();
  } catch {
    specialities = [];
  }
  const dict = getDictionary(locale).sidebar;

  return (
    <main className="public-shell">
      <aside className="public-sidebar">
        <SidebarAccordion title={dict.title}>
          <nav className="public-sidebar-nav">
            {specialities.map((s) => (
              <Link key={s.id} href={`/specialities/${s.slug}`}>
                <IconGraduationCap size={14} />
                <span>{localizeTitle(s, locale)}</span>
              </Link>
            ))}
            {specialities.length === 0 && <p className="public-sidebar-empty">{dict.emptyText}</p>}
          </nav>
          <Link href="/specialities" className="public-sidebar-all">
            {dict.allLink}
          </Link>
        </SidebarAccordion>
      </aside>

      <div className="public-main">{children}</div>
    </main>
  );
}
