import Link from "next/link";
import type { Metadata } from "next";
import { redirect } from "next/navigation";
import { getSessionToken } from "@/lib/session";
import { adminListSubscriptions } from "@/lib/admin-api";

export const metadata: Metadata = {
  title: "Subscriptions — Admin",
};

const STATUSES = ["", "pending", "active", "expired", "canceled"];

export default async function AdminSubscriptionsPage({
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

  const result = await adminListSubscriptions(token, { page, limit: 20, status });

  return (
    <div>
      <div className="admin-header">
        <h1>Subscriptions</h1>
      </div>
      <p className="subtitle">Read-only — a subscription only ever becomes active via a confirmed payment.</p>

      <form className="admin-search" action="/admin/subscriptions" method="get">
        <select name="status" defaultValue={status ?? ""}>
          {STATUSES.map((s) => (
            <option key={s} value={s}>
              {s || "all statuses"}
            </option>
          ))}
        </select>
        <button type="submit" className="btn-secondary">
          Filter
        </button>
      </form>

      <div className="admin-table-wrapper">
        <table className="admin-table">
          <thead>
            <tr>
              <th>User</th>
              <th>Plan</th>
              <th>Status</th>
              <th>Starts</th>
              <th>Expires</th>
            </tr>
          </thead>
          <tbody>
            {result.items.map((sub) => (
              <tr key={sub.id}>
                <td>
                  {sub.user_name} ({sub.user_email})
                </td>
                <td>{sub.plan_name}</td>
                <td>
                  <span className="badge">{sub.status}</span>
                </td>
                <td>{sub.starts_at ? new Date(sub.starts_at).toLocaleDateString("ru-RU") : "—"}</td>
                <td>{sub.expires_at ? new Date(sub.expires_at).toLocaleDateString("ru-RU") : "—"}</td>
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
          <Link href={`/admin/subscriptions?page=${result.page - 1}${status ? `&status=${status}` : ""}`}>← Prev</Link>
        )}
        {result.page < result.total_pages && (
          <Link href={`/admin/subscriptions?page=${result.page + 1}${status ? `&status=${status}` : ""}`}>Next →</Link>
        )}
      </div>
    </div>
  );
}
