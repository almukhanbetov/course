import Link from "next/link";
import type { Metadata } from "next";
import { redirect } from "next/navigation";
import { getSessionToken } from "@/lib/session";
import { adminListCourses } from "@/lib/admin-api";

export const metadata: Metadata = {
  title: "Courses — Admin",
};

export default async function AdminCoursesPage({
  searchParams,
}: {
  searchParams: Promise<{ page?: string; search?: string }>;
}) {
  const token = await getSessionToken();
  if (!token) {
    redirect("/login");
  }

  const { page: pageParam, search } = await searchParams;
  const page = Number(pageParam ?? "1") || 1;

  const result = await adminListCourses(token, { page, limit: 20, search });

  return (
    <div>
      <div className="admin-header">
        <h1>Courses</h1>
        <Link href="/admin/courses/new" className="btn-primary">
          New course
        </Link>
      </div>

      <form className="admin-search" action="/admin/courses" method="get">
        <input type="text" name="search" placeholder="Search by title or slug" defaultValue={search} />
        <button type="submit" className="btn-secondary">
          Search
        </button>
      </form>

      <div className="admin-table-wrapper">
        <table className="admin-table">
          <thead>
            <tr>
              <th>Title</th>
              <th>Slug</th>
              <th>Level</th>
              <th>Published</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {result.items.map((course) => (
              <tr key={course.id}>
                <td>{course.title}</td>
                <td>{course.slug}</td>
                <td>{course.level}</td>
                <td>{course.published ? "yes" : "no"}</td>
                <td>
                  <Link href={`/admin/courses/${course.id}`} className="btn-small">
                    Manage
                  </Link>
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
        {result.page > 1 && (
          <Link href={`/admin/courses?page=${result.page - 1}${search ? `&search=${search}` : ""}`}>← Prev</Link>
        )}
        {result.page < result.total_pages && (
          <Link href={`/admin/courses?page=${result.page + 1}${search ? `&search=${search}` : ""}`}>Next →</Link>
        )}
      </div>
    </div>
  );
}
