// Native <details>/<summary> disclosure — collapsible on mobile with zero
// client JS, and CSS-neutralized back into a plain static block on desktop
// (see .public-sidebar-accordion in globals.css). Always rendered `open` so
// SSR output is fully expanded before any CSS/JS runs.
export function SidebarAccordion({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <details className="public-sidebar-accordion" open>
      <summary className="public-sidebar-title">{title}</summary>
      <div className="public-sidebar-body">{children}</div>
    </details>
  );
}
