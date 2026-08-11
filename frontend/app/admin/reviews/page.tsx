import Link from "next/link";
import type { Metadata } from "next";
import { redirect } from "next/navigation";
import { getSessionToken } from "@/lib/session";
import { adminListReviews } from "@/lib/admin-api";
import { setReviewPublishedAction } from "@/lib/admin-actions";
import { ConfirmButton } from "@/components/ConfirmButton";

export const metadata: Metadata = {
  title: "Reviews — Admin",
};

const RATINGS = ["", "5", "4", "3", "2", "1"];

export default async function AdminReviewsPage({
  searchParams,
}: {
  searchParams: Promise<{ page?: string; rating?: string }>;
}) {
  const token = await getSessionToken();
  if (!token) {
    redirect("/login");
  }

  const { page: pageParam, rating } = await searchParams;
  const page = Number(pageParam ?? "1") || 1;

  const result = await adminListReviews(token, { page, limit: 20, rating: rating ? Number(rating) : undefined });

  return (
    <div>
      <div className="admin-header">
        <h1>Reviews</h1>
      </div>
      <p className="subtitle">
        Moderation only — hiding a review removes it from the public rating and review list; it never edits the
        student&apos;s rating or text.
      </p>

      <form className="admin-search" action="/admin/reviews" method="get">
        <select name="rating" defaultValue={rating ?? ""}>
          {RATINGS.map((r) => (
            <option key={r} value={r}>
              {r ? `${r} stars` : "all ratings"}
            </option>
          ))}
        </select>
        <button type="submit" className="btn-secondary">
          Filter
        </button>
      </form>

      <div className="admin-table-wrapper">
        <table className="admin-table">
          <thead>
            <tr>
              <th>Course</th>
              <th>Student</th>
              <th>Rating</th>
              <th>Text</th>
              <th>Published</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {result.items.map((review) => (
              <tr key={review.id}>
                <td>{review.course_title}</td>
                <td>{review.user_name}</td>
                <td>{review.rating}</td>
                <td>{review.review_text ?? "—"}</td>
                <td>{review.published ? "yes" : "no"}</td>
                <td>
                  {review.published ? (
                    <form action={setReviewPublishedAction.bind(null, review.id, false)}>
                      <ConfirmButton className="btn-danger" confirmMessage="Hide this review from the public course page?">
                        Hide
                      </ConfirmButton>
                    </form>
                  ) : (
                    <form action={setReviewPublishedAction.bind(null, review.id, true)}>
                      <button type="submit" className="btn-small">
                        Restore
                      </button>
                    </form>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="admin-pagination">
        <span>
          Page {result.page} / {result.total_pages || 1} ({result.total} total)
        </span>
        {result.page > 1 && (
          <Link href={`/admin/reviews?page=${result.page - 1}${rating ? `&rating=${rating}` : ""}`}>← Prev</Link>
        )}
        {result.page < result.total_pages && (
          <Link href={`/admin/reviews?page=${result.page + 1}${rating ? `&rating=${rating}` : ""}`}>Next →</Link>
        )}
      </div>
    </div>
  );
}
