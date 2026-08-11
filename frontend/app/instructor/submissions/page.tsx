import Link from "next/link";
import type { Metadata } from "next";
import { redirect } from "next/navigation";
import { getSessionToken } from "@/lib/session";
import { instructorListSubmissions } from "@/lib/instructor-api";
import { SubmissionsTable } from "@/components/instructor/SubmissionsTable";

export const metadata: Metadata = {
  title: "Submissions Inbox — Instructor",
};

const STATUSES = ["", "submitted", "needs_revision", "approved"];
const STATUS_LABELS: Record<string, string> = {
  "": "Все статусы",
  submitted: "На проверке",
  needs_revision: "Требует доработки",
  approved: "Принято",
};

// The global inbox across every course this instructor owns (item 21) —
// more useful than making the instructor open each course individually.
export default async function InstructorSubmissionsInboxPage({
  searchParams,
}: {
  searchParams: Promise<{ page?: string; status?: string }>;
}) {
  const token = await getSessionToken();
  if (!token) {
    redirect("/login");
  }

  const { page: pageParam, status } = await searchParams;
  const page = Number(pageParam ?? "1") || 1;

  const result = await instructorListSubmissions(token, { page, limit: 20, status });

  return (
    <div>
      <h1>Входящие решения</h1>
      <p className="subtitle">Все отправленные домашние задания по всем вашим курсам.</p>

      <form className="admin-search" action="/instructor/submissions" method="get">
        <select name="status" defaultValue={status ?? ""}>
          {STATUSES.map((s) => (
            <option key={s} value={s}>
              {STATUS_LABELS[s]}
            </option>
          ))}
        </select>
        <button type="submit" className="btn-secondary">
          Фильтр
        </button>
      </form>

      <SubmissionsTable items={result.items} showCourse />

      <div className="admin-pagination">
        <span>
          Страница {result.page} / {result.total_pages || 1} ({result.total} всего)
        </span>
        {result.page > 1 && (
          <Link href={`/instructor/submissions?page=${result.page - 1}${status ? `&status=${status}` : ""}`}>← Назад</Link>
        )}
        {result.page < result.total_pages && (
          <Link href={`/instructor/submissions?page=${result.page + 1}${status ? `&status=${status}` : ""}`}>Вперёд →</Link>
        )}
      </div>
    </div>
  );
}
