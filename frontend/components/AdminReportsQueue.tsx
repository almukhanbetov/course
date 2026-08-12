"use client";

import { useState, useTransition } from "react";
import type { AdminContentReport } from "@/lib/admin-api";
import { updateReportStatusAction } from "@/lib/admin-actions";

type Status = AdminContentReport["status"];

const STATUS_LABEL: Record<Status, string> = {
  open: "Открыта",
  resolved: "Решена",
  dismissed: "Отклонена",
};

// The two transitions offered from each current status — never the status
// the row is already in (no "mark open as open" no-op button). Covers
// every pairwise transition across the three statuses without a redundant
// "you're already here" control.
const TRANSITIONS: Record<Status, { target: Status; label: string }[]> = {
  open: [
    { target: "resolved", label: "Решить" },
    { target: "dismissed", label: "Отклонить" },
  ],
  resolved: [
    { target: "open", label: "Открыть заново" },
    { target: "dismissed", label: "Отклонить" },
  ],
  dismissed: [
    { target: "open", label: "Открыть заново" },
    { target: "resolved", label: "Решить" },
  ],
};

// Owns per-row status-update state so a successful PATCH updates just that
// row's badge/actions in place — no full page reload (Stage 24B1). Never
// removes a row or touches the reported content itself; only the status
// column changes, matching the backend's own Stage 24A3 scope boundary.
export function AdminReportsQueue({ initialReports }: { initialReports: AdminContentReport[] }) {
  const [reports, setReports] = useState(initialReports);
  const [pendingId, setPendingId] = useState<string | null>(null);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [, startTransition] = useTransition();

  function handleStatusChange(reportId: string, target: Status) {
    setErrors((prev) => ({ ...prev, [reportId]: "" }));
    setPendingId(reportId);
    startTransition(async () => {
      const result = await updateReportStatusAction(reportId, target);
      setPendingId(null);
      if (!result.ok) {
        setErrors((prev) => ({ ...prev, [reportId]: result.error ?? "Не удалось изменить статус жалобы" }));
        return;
      }
      setReports((prev) => prev.map((r) => (r.id === reportId ? { ...r, status: target } : r)));
    });
  }

  if (reports.length === 0) {
    return (
      <div className="empty-state">
        <p>Жалоб не найдено.</p>
      </div>
    );
  }

  return (
    <div className="admin-table-wrapper">
      <table className="admin-table">
        <thead>
          <tr>
            <th>Автор жалобы</th>
            <th>Тип</th>
            <th>ID контента</th>
            <th>Причина</th>
            <th>Статус</th>
            <th>Создана</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          {reports.map((report) => {
            const isPending = pendingId === report.id;
            return (
              <tr key={report.id}>
                <td>{report.reporter_name}</td>
                <td>{report.content_type}</td>
                <td>
                  <code>{report.content_id}</code>
                </td>
                <td>{report.reason}</td>
                <td>
                  <span className="badge">{STATUS_LABEL[report.status]}</span>
                </td>
                <td>{new Date(report.created_at).toLocaleString("ru-RU")}</td>
                <td>
                  <div className="qa-moderation-actions">
                    {TRANSITIONS[report.status].map(({ target, label }) => (
                      <button
                        key={target}
                        type="button"
                        className="btn-small"
                        onClick={() => handleStatusChange(report.id, target)}
                        disabled={isPending}
                      >
                        {isPending ? "..." : label}
                      </button>
                    ))}
                  </div>
                  {errors[report.id] && <p role="alert">{errors[report.id]}</p>}
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
