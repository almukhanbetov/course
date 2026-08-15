import Link from "next/link";
import type { Course } from "@/lib/api";
import { Rating } from "@/components/Rating";
import { WishlistButton } from "@/components/WishlistButton";
import { IconUser } from "@/components/shell/icons";
import { getCategoryVisual } from "@/lib/categoryVisuals";

// Wishlist is a sibling of the navigational Link, not nested inside it —
// a <button> inside an <a> is both invalid HTML and would double-fire
// navigation on click. `showWishlist` is false whenever the caller has no
// session (the public listing renders fine for anonymous visitors, just
// without the toggle — see CourseListing).
export function CourseCard({
  course,
  showWishlist = false,
  initialInWishlist = false,
}: {
  course: Course;
  showWishlist?: boolean;
  initialInWishlist?: boolean;
}) {
  const { Icon: PlaceholderIcon, accent } = getCategoryVisual(course.category_slug);

  return (
    <div className="course-card">
      {showWishlist && <WishlistButton courseId={course.id} initialInWishlist={initialInWishlist} />}
      <Link href={`/courses/${course.id}`} className="course-card-link">
        <div className="course-card-image">
          {course.image_url ? (
            <img src={course.image_url} alt="" />
          ) : (
            <div className="course-card-placeholder" style={{ "--placeholder-accent": accent } as React.CSSProperties}>
              <PlaceholderIcon size={24} />
            </div>
          )}
        </div>

        {/* Priority order, not DOM-arbitrary: access/payment status reads
            first (it's the thing that actually gates the student), then
            category, then level. */}
        <div className="course-card-badges">
          <span className={`badge ${course.access_type === "free" ? "badge-free" : "badge-premium"}`}>
            {course.access_type === "free" ? "Бесплатный" : "По подписке"}
          </span>
          {course.category_name && <span className="badge badge-category">{course.category_name}</span>}
          <span className="badge">{course.level}</span>
        </div>

        <h2>{course.title}</h2>
        <p className="course-card-description">{course.description}</p>

        <div className="course-card-footer">
          <Rating average={course.rating_average} count={course.rating_count} />
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
