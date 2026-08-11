import Link from "next/link";
import type { Metadata } from "next";
import { redirect } from "next/navigation";
import { getSessionToken } from "@/lib/session";
import { adminListNotificationJobs } from "@/lib/admin-api";
import { retryNotificationJobAction } from "@/lib/admin-actions";
import { ConfirmButton } from "@/components/ConfirmButton";

export const metadata: Metadata = {
  title: "Notifications — Admin",
};

const STATUSES = ["", "pending", "processing", "completed", "failed"];
const CHANNELS = ["", "in_app", "email"];

export default async function AdminNotificationsPage({
  searchParams,
}: {
  searchParams: Promise<{ page?: string; status?: string; channel?: string; error?: string }>;
}) {
  const token = await getSessionToken();
  if (!token) {
    redirect("/login");
  }

  const { page: pageParam, status, channel, error } = await searchParams;
  const page = Number(pageParam ?? "1") || 1;

  const result = await adminListNotificationJobs(token, { page, limit: 20, status, channel });

  const query = (overrides: Record<string, string | number>) => {
    const params = new URLSearchParams();
    if (status) params.set("status", status);
    if (channel) params.set("channel", channel);
    Object.entries(overrides).forEach(([k, v]) => params.set(k, String(v)));
    return `/admin/notifications?${params.toString()}`;
  };

  return (
    <div>
      <div className="admin-header">
        <h1>Notification jobs</h1>
      </div>
      <p className="subtitle">
        The transactional outbox that drives both in-app notifications and email. Only a failed job can be retried —
        recipient and payload can never be changed.
      </p>

      {error && <p role="alert">{decodeURIComponent(error)}</p>}

      <form className="admin-search" action="/admin/notifications" method="get">
        <select name="status" defaultValue={status ?? ""}>
          {STATUSES.map((s) => (
            <option key={s} value={s}>
              {s || "all statuses"}
            </option>
          ))}
        </select>
        <select name="channel" defaultValue={channel ?? ""}>
          {CHANNELS.map((c) => (
            <option key={c} value={c}>
              {c || "all channels"}
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
              <th>Type</th>
              <th>Channel</th>
              <th>Status</th>
              <th>Attempts</th>
              <th>Last error</th>
              <th>Created</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {result.items.map((job) => (
              <tr key={job.id}>
                <td>{job.user_email}</td>
                <td>{job.type}</td>
                <td>
                  <span className="badge">{job.channel}</span>
                </td>
                <td>
                  <span className="badge">{job.status}</span>
                </td>
                <td>{job.attempts}</td>
                <td>{job.last_error ?? "—"}</td>
                <td>{new Date(job.created_at).toLocaleString("ru-RU")}</td>
                <td>
                  {job.status === "failed" && (
                    <form action={retryNotificationJobAction.bind(null, job.id)}>
                      <ConfirmButton className="btn-small" confirmMessage="Retry this notification job?">
                        Retry
                      </ConfirmButton>
                    </form>
                  )}
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
        {result.page > 1 && <Link href={query({ page: result.page - 1 })}>← Prev</Link>}
        {result.page < result.total_pages && <Link href={query({ page: result.page + 1 })}>Next →</Link>}
      </div>
    </div>
  );
}
