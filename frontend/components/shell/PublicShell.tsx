import Link from "next/link";
import { getSpecialities } from "@/lib/api";
import { SidebarAccordion } from "@/components/shell/SidebarAccordion";

// Shared wide layout for public marketing/catalog pages (/, /courses,
// /specialities): a left sidebar of specialities plus a main content column.
// Authenticated areas (dashboard/instructor/admin) use their own SidebarShell.
export async function PublicShell({ children }: { children: React.ReactNode }) {
  let specialities: Awaited<ReturnType<typeof getSpecialities>> = [];
  try {
    specialities = await getSpecialities();
  } catch {
    specialities = [];
  }

  return (
    <main className="public-shell">
      <aside className="public-sidebar">
        <SidebarAccordion title="IT-специальности">
          <nav className="public-sidebar-nav">
            {specialities.map((s) => (
              <Link key={s.id} href={`/specialities/${s.slug}`}>
                {s.title}
              </Link>
            ))}
            {specialities.length === 0 && <p className="public-sidebar-empty">Специальностей пока нет.</p>}
          </nav>
          <Link href="/specialities" className="public-sidebar-all">
            Все специальности →
          </Link>
        </SidebarAccordion>
      </aside>

      <div className="public-main">{children}</div>
    </main>
  );
}
