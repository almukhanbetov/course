import Link from "next/link";
import { reasonLabel, type Recommendation } from "@/lib/api";
import { Rating } from "@/components/Rating";

// Shared by the /dashboard "Рекомендуем вам" grid and the /courses/[id]
// "Похожие курсы" grid — same card shape (Recommendation), same fields
// (item 17: title/image/category/rating/access type/reason). Only the
// first reason is shown as the headline explanation, matching the spec's
// own examples ("Потому что вы изучаете Go") rather than listing every
// contributing factor.
export function RecommendationCard({ rec }: { rec: Recommendation }) {
  const primaryReason = rec.reasons[0];

  return (
    <Link href={`/courses/${rec.course_id}`} className="course-card recommendation-card">
      <div className="course-card-image">
        {rec.image_url ? <img src={rec.image_url} alt="" /> : "Нет изображения"}
      </div>
      <h3>{rec.title}</h3>
      {rec.category_name && <span className="badge badge-category">{rec.category_name}</span>}{" "}
      <span className={`badge ${rec.access_type === "free" ? "badge-free" : "badge-premium"}`}>
        {rec.access_type === "free" ? "Бесплатный" : "По подписке"}
      </span>
      <div className="course-card-rating">
        <Rating average={rec.rating_average} count={rec.rating_count} />
      </div>
      {primaryReason && <p className="recommendation-reason">{reasonLabel(primaryReason, rec.category_name)}</p>}
    </Link>
  );
}
