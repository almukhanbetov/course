import Link from "next/link";
import type { Metadata } from "next";
import { redirect } from "next/navigation";
import { getSessionToken } from "@/lib/session";
import { adminListCertificates } from "@/lib/admin-api";

export const metadata: Metadata = {
  title: "Certificates — Admin",
};

// Read-only by design: a certificate is only ever issued by the
// completion-driven business logic (see internal/certificates), never
// created directly by an admin — so there is no "new certificate" form here.
export default async function AdminCertificatesPage({
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

  const result = await adminListCertificates(token, { page, limit: 20, search });

  return (
    <div>
      <h1>Certificates</h1>
      <p className="subtitle">Read-only — certificates are issued automatically when a course is completed.</p>

      <form className="admin-search" action="/admin/certificates" method="get">
        <input type="text" name="search" placeholder="Search by certificate number" defaultValue={search} />
        <button type="submit" className="btn-secondary">
          Search
        </button>
      </form>

      <div className="admin-table-wrapper">
        <table className="admin-table">
          <thead>
            <tr>
              <th>Certificate number</th>
              <th>Student</th>
              <th>Course</th>
              <th>Issued</th>
            </tr>
          </thead>
          <tbody>
            {result.items.map((cert) => (
              <tr key={cert.id}>
                <td>{cert.certificate_number}</td>
                <td>
                  {cert.student_name} ({cert.student_email})
                </td>
                <td>{cert.course_title}</td>
                <td>{new Date(cert.issued_at).toLocaleDateString("ru-RU")}</td>
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
          <Link href={`/admin/certificates?page=${result.page - 1}${search ? `&search=${search}` : ""}`}>
            ← Prev
          </Link>
        )}
        {result.page < result.total_pages && (
          <Link href={`/admin/certificates?page=${result.page + 1}${search ? `&search=${search}` : ""}`}>
            Next →
          </Link>
        )}
      </div>
    </div>
  );
}
