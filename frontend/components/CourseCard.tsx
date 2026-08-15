import Link from "next/link";
import type { Course } from "@/lib/api";
import { Rating } from "@/components/Rating";
import { WishlistButton } from "@/components/WishlistButton";
import { IconUser } from "@/components/shell/icons";
import { getCategoryVisual } from "@/lib/categoryVisuals";
import { getDictionary, localizeLevel } from "@/lib/i18n/dictionaries";
import { localizeDescription, localizeTitle } from "@/lib/i18n/localize";
import type { Locale } from "@/lib/i18n/locale";

// Wishlist is a sibling of the navigational Link, not nested inside it —
// a <button> inside an <a> is both invalid HTML and would double-fire
// navigation on click. `showWishlist` is false whenever the caller has no
// session (the public listing renders fine for anonymous visitors, just
// without the toggle — see CourseListing).
export function CourseCard({
  course,
  locale,
  showWishlist = false,
  initialInWishlist = false,
}: {
  course: Course;
  locale: Locale;
  showWishlist?: boolean;
  initialInWishlist?: boolean;
}) {
  const { Icon: PlaceholderIcon, accent } = getCategoryVisual(course.category_slug);
  const dict = getDictionary(locale);

  return (
    <div className="course-card">
      {showWishlist && <WishlistButton courseId={course.id} initialInWishlist={initialInWishlist} />}
      <Link href={`/courses/${course.id}`} className="course-card-link">
        <div className="course-card-image">
          {course.image_url ? (
            <img src={course.image_url} alt="" />
          ) : (
            <div className="course-card-placeholder" style={{ "--placeholder-accent": accent } as React.CSSProperties}>
              <PlaceholderIcon size={96} className="course-card-placeholder-watermark" />
              <PlaceholderIcon size={26} className="course-card-placeholder-icon" />
            </div>
          )}
        </div>

        {/* Priority order, not DOM-arbitrary: access/payment status reads
            first (it's the thing that actually gates the student), then
            category, then level. */}
        <div className="course-card-badges">
          <span className={`badge ${course.access_type === "free" ? "badge-free" : "badge-premium"}`}>
            {course.access_type === "free" ? dict.courses.accessFree : dict.courses.accessSubscription}
          </span>
          {course.category_name && <span className="badge badge-category">{course.category_name}</span>}
          <span className="badge">{localizeLevel(course.level, dict)}</span>
        </div>

        <h2>{localizeTitle(course, locale)}</h2>
        <p className="course-card-description">{localizeDescription(course, locale)}</p>

        <div className="course-card-footer">
          <Rating average={course.rating_average} count={course.rating_count} locale={locale} />
          {course.instructor_name && (
            <span className="course-card-instructor">
              <IconUser size={14} />
              {course.instructor_name}
            </span>
          )}
        </div>
      </Link>
    </div>
  );
}
