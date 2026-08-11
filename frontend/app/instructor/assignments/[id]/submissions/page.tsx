import Link from "next/link";
import { redirect } from "next/navigation";
import { getSessionToken } from "@/lib/session";
import { instructorListAssignmentSubmissions } from "@/lib/instructor-api";
import { SubmissionsTable } from "@/components/instructor/SubmissionsTable";

const STATUSES = ["", "submitted", "needs_revision", "approved"];
const STATUS_LABELS: Record<string, string> = {
  "": "Все статусы",
  submitted: "На проверке",
  needs_revision: "Требует доработки",
  approved: "Принято",
};

export default async function InstructorAssignmentSubmissionsPage({
  params,
  searchParams,
}: {
  params: Promise<{ id: string }>;
  searchParams: Promise<{ page?: string; status?: string }>;
}) {
  const { id } = await params;
  const { page: pageParam, status } = await searchParams;
  const page = Number(pageParam ?? "1") || 1;

  const token = await getSessionToken();
  if (!token) {
    redirect("/login");
  }

  const result = await instructorListAssignmentSubmissions(token, id, { page, limit: 20, status });

  return (
    <div>
      <h1>Решения по заданию</h1>

      <form className="admin-search" action={`/instructor/assignments/${id}/submissions`} method="get">
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

      <SubmissionsTable items={result.items} showCourse={false} />

      <div className="admin-pagination">
        <span>
          Страница {result.page} / {result.total_pages || 1} ({result.total} всего)
        </span>
        {result.page > 1 && (
          <Link href={`/instructor/assignments/${id}/submissions?page=${result.page - 1}${status ? `&status=${status}` : ""}`}>
            ← Назад
          </Link>
        )}
        {result.page < result.total_pages && (
          <Link href={`/instructor/assignments/${id}/submissions?page=${result.page + 1}${status ? `&status=${status}` : ""}`}>
            Вперёд →
          </Link>
        )}
      </div>
    </div>
  );
}
