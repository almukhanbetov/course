import Link from "next/link";

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
}: {
  basePath: string;
  searchParams: SearchParams;
  page: number;
  totalPages: number;
}) {
  if (totalPages <= 1) return null;

  return (
    <nav className="pagination" aria-label="Пагинация">
      {page > 1 ? (
        <Link href={buildHref(basePath, searchParams, page - 1)} className="btn-secondary">
          ← Назад
        </Link>
      ) : (
        <span />
      )}
      <span>
        Страница {page} из {totalPages}
      </span>
      {page < totalPages ? (
        <Link href={buildHref(basePath, searchParams, page + 1)} className="btn-secondary">
          Вперёд →
        </Link>
      ) : (
        <span />
      )}
    </nav>
  );
}
