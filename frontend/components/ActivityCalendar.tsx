import type { ActivityDayCount } from "@/lib/api";

// GitHub-contribution-style calendar, restyled with this project's own
// design system (see .activity-calendar* in globals.css) rather than
// copied visually — five shading levels driven by a data-level attribute.
function levelFor(count: number): 0 | 1 | 2 | 3 | 4 {
  if (count <= 0) return 0;
  if (count === 1) return 1;
  if (count <= 3) return 2;
  if (count <= 5) return 3;
  return 4;
}

export function ActivityCalendar({ days, rangeDays = 91 }: { days: ActivityDayCount[]; rangeDays?: number }) {
  const countByDate = new Map(days.map((d) => [d.date, d.activity_count]));

  const today = new Date();
  today.setHours(0, 0, 0, 0);
  const cells: { date: string; count: number }[] = [];
  for (let i = rangeDays - 1; i >= 0; i--) {
    const d = new Date(today);
    d.setDate(d.getDate() - i);
    const iso = d.toISOString().slice(0, 10);
    cells.push({ date: iso, count: countByDate.get(iso) ?? 0 });
  }

  // Pad the front so the grid starts on a Sunday-aligned week column,
  // matching the 7-row (Sun..Sat) layout .activity-calendar expects.
  const firstDow = new Date(cells[0].date + "T00:00:00").getDay();
  const padded = [...Array(firstDow).fill(null), ...cells];

  return (
    <div className="activity-calendar" role="img" aria-label={`Календарь активности за последние ${rangeDays} дней`}>
      {padded.map((cell, i) =>
        cell === null ? (
          <div key={`pad-${i}`} className="activity-calendar-day" style={{ visibility: "hidden" }} />
        ) : (
          <div
            key={cell.date}
            className="activity-calendar-day"
            data-level={levelFor(cell.count)}
            title={`${cell.date}: ${cell.count} событий`}
          />
        ),
      )}
    </div>
  );
}
