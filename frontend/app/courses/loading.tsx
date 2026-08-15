export default function Loading() {
  return (
    <main className="public-shell">
      <aside className="public-sidebar">
        <div className="skeleton skeleton-line" style={{ width: "50%", height: "0.7rem" }} />
        <div className="skeleton skeleton-line" style={{ width: "90%", marginTop: "1.5rem" }} />
        <div className="skeleton skeleton-line" style={{ width: "75%" }} />
        <div className="skeleton skeleton-line" style={{ width: "85%" }} />
        <div className="skeleton skeleton-line" style={{ width: "65%" }} />
      </aside>
      <div className="public-main">
        <div className="skeleton skeleton-line" style={{ width: "180px", height: "2rem" }} />
        <div className="skeleton skeleton-line" style={{ width: "60%", marginTop: "0.75rem" }} />
        <div className="course-grid" style={{ marginTop: "1.5rem" }}>
          {Array.from({ length: 6 }).map((_, i) => (
            <div key={i} className="skeleton-card">
              <div className="skeleton skeleton-image" />
              <div className="skeleton skeleton-line" style={{ width: "40%" }} />
              <div className="skeleton skeleton-line" style={{ width: "85%" }} />
              <div className="skeleton skeleton-line" style={{ width: "55%" }} />
            </div>
          ))}
        </div>
      </div>
    </main>
  );
}
