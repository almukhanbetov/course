import Link from "next/link";
import { getCategories, getCourses, getMyWishlistCourseIds } from "@/lib/api";
import { getSessionToken } from "@/lib/session";
import { CourseCard } from "@/components/CourseCard";
import { CourseFilters } from "@/components/CourseFilters";
import { CoursePagination } from "@/components/CoursePagination";
import { ErrorState } from "@/components/shell/ErrorState";
import { IconCourses } from "@/components/shell/icons";
import { getDictionary } from "@/lib/i18n/dictionaries";
import type { Locale } from "@/lib/i18n/locale";

export type SearchParams = Record<string, string | string[] | undefined>;

export function first(value: string | string[] | undefined): string {
  return (Array.isArray(value) ? value[0] : value) ?? "";
}

// Shared by CourseListing and any caller that renders CourseFilters itself
// (e.g. /courses' left sidebar) — keeps the searchParams -> form-values
// parsing in exactly one place.
export function parseCourseFilters(searchParams: SearchParams, fixedCategory?: string) {
  return {
    q: first(searchParams.q),
    level: first(searchParams.level),
    access_type: first(searchParams.access_type),
    sort: first(searchParams.sort),
    category: fixedCategory ?? first(searchParams.category),
    page: Number(first(searchParams.page)) || 1,
  };
}

interface Props {
  searchParams: SearchParams;
  basePath: string;
  locale: Locale;
  // Set on /categories/[slug] — fixes the category and hides the category
  // selector from the filter bar (the route itself already picks it).
  fixedCategory?: string;
  // /courses renders CourseFilters itself, in a left sidebar, sharing the
  // same searchParams/basePath — set this so CourseListing doesn't render
  // a second copy of the form.
  showFilters?: boolean;
}

// Shared by /courses and /categories/[slug] so the search/filter/sort/
// pagination logic exists in exactly one place.
export async function CourseListing({ searchParams, basePath, locale, fixedCategory, showFilters = true }: Props) {
  const { q, level, access_type: accessType, sort, category, page } = parseCourseFilters(searchParams, fixedCategory);
  const dict = getDictionary(locale).courses;

  let categories;
  let result;
  let loadError = false;
  try {
    [categories, result] = await Promise.all([
      getCategories(),
      getCourses({ q, category, level, access_type: accessType, sort, page, limit: 12 }),
    ]);
  } catch {
    loadError = true;
  }

  if (loadError || !result || !categories) {
    return <ErrorState message={dict.errorLoad} />;
  }

  // Wishlist is a second, authenticated-only enrichment call (item 3) —
  // the public getCourses() above never carries or requires a session.
  // Anonymous visitors simply get showWishlist=false on every card.
  const token = await getSessionToken();
  const wishlistIds = token ? await getMyWishlistCourseIds(token).catch(() => []) : [];
  const wishlistSet = new Set(wishlistIds);

  return (
    <>
      {showFilters && (
        <CourseFilters
          categories={categories}
          basePath={basePath}
          current={{ q, category, level, access_type: accessType, sort }}
          showCategoryFilter={!fixedCategory}
          locale={locale}
        />
      )}

      {result.items.length === 0 ? (
        <div className="empty-state">
          <IconCourses size={26} />
          <p className="empty-state-title">{dict.emptyTitle}</p>
          <p className="empty-state-text">{dict.emptyText}</p>
          <Link href={basePath} className="btn-secondary">
            {dict.resetFilters}
          </Link>
        </div>
      ) : (
        <>
          <p className="catalog-result-count">{dict.resultCount(result.total)}</p>
          <div className="course-grid">
            {result.items.map((course) => (
              <CourseCard
                key={course.id}
                course={course}
                locale={locale}
                showWishlist={Boolean(token)}
                initialInWishlist={wishlistSet.has(course.id)}
              />
            ))}
          </div>
          <CoursePagination
            basePath={basePath}
            searchParams={searchParams}
            page={result.page}
            totalPages={result.total_pages}
            locale={locale}
          />
        </>
      )}
    </>
  );
}
