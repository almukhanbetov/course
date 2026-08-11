import Link from "next/link";
import { getCategories, getCourses, getMyWishlistCourseIds } from "@/lib/api";
import { getSessionToken } from "@/lib/session";
import { CourseCard } from "@/components/CourseCard";
import { CourseFilters } from "@/components/CourseFilters";
import { CoursePagination } from "@/components/CoursePagination";

type SearchParams = Record<string, string | string[] | undefined>;

function first(value: string | string[] | undefined): string {
  return (Array.isArray(value) ? value[0] : value) ?? "";
}

interface Props {
  searchParams: SearchParams;
  basePath: string;
  // Set on /categories/[slug] — fixes the category and hides the category
  // selector from the filter bar (the route itself already picks it).
  fixedCategory?: string;
}

// Shared by /courses and /categories/[slug] so the search/filter/sort/
// pagination logic exists in exactly one place.
export async function CourseListing({ searchParams, basePath, fixedCategory }: Props) {
  const q = first(searchParams.q);
  const level = first(searchParams.level);
  const accessType = first(searchParams.access_type);
  const sort = first(searchParams.sort);
  const category = fixedCategory ?? first(searchParams.category);
  const page = Number(first(searchParams.page)) || 1;

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
    return <p role="alert">Не удалось загрузить курсы. Попробуйте позже.</p>;
  }

  // Wishlist is a second, authenticated-only enrichment call (item 3) —
  // the public getCourses() above never carries or requires a session.
  // Anonymous visitors simply get showWishlist=false on every card.
  const token = await getSessionToken();
  const wishlistIds = token ? await getMyWishlistCourseIds(token).catch(() => []) : [];
  const wishlistSet = new Set(wishlistIds);

  return (
    <>
      <CourseFilters
        categories={categories}
        basePath={basePath}
        current={{ q, category, level, access_type: accessType, sort }}
        showCategoryFilter={!fixedCategory}
      />

      {result.items.length === 0 ? (
        <div className="empty-state">
          <p>Курсы не найдены.</p>
          <Link href={basePath} className="btn-secondary">
            Сбросить фильтры
          </Link>
        </div>
      ) : (
        <>
          <div className="course-grid">
            {result.items.map((course) => (
              <CourseCard
                key={course.id}
                course={course}
                showWishlist={Boolean(token)}
                initialInWishlist={wishlistSet.has(course.id)}
              />
            ))}
          </div>
          <CoursePagination basePath={basePath} searchParams={searchParams} page={result.page} totalPages={result.total_pages} />
        </>
      )}
    </>
  );
}
