import Link from "next/link";
import type { Metadata } from "next";
import { redirect } from "next/navigation";
import { getSessionToken } from "@/lib/session";
import { adminListPayments } from "@/lib/admin-api";

export const metadata: Metadata = {
  title: "Payments — Admin",
};

const STATUSES = ["", "pending", "paid", "failed", "refunded"];

export default async function AdminPaymentsPage({
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

  const result = await adminListPayments(token, { page, limit: 20, status });

  return (
    <div>
      <div className="admin-header">
        <h1>Payments</h1>
      </div>
      <p className="subtitle">
        Read-only — payments only move to “paid” through the payment provider confirmation flow, never a manual
        admin action.
      </p>

      <form className="admin-search" action="/admin/payments" method="get">
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
              <th>Amount</th>
              <th>Provider</th>
              <th>Status</th>
              <th>Paid at</th>
            </tr>
          </thead>
          <tbody>
            {result.items.map((payment) => (
              <tr key={payment.id}>
                <td>
                  {payment.user_name} ({payment.user_email})
                </td>
                <td>
                  {(payment.amount / 100).toLocaleString("ru-RU", { minimumFractionDigits: 2 })} {payment.currency}
                </td>
                <td>{payment.provider}</td>
                <td>
                  <span className="badge">{payment.status}</span>
                </td>
                <td>{payment.paid_at ? new Date(payment.paid_at).toLocaleString("ru-RU") : "—"}</td>
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
          <Link href={`/admin/payments?page=${result.page - 1}${status ? `&status=${status}` : ""}`}>← Prev</Link>
        )}
        {result.page < result.total_pages && (
          <Link href={`/admin/payments?page=${result.page + 1}${status ? `&status=${status}` : ""}`}>Next →</Link>
        )}
      </div>
    </div>
  );
}
