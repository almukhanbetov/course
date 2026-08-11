import Link from "next/link";
import { notFound } from "next/navigation";
import {
  getCourse,
  getCourseReviews,
  getMyCourseDetail,
  getMyReview,
  getMySubscription,
  getMyWishlistCourseIds,
  getSimilarCourses,
} from "@/lib/api";
import { getSessionToken } from "@/lib/session";
import { enrollAction } from "@/lib/actions";
import { Rating } from "@/components/Rating";
import { ReviewForm } from "@/components/ReviewForm";
import { WishlistButton } from "@/components/WishlistButton";
import { RecommendationCard } from "@/components/RecommendationCard";

export default async function CourseDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;

  let course: Awaited<ReturnType<typeof getCourse>>;
  let error: string | null = null;

  try {
    course = await getCourse(id);
  } catch (err) {
    error = err instanceof Error ? err.message : "Unknown error";
    course = null;
  }

  if (error) {
    return (
      <main>
        <p role="alert">Не удалось загрузить курс: {error}</p>
      </main>
    );
  }

  if (!course) {
    notFound();
  }

  const token = await getSessionToken();
  const myCourse = token ? await getMyCourseDetail(token, id) : null;
  const mySubscription = token ? await getMySubscription(token) : null;
  const reviews = await getCourseReviews(id).catch(() => null);
  const myReview = token ? await getMyReview(token, id) : null;
  const wishlistIds = token ? await getMyWishlistCourseIds(token).catch(() => []) : [];
  const similarCourses = await getSimilarCourses(id).catch(() => []);

  const isPremium = course.access_type === "subscription";
  const hasAccess = !isPremium || Boolean(mySubscription?.active);

  // Mirrors the backend eligibility rule exactly (enrolled AND (course
  // completed OR at least one completed lesson)) — myCourse is null when
  // not enrolled, so this is false in that case too.
  const canReview = Boolean(myCourse && (myCourse.completed || myCourse.completed_lessons > 0));

  return (
    <main>
      <Link href="/courses">← Все курсы</Link>

      <div className="course-hero">
        <div className="course-hero-badges">
          {course.category_name && course.category_slug && (
            <Link href={`/categories/${course.category_slug}`} className="badge badge-category">
              {course.category_name}
            </Link>
          )}
          <span className="badge">{course.level}</span>
          <span className={`badge ${isPremium ? "badge-premium" : "badge-free"}`}>
            {isPremium ? "По подписке" : "Бесплатный"}
          </span>
        </div>

        <h1>{course.title}</h1>

        <div className="rating-summary">
          <span className="rating-average">{course.rating_average.toFixed(1)}</span>
          <Rating average={course.rating_average} count={course.rating_count} />
        </div>

        <p className="course-hero-description">{course.description}</p>

        <div className="enroll-cta">
          {!token && (
            <Link href="/login" className="nav-link">
              Войдите, чтобы начать обучение
            </Link>
          )}
          {token && myCourse && (
            <Link href="/dashboard/courses" className="btn-primary">
              Продолжить обучение ({myCourse.progress_percent}%)
            </Link>
          )}
          {token && !myCourse && !hasAccess && (
            <Link href="/pricing" className="btn-primary">
              Оформить подписку
            </Link>
          )}
          {token && !myCourse && hasAccess && (
            <form action={enrollAction.bind(null, id)}>
              <button type="submit" className="btn-primary">
                Начать обучение
              </button>
            </form>
          )}
          {/* Wishlist stays available even once enrolled/completed — only
              successful enrollment auto-clears it (Stage 18 item 20), the
              student can still add/remove freely otherwise. */}
          {token && (
            <span className="wishlist-inline">
              <WishlistButton courseId={id} initialInWishlist={wishlistIds.includes(id)} />
            </span>
          )}
        </div>
      </div>

      <h2>Программа курса</h2>
      {course.modules.length === 0 && <p>Модули пока не добавлены.</p>}

      {course.modules.map((module) => (
        <section key={module.id} className="module">
          <h3>{module.title}</h3>
          <ul className="lesson-list">
            {module.lessons.map((lesson) => (
              <li key={lesson.id}>
                {lesson.title}
                {lesson.is_free && <span className="badge badge-free">бесплатно</span>}
              </li>
            ))}
          </ul>
        </section>
      ))}

      <h2>Отзывы</h2>

      {canReview && <ReviewForm courseId={id} myReview={myReview} />}
      {token && myCourse && !canReview && (
        <p className="subtitle">Оставить отзыв можно после прохождения хотя бы одного урока курса.</p>
      )}

      {!reviews || reviews.items.length === 0 ? (
        <p>Отзывов пока нет.</p>
      ) : (
        <ul className="review-list">
          {reviews.items.map((review) => (
            <li key={review.id} className="review-item">
              <div className="review-item-header">
                <span className="review-author">{review.display_name}</span>
                <span className="review-date">{new Date(review.created_at).toLocaleDateString("ru-RU")}</span>
              </div>
              <span className="rating">{"★".repeat(review.rating)}{"☆".repeat(5 - review.rating)}</span>
              {review.review_text && <p className="review-text">{review.review_text}</p>}
            </li>
          ))}
        </ul>
      )}

      {similarCourses.length > 0 && (
        <>
          <h2 className="mt-3">Похожие курсы</h2>
          <div className="course-grid">
            {similarCourses.map((rec) => (
              <RecommendationCard key={rec.course_id} rec={rec} />
            ))}
          </div>
        </>
      )}
    </main>
  );
}
