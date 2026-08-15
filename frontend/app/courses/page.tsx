import type { Metadata } from "next";
import { CourseListing, parseCourseFilters } from "@/components/CourseListing";
import { CourseFilters } from "@/components/CourseFilters";
import { SidebarAccordion } from "@/components/shell/SidebarAccordion";
import { getCategories } from "@/lib/api";

export const metadata: Metadata = {
  title: "Курсы — LMS Platform",
  description: "Каталог курсов по программированию, базам данных, DevOps и frontend-разработке с поиском, фильтрами и отзывами студентов.",
  alternates: {
    // Every filter combination (?q=...&level=...&sort=...) renders the same
    // page shell with different data — none of them should be indexed as a
    // separate URL, so every variant declares this bare path as canonical.
    canonical: "/courses",
  },
};

export default async function CoursesPage({
  searchParams,
}: {
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const params = await searchParams;
  const categories = await getCategories().catch(() => []);
  const { q, category, level, access_type, sort } = parseCourseFilters(params);

  return (
    <main className="public-shell">
      <aside className="public-sidebar">
        <SidebarAccordion title="Фильтры">
          <CourseFilters
            categories={categories}
            basePath="/courses"
            current={{ q, category, level, access_type, sort }}
            showCategoryFilter
            layout="sidebar"
          />
        </SidebarAccordion>
      </aside>

      <div className="public-main">
        <div className="catalog-intro">
          <p className="section-eyebrow">Каталог</p>
          <h1>Курсы</h1>
          <p className="subtitle">Каталог курсов по программированию, базам данных, DevOps и frontend-разработке.</p>
        </div>
        <CourseListing searchParams={params} basePath="/courses" showFilters={false} />
      </div>
    </main>
  );
}
