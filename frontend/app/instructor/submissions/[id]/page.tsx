import Link from "next/link";
import { notFound, redirect } from "next/navigation";
import { getSessionToken } from "@/lib/session";
import { instructorGetSubmissionDetail } from "@/lib/instructor-api";
import { SubmissionStatusBadge } from "@/components/SubmissionStatusBadge";
import { SubmissionFileDownloadLink } from "@/components/SubmissionFileDownloadLink";
import { ReviewForm } from "@/components/instructor/ReviewForm";

export default async function InstructorSubmissionDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;

  const token = await getSessionToken();
  if (!token) {
    redirect("/login");
  }

  const submission = await instructorGetSubmissionDetail(token, id);
  if (!submission) {
    notFound();
  }

  const reviews = [...submission.reviews].reverse();

  return (
    <div>
      <Link href="/instructor/submissions">← Входящие решения</Link>
      <h1>{submission.assignment_title}</h1>
      <p className="subtitle">
        {submission.course_title} · {submission.lesson_title} · {submission.student_name}
      </p>

      <div className="admin-card">
        <h2>Решение студента</h2>
        <p>
          <SubmissionStatusBadge status={submission.status} />
          {submission.score != null && <span className="my-course-meta"> Балл: {submission.score}</span>}
        </p>
        {submission.submitted_at && (
          <p className="my-course-meta">Отправлено: {new Date(submission.submitted_at).toLocaleString("ru-RU")}</p>
        )}
        {submission.text_content && <p>{submission.text_content}</p>}
        {submission.files.length > 0 && (
          <div className="admin-inline-actions my-2">
            {submission.files.map((f) => (
              <SubmissionFileDownloadLink key={f.id} file={f} />
            ))}
          </div>
        )}
      </div>

      <div className="admin-card">
        <h2>Проверка</h2>
        <ReviewForm submission={submission} />
      </div>

      {reviews.length > 0 && (
        <div className="admin-card">
          <h2>История проверок</h2>
          <ul className="lesson-list">
            {reviews.map((r) => (
              <li key={r.id}>
                <SubmissionStatusBadge status={r.status} /> {r.score != null && `— ${r.score} баллов`} —{" "}
                {new Date(r.created_at).toLocaleString("ru-RU")}
                {r.feedback && <p className="my-course-meta">{r.feedback}</p>}
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}
