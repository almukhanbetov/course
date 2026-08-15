export default function Loading() {
  return (
    <main className="public-shell">
      <aside className="public-sidebar">
        <div className="skeleton skeleton-line" style={{ width: "50%", height: "0.7rem" }} />
        <div className="skeleton skeleton-line" style={{ width: "90%", marginTop: "1.5rem" }} />
        <div className="skeleton skeleton-line" style={{ width: "75%" }} />
      </aside>
      <div className="public-main">
        <div className="skeleton skeleton-line" style={{ width: "220px", height: "2rem" }} />
        <div className="skeleton skeleton-line" style={{ width: "55%", marginTop: "0.75rem" }} />
        <div className="speciality-grid" style={{ marginTop: "1.5rem" }}>
          {Array.from({ length: 3 }).map((_, i) => (
            <div key={i} className="skeleton-card">
              <div className="skeleton" style={{ width: 46, height: 46, borderRadius: "var(--radius-sm)", marginBottom: "1rem" }} />
              <div className="skeleton skeleton-line" style={{ width: "70%" }} />
              <div className="skeleton skeleton-line" style={{ width: "90%" }} />
              <div className="skeleton skeleton-line" style={{ width: "50%" }} />
            </div>
          ))}
        </div>
      </div>
    </main>
  );
}
