import Link from "next/link";
import type { Metadata } from "next";
import { redirect } from "next/navigation";
import { getSessionToken } from "@/lib/session";
import { adminListCategories } from "@/lib/admin-api";

export const metadata: Metadata = {
  title: "Categories — Admin",
};

export default async function AdminCategoriesPage() {
  const token = await getSessionToken();
  if (!token) {
    redirect("/login");
  }

  const categories = await adminListCategories(token);

  return (
    <div>
      <div className="admin-header">
        <h1>Categories</h1>
        <Link href="/admin/categories/new" className="btn-primary">
          New category
        </Link>
      </div>

      <div className="admin-table-wrapper">
        <table className="admin-table">
          <thead>
            <tr>
              <th>#</th>
              <th>Name</th>
              <th>Slug</th>
              <th>Active</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {categories.map((category) => (
              <tr key={category.id}>
                <td>{category.position}</td>
                <td>{category.name}</td>
                <td>{category.slug}</td>
                <td>{category.active ? "yes" : "no"}</td>
                <td>
                  <Link href={`/admin/categories/${category.id}`} className="btn-small">
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
