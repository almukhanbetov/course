import type { Metadata } from "next";
import { CourseListing, parseCourseFilters } from "@/components/CourseListing";
import { CourseFilters } from "@/components/CourseFilters";
import { SidebarAccordion } from "@/components/shell/SidebarAccordion";
import { getCategories } from "@/lib/api";
import { getDictionary } from "@/lib/i18n/dictionaries";
import { getLocale } from "@/lib/i18n/getLocale";

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
  const locale = await getLocale();
  const dict = getDictionary(locale).courses;
  const categories = await getCategories().catch(() => []);
  const { q, category, level, access_type, sort } = parseCourseFilters(params);

  return (
    <main className="public-shell">
      <aside className="public-sidebar">
        <SidebarAccordion title={dict.filtersTitle}>
          <CourseFilters
            categories={categories}
            basePath="/courses"
            current={{ q, category, level, access_type, sort }}
            showCategoryFilter
            layout="sidebar"
            locale={locale}
          />
        </SidebarAccordion>
      </aside>

      <div className="public-main">
        <div className="catalog-intro">
          <p className="section-eyebrow">{dict.eyebrow}</p>
          <h1>{dict.title}</h1>
          <p className="subtitle">{dict.subtitle}</p>
        </div>
        <CourseListing searchParams={params} basePath="/courses" showFilters={false} locale={locale} />
      </div>
    </main>
  );
}
