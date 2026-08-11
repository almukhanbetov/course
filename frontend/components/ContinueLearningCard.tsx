import Link from "next/link";
import type { ContinueLearningItem } from "@/lib/api";

// CTA links straight into the next lesson the backend already computed
// server-side (Stage 18 item 7: "ведёт прямо на next_lesson") — never a
// frontend guess. The rare edge case where next_lesson_id is absent (every
// published lesson individually done, course-level completion still
// pending e.g. on a final test) falls back to the course page itself.
export function ContinueLearningCard({ item }: { item: ContinueLearningItem }) {
  const href = item.next_lesson_id ? `/learn/${item.course_id}/${item.next_lesson_id}` : `/courses/${item.course_id}`;

  return (
    <div className="course-card continue-learning-card">
      <div className="course-card-image">
        {item.image_url ? <img src={item.image_url} alt="" /> : "Нет изображения"}
      </div>
      <h3>{item.title}</h3>
      <div className="progress-bar">
        <div className="progress-bar-fill" style={{ width: `${item.progress_percent}%` }} />
      </div>
      <p className="my-course-meta">{item.progress_percent}% пройдено</p>
      {item.next_lesson_title && <p className="my-course-meta">Далее: {item.next_lesson_title}</p>}
      <Link href={href} className="btn-primary mt-3">
        Продолжить
      </Link>
    </div>
  );
}
