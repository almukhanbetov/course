import Link from "next/link";
import type { Metadata } from "next";
import { redirect } from "next/navigation";
import { getSessionToken } from "@/lib/session";
import { adminGetRevenueAnalytics, type AdminRevenueAnalytics } from "@/lib/admin-api";
import { formatPrice } from "@/lib/api";

export const metadata: Metadata = {
  title: "Revenue Analytics — Admin",
};

export default async function AdminAnalyticsPage({
  searchParams,
}: {
  searchParams: Promise<{ from?: string; to?: string }>;
}) {
  const token = await getSessionToken();
  if (!token) {
    redirect("/login");
  }

  const { from, to } = await searchParams;

  let analytics: AdminRevenueAnalytics | null = null;
  let errorState: "unauthorized" | "forbidden" | "error" | null = null;
  let errorMessage = "";

  try {
    analytics = await adminGetRevenueAnalytics(token, { from, to });
  } catch (err) {
    if (err instanceof Error && err.message === "UNAUTHORIZED") {
      errorState = "unauthorized";
    } else if (err instanceof Error && err.message === "FORBIDDEN") {
      errorState = "forbidden";
    } else {
      errorState = "error";
      errorMessage = err instanceof Error ? err.message : "Unknown error";
    }
  }

  if (errorState === "unauthorized") {
    return (
      <div>
        <div className="admin-header">
          <h1>Revenue Analytics</h1>
        </div>
        <p role="alert">
          Your session has expired. <Link href="/login">Log in again</Link>.
        </p>
      </div>
    );
  }

  if (errorState === "forbidden") {
    return (
      <div>
        <div className="admin-header">
          <h1>Revenue Analytics</h1>
        </div>
        <p role="alert">You do not have permission to view revenue analytics.</p>
      </div>
    );
  }

  if (errorState === "error" || !analytics) {
    return (
      <div>
        <div className="admin-header">
          <h1>Revenue Analytics</h1>
        </div>
        <p role="alert">Failed to load revenue analytics: {errorMessage || "unknown error"}</p>
      </div>
    );
  }

  // The date inputs (and the "showing" label) prefer the raw query params
  // the admin actually submitted; when a param is absent they fall back to
  // the backend's own echoed default (analytics.from/to) rather than
  // reconstructing it here, since `to` gets a +1-day adjustment server-side
  // only when explicitly provided (see handler_admin.go) — recomputing that
  // locally would risk an off-by-one on the displayed end date.
  const displayFrom = from || analytics.from.slice(0, 10);
  const displayTo = to || analytics.to.slice(0, 10);

  return (
    <div>
      <div className="admin-header">
        <h1>Revenue Analytics</h1>
      </div>
      <p className="subtitle">
        Every figure below is grouped by currency and never combined into a single total — a plan can be priced in a
        different currency than another, so summing across currencies would be meaningless. Payments are processed
        through this project&apos;s mock payment provider (see <code>STAGE19_PROGRESS.md</code>).
      </p>

      <form className="admin-search" action="/admin/analytics" method="get">
        <label>
          From
          <input type="date" name="from" defaultValue={displayFrom} />
        </label>
        <label>
          To
          <input type="date" name="to" defaultValue={displayTo} />
        </label>
        <button type="submit" className="btn-secondary">
          Apply
        </button>
      </form>

      <p className="subtitle">
        Showing {displayFrom} — {displayTo}. Active subscriptions, subscription breakdown and MRR are always
        point-in-time (as of now), independent of this range; revenue, new and canceled subscriptions are scoped to
        it.
      </p>

      {analytics.by_currency.length === 0 ? (
        <div className="empty-state">
          <p>No subscriptions or payments recorded yet — analytics will appear here once the first one exists.</p>
        </div>
      ) : (
        analytics.by_currency.map((cr) => (
          <section key={cr.currency} className="mt-3">
            <h2>{cr.currency}</h2>
            <div className="stat-grid">
              <div className="stat-tile">
                <span className="value">{formatPrice({ price_amount: cr.paid_amount, currency: cr.currency })}</span>
                <span className="label">
                  Revenue ({cr.paid_count} payment{cr.paid_count === 1 ? "" : "s"})
                </span>
              </div>
              <div className="stat-tile">
                <span className="value">{formatPrice({ price_amount: cr.mrr, currency: cr.currency })}</span>
                <span className="label">MRR (normalized to 30 days)</span>
              </div>
              <div className="stat-tile">
                <span className="value">{cr.active_subscriptions}</span>
                <span className="label">Active subscriptions</span>
              </div>
              <div className="stat-tile">
                <span className="value">{cr.new_subscriptions}</span>
                <span className="label">New subscriptions</span>
              </div>
              <div className="stat-tile">
                <span className="value">{cr.canceled_subscriptions}</span>
                <span className="label">Canceled subscriptions</span>
              </div>
              <div className="stat-tile">
                <span className="value">{formatPrice({ price_amount: cr.refunded_amount, currency: cr.currency })}</span>
                <span className="label">
                  Refunded ({cr.refunded_count} payment{cr.refunded_count === 1 ? "" : "s"})
                </span>
              </div>
            </div>
          </section>
        ))
      )}

      <p className="subtitle mt-3">
        Per-plan revenue breakdown is not available yet — the Stage 19A backend deliberately scoped this endpoint to
        currency-only aggregation. See <code>STAGE19_PROGRESS.md</code> for details.
      </p>
    </div>
  );
}
