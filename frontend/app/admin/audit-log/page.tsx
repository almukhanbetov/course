import Link from "next/link";
import type { Metadata } from "next";
import { redirect } from "next/navigation";
import { getSessionToken } from "@/lib/session";
import { adminListAuditLog, type AdminAuditLogEntry, type PageResult } from "@/lib/admin-api";

export const metadata: Metadata = {
  title: "Audit Log — Admin",
};

const ACTOR_ROLES = ["", "student", "instructor", "admin"];

// A compact single-line preview, not a full JSON dump — the point is to
// let a moderator recognize the entry at a glance; the raw metadata is
// still all there in the JSON response for anyone who needs the full
// object, just not spelled out in the table.
function metadataPreview(metadata?: Record<string, unknown>): string {
  if (!metadata || Object.keys(metadata).length === 0) return "—";
  const json = JSON.stringify(metadata);
  return json.length > 70 ? `${json.slice(0, 70)}…` : json;
}

// Read-only page: no client component, no server action, no per-row
// interactivity of any kind — internal/audit has no update/delete route
// to wire one to (Stage 25A3's own scope boundary), so unlike
// AdminReportsQueue this session has nothing that needs local state.
export default async function AdminAuditLogPage({
  searchParams,
}: {
  searchParams: Promise<{ page?: string; actor_role?: string; action?: string; entity_type?: string }>;
}) {
  const token = await getSessionToken();
  if (!token) {
    redirect("/login");
  }

  const { page: pageParam, actor_role: actorRole, action, entity_type: entityType } = await searchParams;
  const page = Number(pageParam ?? "1") || 1;

  let result: PageResult<AdminAuditLogEntry> | null = null;
  let loadError: string | null = null;
  try {
    result = await adminListAuditLog(token, { page, limit: 20, actor_role: actorRole, action, entity_type: entityType });
  } catch (err) {
    loadError = err instanceof Error ? err.message : "Unknown error";
  }

  const filterQuery = `${actorRole ? `&actor_role=${actorRole}` : ""}${action ? `&action=${action}` : ""}${
    entityType ? `&entity_type=${entityType}` : ""
  }`;

  return (
    <div>
      <div className="admin-header">
        <h1>Журнал действий</h1>
      </div>
      <p className="subtitle">
        Только для чтения: кто и когда выполнил значимое административное действие. Запись нельзя изменить или
        удалить из этого интерфейса.
      </p>

      {/* actor_role is a genuinely fixed small set (student/instructor/admin),
          so a <select> fits; action/entity_type are deliberately open-ended
          on the backend (Stage 25A1's own design decision — new values are
          added routinely as more call sites get audited), so free-text
          inputs are the honest match rather than a dropdown that would need
          updating every time a new action is added. */}
      <form className="admin-search" action="/admin/audit-log" method="get">
        <select name="actor_role" defaultValue={actorRole ?? ""}>
          {ACTOR_ROLES.map((r) => (
            <option key={r} value={r}>
              {r || "все роли"}
            </option>
          ))}
        </select>
        <input type="text" name="action" placeholder="действие (например, content_hidden)" defaultValue={action ?? ""} />
        <input
          type="text"
          name="entity_type"
          placeholder="тип сущности (например, question)"
          defaultValue={entityType ?? ""}
        />
        <button type="submit" className="btn-secondary">
          Filter
        </button>
      </form>

      {loadError ? (
        <p role="alert">Не удалось загрузить журнал: {loadError}</p>
      ) : result === null ? null : result.items.length === 0 ? (
        <div className="empty-state">
          <p>Записей не найдено.</p>
        </div>
      ) : (
        <>
          <div className="admin-table-wrapper">
            <table className="admin-table">
              <thead>
                <tr>
                  <th>Актор</th>
                  <th>Роль</th>
                  <th>Действие</th>
                  <th>Тип сущности</th>
                  <th>ID сущности</th>
                  <th>Метаданные</th>
                  <th>Когда</th>
                </tr>
              </thead>
              <tbody>
                {result.items.map((entry) => (
                  <tr key={entry.id}>
                    <td>{entry.actor_name?.trim() || "—"}</td>
                    <td>{entry.actor_role ?? "—"}</td>
                    <td>{entry.action}</td>
                    <td>{entry.entity_type}</td>
                    <td>{entry.entity_id ? <code>{entry.entity_id}</code> : "—"}</td>
                    <td>
                      <code>{metadataPreview(entry.metadata)}</code>
                    </td>
                    <td>{new Date(entry.created_at).toLocaleString("ru-RU")}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          <div className="admin-pagination">
            <span>
              Page {result.page} / {result.total_pages || 1} ({result.total} total)
            </span>
            {result.page > 1 && <Link href={`/admin/audit-log?page=${result.page - 1}${filterQuery}`}>← Prev</Link>}
            {result.page < result.total_pages && (
              <Link href={`/admin/audit-log?page=${result.page + 1}${filterQuery}`}>Next →</Link>
            )}
          </div>
        </>
      )}
    </div>
  );
}
