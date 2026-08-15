import Link from "next/link";
import { getDictionary } from "@/lib/i18n/dictionaries";
import type { Locale } from "@/lib/i18n/locale";

type SearchParams = Record<string, string | string[] | undefined>;

function buildHref(basePath: string, searchParams: SearchParams, page: number): string {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(searchParams)) {
    if (key === "page") continue;
    const v = Array.isArray(value) ? value[0] : value;
    if (v) params.set(key, v);
  }
  if (page > 1) params.set("page", String(page));

  const qs = params.toString();
  return qs ? `${basePath}?${qs}` : basePath;
}

export function CoursePagination({
  basePath,
  searchParams,
  page,
  totalPages,
  locale,
}: {
  basePath: string;
  searchParams: SearchParams;
  page: number;
  totalPages: number;
  locale: Locale;
}) {
  if (totalPages <= 1) return null;
  const dict = getDictionary(locale).courses;

  return (
    <nav className="pagination" aria-label={dict.paginationLabel}>
      {page > 1 ? (
        <Link href={buildHref(basePath, searchParams, page - 1)} className="btn-secondary">
          {dict.paginationPrev}
        </Link>
      ) : (
        <span />
      )}
      <span>{dict.paginationPage(page, totalPages)}</span>
      {page < totalPages ? (
        <Link href={buildHref(basePath, searchParams, page + 1)} className="btn-secondary">
          {dict.paginationNext}
        </Link>
      ) : (
        <span />
      )}
    </nav>
  );
}
