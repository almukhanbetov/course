import Link from "next/link";
import type { Metadata } from "next";
import { redirect } from "next/navigation";
import { getSessionToken } from "@/lib/session";
import { adminListSpecialities } from "@/lib/admin-api";

export const metadata: Metadata = {
  title: "Specialities — Admin",
};

export default async function AdminSpecialitiesPage() {
  const token = await getSessionToken();
  if (!token) {
    redirect("/login");
  }

  const specialities = await adminListSpecialities(token);

  return (
    <div>
      <div className="admin-header">
        <h1>Specialities</h1>
        <Link href="/admin/specialities/new" className="btn-primary">
          New speciality
        </Link>
      </div>

      <div className="admin-table-wrapper">
        <table className="admin-table">
          <thead>
            <tr>
              <th>Title</th>
              <th>Slug</th>
              <th>Published</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {specialities.map((speciality) => (
              <tr key={speciality.id}>
                <td>{speciality.title}</td>
                <td>{speciality.slug}</td>
                <td>{speciality.published ? "yes" : "no"}</td>
                <td>
                  <Link href={`/admin/specialities/${speciality.id}`} className="btn-small">
                    Manage
                  </Link>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
