import Link from "next/link";
import type { Metadata } from "next";
import { redirect } from "next/navigation";
import { getSessionToken } from "@/lib/session";
import { adminListReports, type PageResult, type AdminContentReport } from "@/lib/admin-api";
import { AdminReportsQueue } from "@/components/AdminReportsQueue";

export const metadata: Metadata = {
  title: "Reports — Admin",
};

const STATUSES = ["", "open", "resolved", "dismissed"];
const CONTENT_TYPES = ["", "question", "answer", "review"];

export default async function AdminReportsPage({
  searchParams,
}: {
  searchParams: Promise<{ page?: string; status?: string; content_type?: string }>;
}) {
  const token = await getSessionToken();
  if (!token) {
    redirect("/login");
  }

  const { page: pageParam, status, content_type: contentType } = await searchParams;
  const page = Number(pageParam ?? "1") || 1;

  let result: PageResult<AdminContentReport> | null = null;
  let loadError: string | null = null;
  try {
    result = await adminListReports(token, { page, limit: 20, status, content_type: contentType });
  } catch (err) {
    loadError = err instanceof Error ? err.message : "Unknown error";
  }

  const filterQuery = `${status ? `&status=${status}` : ""}${contentType ? `&content_type=${contentType}` : ""}`;

  return (
    <div>
      <div className="admin-header">
        <h1>Жалобы</h1>
      </div>
      <p className="subtitle">
        Жалобы на вопросы, ответы и отзывы студентов. Изменение статуса не скрывает и не удаляет сам контент —
        для этого используйте модерацию Q&amp;A или отзывов.
      </p>

      <form className="admin-search" action="/admin/reports" method="get">
        <select name="status" defaultValue={status ?? ""}>
          {STATUSES.map((s) => (
            <option key={s} value={s}>
              {s || "все статусы"}
            </option>
          ))}
        </select>
        <select name="content_type" defaultValue={contentType ?? ""}>
          {CONTENT_TYPES.map((t) => (
            <option key={t} value={t}>
              {t || "все типы"}
            </option>
          ))}
        </select>
        <button type="submit" className="btn-secondary">
          Filter
        </button>
      </form>

      {loadError ? (
        <p role="alert">Не удалось загрузить жалобы: {loadError}</p>
      ) : result === null ? null : (
        <>
          <AdminReportsQueue initialReports={result.items} />

          <div className="admin-pagination">
            <span>
              Page {result.page} / {result.total_pages || 1} ({result.total} total)
            </span>
            {result.page > 1 && (
              <Link href={`/admin/reports?page=${result.page - 1}${filterQuery}`}>← Prev</Link>
            )}
            {result.page < result.total_pages && (
              <Link href={`/admin/reports?page=${result.page + 1}${filterQuery}`}>Next →</Link>
            )}
          </div>
        </>
      )}
    </div>
  );
}
