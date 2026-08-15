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
import { ErrorState } from "@/components/shell/ErrorState";
import { IconLayers, IconPlayCircle, IconUser } from "@/components/shell/icons";
import { getDictionary, localizeLevel } from "@/lib/i18n/dictionaries";
import { localizeDescription, localizeTitle } from "@/lib/i18n/localize";
import { getLocale } from "@/lib/i18n/getLocale";

const DATE_LOCALES: Record<string, string> = { ru: "ru-RU", kk: "kk-KZ", en: "en-US" };

export default async function CourseDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  const locale = await getLocale();
  const dict = getDictionary(locale);

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
        <ErrorState message={dict.courseDetail.errorLoad(error)} />
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

  const lessonCount = course.modules.reduce((sum, m) => sum + m.lessons.length, 0);
  const courseTitle = localizeTitle(course, locale);

  return (
    <main className="course-detail-shell">
      <nav className="breadcrumbs" aria-label={dict.breadcrumbs.ariaLabel}>
        <Link href="/">{dict.breadcrumbs.home}</Link>
        <span>/</span>
        <Link href="/courses">{dict.breadcrumbs.courses}</Link>
        {course.category_name && course.category_slug && (
          <>
            <span>/</span>
            <Link href={`/categories/${course.category_slug}`}>{course.category_name}</Link>
          </>
        )}
        <span>/</span>
        <span>{courseTitle}</span>
      </nav>

      <div className="course-detail-layout">
        <div className="course-detail-main">
          <div className="course-hero">
            <div className="course-hero-badges">
              {course.category_name && course.category_slug && (
                <Link href={`/categories/${course.category_slug}`} className="badge badge-category">
                  {course.category_name}
                </Link>
              )}
              <span className="badge">{localizeLevel(course.level, dict)}</span>
              <span className={`badge ${isPremium ? "badge-premium" : "badge-free"}`}>
                {isPremium ? dict.courses.accessSubscription : dict.courses.accessFree}
              </span>
            </div>

            <h1>{courseTitle}</h1>

            <div className="rating-summary">
              <span className="rating-average">{course.rating_average.toFixed(1)}</span>
              <Rating average={course.rating_average} count={course.rating_count} locale={locale} />
            </div>

            {course.modules.length > 0 && (
              <div className="course-hero-meta">
                <span>
                  <IconLayers size={15} /> {dict.courseDetail.modulesCount(course.modules.length)}
                </span>
                <span>
                  <IconPlayCircle size={15} /> {dict.courseDetail.lessonsCount(lessonCount)}
                </span>
              </div>
            )}

            {course.instructor_name && (
              <p className="course-hero-instructor">
                <IconUser size={15} /> {dict.courseDetail.instructorLabel(course.instructor_name)}
              </p>
            )}

            <p className="course-hero-description">{localizeDescription(course, locale)}</p>
          </div>

          <h2>{dict.courseDetail.curriculumTitle}</h2>
          {course.modules.length === 0 && <p>{dict.courseDetail.noModules}</p>}

          {course.modules.map((module) => (
            <section key={module.id} className="module">
              <h3>{module.title}</h3>
              <ul className="lesson-list">
                {module.lessons.map((lesson) => (
                  <li key={lesson.id}>
                    {lesson.title}
                    {lesson.is_free && <span className="badge badge-free">{dict.courseDetail.freeBadge}</span>}
                  </li>
                ))}
              </ul>
            </section>
          ))}

          <h2>{dict.courseDetail.reviewsTitle}</h2>

          {canReview && <ReviewForm courseId={id} myReview={myReview} />}
          {token && myCourse && !canReview && <p className="subtitle">{dict.courseDetail.canReviewHint}</p>}

          {!reviews || reviews.items.length === 0 ? (
            <p>{dict.courseDetail.noReviews}</p>
          ) : (
            <ul className="review-list">
              {reviews.items.map((review) => (
                <li key={review.id} className="review-item">
                  <div className="review-item-header">
                    <span className="review-author">{review.display_name}</span>
                    <span className="review-date">
                      {new Date(review.created_at).toLocaleDateString(DATE_LOCALES[locale])}
                    </span>
                  </div>
                  <span className="rating">
                    {"★".repeat(review.rating)}
                    {"☆".repeat(5 - review.rating)}
                  </span>
                  {review.review_text && <p className="review-text">{review.review_text}</p>}
                </li>
              ))}
            </ul>
          )}

          {similarCourses.length > 0 && (
            <>
              <h2 className="mt-3">{dict.courseDetail.similarCoursesTitle}</h2>
              <div className="course-grid">
                {similarCourses.map((rec) => (
                  <RecommendationCard key={rec.course_id} rec={rec} locale={locale} />
                ))}
              </div>
            </>
          )}
        </div>

        <aside className="course-detail-aside">
          <div className="enroll-card">
            {!token && (
              <Link href="/login" className="btn-primary">
                {dict.courseDetail.enrollLoginPrompt}
              </Link>
            )}
            {token && myCourse && (
              <Link href="/dashboard/courses" className="btn-primary">
                {dict.courseDetail.continueLabel(myCourse.progress_percent)}
              </Link>
            )}
            {token && !myCourse && !hasAccess && (
              <Link href="/pricing" className="btn-primary">
                {dict.courseDetail.subscribeCta}
              </Link>
            )}
            {token && !myCourse && hasAccess && (
              <form action={enrollAction.bind(null, id)}>
                <button type="submit" className="btn-primary" style={{ width: "100%" }}>
                  {dict.courseDetail.enrollCta}
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
        </aside>
      </div>
    </main>
  );
}
