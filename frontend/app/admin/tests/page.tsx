import Link from "next/link";
import type { Metadata } from "next";
import { redirect } from "next/navigation";
import { getSessionToken } from "@/lib/session";
import { adminListTests, adminListCourses } from "@/lib/admin-api";
import { TestForm } from "@/components/admin/TestForm";

export const metadata: Metadata = {
  title: "Tests — Admin",
};

export default async function AdminTestsPage() {
  const token = await getSessionToken();
  if (!token) {
    redirect("/login");
  }

  const [tests, courses] = await Promise.all([
    adminListTests(token),
    adminListCourses(token, { limit: 100 }),
  ]);

  return (
    <div>
      <h1>Tests</h1>

      <div className="admin-table-wrapper">
        <table className="admin-table">
          <thead>
            <tr>
              <th>Title</th>
              <th>Course</th>
              <th>Passing score</th>
              <th>Final</th>
              <th>Published</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {tests.map((test) => (
              <tr key={test.id}>
                <td>{test.title}</td>
                <td>{test.course_title || "—"}</td>
                <td>{test.passing_score}%</td>
                <td>{test.is_final ? "yes" : "no"}</td>
                <td>{test.published ? "yes" : "no"}</td>
                <td>
                  <Link href={`/admin/tests/${test.id}`} className="btn-small">
                    Manage
                  </Link>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="admin-card">
        <h2>New test</h2>
        <TestForm courses={courses.items} />
      </div>
    </div>
  );
}
