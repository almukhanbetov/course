export default function Loading() {
  return (
    <main className="course-detail-shell">
      <div className="skeleton skeleton-line" style={{ width: "40%", height: "0.8rem" }} />

      <div className="course-detail-layout">
        <div className="course-detail-main">
          <div className="skeleton-card" style={{ marginTop: "1.5rem" }}>
            <div className="skeleton skeleton-line" style={{ width: "30%" }} />
            <div className="skeleton skeleton-line" style={{ width: "70%", height: "2rem" }} />
            <div className="skeleton skeleton-line" style={{ width: "50%" }} />
            <div className="skeleton skeleton-line" style={{ width: "95%" }} />
            <div className="skeleton skeleton-line" style={{ width: "80%" }} />
          </div>

          <div className="skeleton skeleton-line" style={{ width: "200px", height: "1.75rem", marginTop: "3rem" }} />
          {Array.from({ length: 2 }).map((_, i) => (
            <div key={i} className="skeleton-card" style={{ marginTop: "1.25rem" }}>
              <div className="skeleton skeleton-line" style={{ width: "40%" }} />
              <div className="skeleton skeleton-line" style={{ width: "90%" }} />
              <div className="skeleton skeleton-line" style={{ width: "75%" }} />
            </div>
          ))}
        </div>

        <aside className="course-detail-aside">
          <div className="skeleton-card">
            <div className="skeleton skeleton-line" style={{ width: "100%", height: "2.5rem" }} />
          </div>
        </aside>
      </div>
    </main>
  );
}
