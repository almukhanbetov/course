import Link from "next/link";
import { SubmissionStatusBadge } from "@/components/SubmissionStatusBadge";
import type { InstructorSubmissionRow } from "@/lib/instructor-api";

// Shared by the global inbox (/instructor/submissions) and the
// per-assignment list (/instructor/assignments/:id/submissions) — same
// row shape either way (see InstructorSubmissionRow).
export function SubmissionsTable({ items, showCourse }: { items: InstructorSubmissionRow[]; showCourse: boolean }) {
  if (items.length === 0) {
    return <p>Ничего не найдено.</p>;
  }

  return (
    <div className="admin-table-wrapper">
      <table className="admin-table">
        <thead>
          <tr>
            <th>Студент</th>
            {showCourse && <th>Курс</th>}
            <th>Задание</th>
            <th>Отправлено</th>
            <th>Статус</th>
            <th>Балл</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          {items.map((row) => (
            <tr key={row.submission_id}>
              <td>{row.student_name}</td>
              {showCourse && <td>{row.course_title}</td>}
              <td>{row.assignment_title}</td>
              <td>{row.submitted_at ? new Date(row.submitted_at).toLocaleString("ru-RU") : "—"}</td>
              <td>
                <SubmissionStatusBadge status={row.status} />
              </td>
              <td>{row.score ?? "—"}</td>
              <td>
                <Link href={`/instructor/submissions/${row.submission_id}`} className="btn-small">
                  Проверить
                </Link>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
