import Link from "next/link";
import type { Metadata } from "next";
import { redirect } from "next/navigation";
import { getSessionToken } from "@/lib/session";
import {
  adminGetSystemHealth,
  AdminSystemHealthError,
  type AdminSystemHealth,
  type AdminSystemHealthComponent,
} from "@/lib/admin-api";

export const metadata: Metadata = {
  title: "System Health — Admin",
};

function ComponentRow({ label, component }: { label: string; component: AdminSystemHealthComponent }) {
  return (
    <tr>
      <td>{label}</td>
      <td>
        <span className={`badge badge-status-${component.status}`}>{component.status}</span>
      </td>
      <td>{component.detail ?? "—"}</td>
    </tr>
  );
}

// Read-only, server-rendered, no auto-refresh — same shape as every other
// admin page (audit-log, notifications): fetch once per navigation/reload,
// no client component, no polling. The backend check itself is already
// cheap and timeout-bounded (Stage 29A3); this page just displays one
// snapshot of it.
export default async function AdminSystemHealthPage() {
  const token = await getSessionToken();
  if (!token) {
    redirect("/login");
  }

  let health: AdminSystemHealth | null = null;
  let errorMessage: string | null = null;
  let sessionExpired = false;

  try {
    health = await adminGetSystemHealth(token);
  } catch (err) {
    if (err instanceof AdminSystemHealthError && err.status === 401) {
      sessionExpired = true;
    }
    errorMessage = err instanceof Error ? err.message : "Unknown error";
  }

  // Three states, always rendered the same consistent way: "ok"/"degraded"
  // come straight from the backend; "unavailable" is this page's own
  // client-side concept for "couldn't even reach/parse the check" — a
  // fetch failure, an expired session, anything that never produced a real
  // DeepStatus body at all.
  const overallStatus: "ok" | "degraded" | "unavailable" = health ? health.status : "unavailable";

  return (
    <div>
      <div className="admin-header">
        <h1>System Health</h1>
      </div>
      <p className="subtitle">
        Live status of the production stack&apos;s core dependencies — database, object storage, and the
        notification queue. The same deep check the deploy pipeline itself relies on (Stage 29A3); nothing here goes
        beyond what an authenticated admin is meant to see.
      </p>

      <div className="module">
        <h3>
          Overall status: <span className={`badge badge-status-${overallStatus}`}>{overallStatus}</span>
        </h3>
        {sessionExpired ? (
          <p role="alert">
            Your session appears to have expired. <Link href="/login">Sign in again</Link>.
          </p>
        ) : errorMessage ? (
          <p role="alert">Could not reach the system health check: {errorMessage}</p>
        ) : null}
      </div>

      {health && (
        <div className="admin-table-wrapper">
          <table className="admin-table">
            <thead>
              <tr>
                <th>Component</th>
                <th>Status</th>
                <th>Detail</th>
              </tr>
            </thead>
            <tbody>
              <ComponentRow label="Database (PostgreSQL)" component={health.database} />
              <ComponentRow label="Object storage (MinIO)" component={health.storage} />
              <ComponentRow label="Notification queue" component={health.notifications} />
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
