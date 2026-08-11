import Link from "next/link";
import type { Metadata } from "next";
import { redirect } from "next/navigation";
import { getSessionToken } from "@/lib/session";
import { adminListCourseSubmissions } from "@/lib/admin-api";
import { approveCourseSubmissionAction } from "@/lib/admin-actions";
import { RejectCourseForm } from "@/components/admin/RejectCourseForm";
import { ConfirmButton } from "@/components/ConfirmButton";

export const metadata: Metadata = {
  title: "Course Submissions — Admin",
};

export default async function AdminCourseSubmissionsPage({
  searchParams,
}: {
  searchParams: Promise<{ page?: string; error?: string }>;
}) {
  const token = await getSessionToken();
  if (!token) {
    redirect("/login");
  }

  const { page: pageParam, error } = await searchParams;
  const page = Number(pageParam ?? "1") || 1;

  const result = await adminListCourseSubmissions(token, { page, limit: 20 });

  return (
    <div>
      <div className="admin-header">
        <h1>Course Submissions</h1>
      </div>
      <p className="subtitle">
        Instructor-authored courses waiting for moderation. Not to be confused with student course reviews — see
        Reviews for those.
      </p>

      {error && <p role="alert">{decodeURIComponent(error)}</p>}

      {result.items.length === 0 && <p>Nothing pending review.</p>}

      <div className="admin-table-wrapper">
        <table className="admin-table">
          <thead>
            <tr>
              <th>Course</th>
              <th>Instructor</th>
              <th>Submitted</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {result.items.map((course) => (
              <tr key={course.id}>
                <td>{course.title}</td>
                <td>{course.instructor_name ?? "—"}</td>
                <td>{new Date(course.updated_at).toLocaleString("ru-RU")}</td>
                <td>
                  <div className="admin-inline-actions">
                    <Link href={`/admin/courses/${course.id}`} className="btn-small">
                      View
                    </Link>
                    <form action={approveCourseSubmissionAction.bind(null, course.id)}>
                      <ConfirmButton className="btn-primary" confirmMessage="Publish this course?">
                        Approve
                      </ConfirmButton>
                    </form>
                    <RejectCourseForm courseId={course.id} />
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="admin-pagination">
        <span>
          Page {result.page} / {result.total_pages || 1} ({result.total} total)
        </span>
        {result.page > 1 && <Link href={`/admin/course-submissions?page=${result.page - 1}`}>← Prev</Link>}
        {result.page < result.total_pages && <Link href={`/admin/course-submissions?page=${result.page + 1}`}>Next →</Link>}
      </div>
    </div>
  );
}
