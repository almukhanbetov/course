import Link from "next/link";
import { getSpecialities } from "@/lib/api";

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
        <p className="public-sidebar-title">IT-специальности</p>
        <nav className="public-sidebar-nav">
          {specialities.map((s) => (
            <Link key={s.id} href={`/specialities/${s.slug}`}>
              {s.title}
            </Link>
          ))}
        </nav>
        <Link href="/specialities" className="public-sidebar-all">
          Все специальности →
        </Link>
      </aside>

      <div className="public-main">{children}</div>
    </main>
  );
}
