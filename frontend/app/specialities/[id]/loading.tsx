export default function Loading() {
  return (
    <main className="speciality-detail-shell">
      <div className="skeleton skeleton-line" style={{ width: "35%", height: "0.8rem" }} />

      <div className="skeleton-card" style={{ marginTop: "1.5rem" }}>
        <div className="skeleton skeleton-line" style={{ width: "60%", height: "2rem" }} />
        <div className="skeleton skeleton-line" style={{ width: "90%" }} />
        <div className="skeleton skeleton-line" style={{ width: "40%" }} />
      </div>

      <div className="skeleton skeleton-line" style={{ width: "150px", height: "1.75rem", marginTop: "3rem" }} />
      {Array.from({ length: 3 }).map((_, i) => (
        <div key={i} className="skeleton-card" style={{ marginTop: "1.25rem" }}>
          <div className="skeleton skeleton-line" style={{ width: "50%" }} />
          <div className="skeleton skeleton-line" style={{ width: "85%" }} />
          <div className="skeleton skeleton-line" style={{ width: "30%" }} />
        </div>
      ))}
    </main>
  );
}
