import Link from "next/link";
import type { Metadata } from "next";
import { redirect } from "next/navigation";
import { getSessionToken } from "@/lib/session";
import { adminListUsers } from "@/lib/admin-api";

export const metadata: Metadata = {
  title: "Users — Admin",
};

export default async function AdminUsersPage({
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

  const result = await adminListUsers(token, { page, limit: 20, search });

  return (
    <div>
      <div className="admin-header">
        <h1>Users</h1>
      </div>

      <form className="admin-search" action="/admin/users" method="get">
        <input type="text" name="search" placeholder="Search by email or name" defaultValue={search} />
        <button type="submit" className="btn-secondary">
          Search
        </button>
      </form>

      <div className="admin-table-wrapper">
        <table className="admin-table">
          <thead>
            <tr>
              <th>Email</th>
              <th>Name</th>
              <th>Role</th>
              <th>Active</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {result.items.map((user) => (
              <tr key={user.id}>
                <td>{user.email}</td>
                <td>
                  {user.first_name} {user.last_name}
                </td>
                <td>
                  <span className="badge">{user.role}</span>
                </td>
                <td>{user.active ? "yes" : "no"}</td>
                <td>
                  <Link href={`/admin/users/${user.id}`} className="btn-small">
                    Edit
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
          <Link href={`/admin/users?page=${result.page - 1}${search ? `&search=${search}` : ""}`}>← Prev</Link>
        )}
        {result.page < result.total_pages && (
          <Link href={`/admin/users?page=${result.page + 1}${search ? `&search=${search}` : ""}`}>Next →</Link>
        )}
      </div>
    </div>
  );
}
