import Link from "next/link";
import { reasonLabel, type Recommendation } from "@/lib/api";
import { Rating } from "@/components/Rating";
import { getDictionary } from "@/lib/i18n/dictionaries";
import { DEFAULT_LOCALE, type Locale } from "@/lib/i18n/locale";

// Optional feedback controls (Stage 23B1) — only the personalized
// /dashboard grid passes this prop (via PersonalizedRecommendations);
// /courses/[id]'s public "Похожие курсы" grid renders this same component
// with no `feedback` prop at all, so it gets zero buttons and zero behavior
// change, satisfying "no feedback controls on public Similar Courses"
// without any special-casing at the call site.
export interface RecommendationFeedbackProps {
  pending: boolean;
  error?: string;
  onDismiss: () => void;
  onNotInterested: () => void;
}

// Shared by the /dashboard "Рекомендуем вам" grid and the /courses/[id]
// "Похожие курсы" grid — same card shape (Recommendation), same fields
// (item 17: title/image/category/rating/access type/reason). Only the
// first reason is shown as the headline explanation, matching the spec's
// own examples ("Потому что вы изучаете Go") rather than listing every
// contributing factor.
//
// Structure: an outer <div class="course-card"> with the navigational
// <Link> as one child and (when present) the feedback buttons as a
// sibling — the same pattern CourseCard.tsx already established for
// WishlistButton, and for the same reason its own comment gives: a
// <button> nested inside an <a> is invalid HTML and would double-fire
// navigation on click. This is a structural fix reused from an existing
// pattern, not a redesign — the rendered content/fields are unchanged.
export function RecommendationCard({
  rec,
  feedback,
  locale = DEFAULT_LOCALE,
}: {
  rec: Recommendation;
  feedback?: RecommendationFeedbackProps;
  locale?: Locale;
}) {
  const primaryReason = rec.reasons[0];
  const dict = getDictionary(locale);

  return (
    <div className="course-card recommendation-card">
      {feedback && (
        <div className="recommendation-feedback-actions">
          <button type="button" className="btn-small" onClick={feedback.onDismiss} disabled={feedback.pending}>
            {feedback.pending ? "..." : "Скрыть"}
          </button>
          <button type="button" className="btn-small" onClick={feedback.onNotInterested} disabled={feedback.pending}>
            {feedback.pending ? "..." : "Не интересно"}
          </button>
        </div>
      )}

      <Link href={`/courses/${rec.course_id}`} className="course-card-link">
        <div className="course-card-image">
          {rec.image_url ? <img src={rec.image_url} alt="" /> : dict.common.noImage}
        </div>
        <h3>{rec.title}</h3>
        {rec.category_name && <span className="badge badge-category">{rec.category_name}</span>}{" "}
        <span className={`badge ${rec.access_type === "free" ? "badge-free" : "badge-premium"}`}>
          {rec.access_type === "free" ? dict.common.free : dict.common.subscription}
        </span>
        <div className="course-card-rating">
          <Rating average={rec.rating_average} count={rec.rating_count} locale={locale} />
        </div>
        {primaryReason && <p className="recommendation-reason">{reasonLabel(primaryReason, rec.category_name)}</p>}
      </Link>

      {feedback?.error && (
        <p className="recommendation-feedback-error" role="alert">
          {feedback.error}
        </p>
      )}
    </div>
  );
}
