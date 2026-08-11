import Link from "next/link";
import type { Course } from "@/lib/api";
import { Rating } from "@/components/Rating";
import { WishlistButton } from "@/components/WishlistButton";

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
  return (
    <div className="course-card">
      {showWishlist && <WishlistButton courseId={course.id} initialInWishlist={initialInWishlist} />}
      <Link href={`/courses/${course.id}`} className="course-card-link">
        <div className="course-card-image">
          {course.image_url ? <img src={course.image_url} alt="" /> : "Нет изображения"}
        </div>
        <h2>{course.title}</h2>
        <p className="course-card-description">{course.description}</p>
        {course.category_name && <span className="badge badge-category">{course.category_name}</span>}{" "}
        <span className="badge">{course.level}</span>{" "}
        <span className={`badge ${course.access_type === "free" ? "badge-free" : "badge-premium"}`}>
          {course.access_type === "free" ? "Бесплатный" : "По подписке"}
        </span>
        <div className="course-card-rating">
          <Rating average={course.rating_average} count={course.rating_count} />
        </div>
      </Link>
    </div>
  );
}
