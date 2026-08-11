import Link from "next/link";
import { notFound, redirect } from "next/navigation";
import { getSessionToken } from "@/lib/session";
import { adminGetTest, adminListCourses } from "@/lib/admin-api";
import { TestForm } from "@/components/admin/TestForm";
import { QuestionForm } from "@/components/admin/QuestionForm";
import { AnswerForm } from "@/components/admin/AnswerForm";
import { ConfirmButton } from "@/components/ConfirmButton";
import { deleteTestAction, deleteQuestionAction, deleteAnswerAction, setCorrectAnswerAction } from "@/lib/admin-actions";

export default async function AdminTestDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;

  const token = await getSessionToken();
  if (!token) {
    redirect("/login");
  }

  const [test, courses] = await Promise.all([adminGetTest(token, id), adminListCourses(token, { limit: 100 })]);
  if (!test) {
    notFound();
  }

  const questions = [...test.questions].sort((a, b) => a.position - b.position);

  return (
    <div>
      <Link href="/admin/tests">← All tests</Link>
      <h1>{test.title}</h1>

      <div className="admin-card">
        <h2>Details</h2>
        <TestForm test={test} courses={courses.items} />
        <form action={deleteTestAction.bind(null, test.id)} className="mt-3">
          <ConfirmButton className="btn-danger" confirmMessage="Delete this test and all its questions?">
            Delete test
          </ConfirmButton>
        </form>
      </div>

      <h2>Questions</h2>
      {questions.length === 0 && <p>No questions yet.</p>}

      {questions.map((question) => {
        const answers = [...question.answers].sort((a, b) => a.position - b.position);

        return (
          <div key={question.id} className="admin-card">
            <div className="admin-inline-actions">
              <strong>
                {question.position}. {question.text}
              </strong>
              <form action={deleteQuestionAction.bind(null, test.id, question.id)}>
                <ConfirmButton className="btn-danger" confirmMessage="Delete this question and its answers?">
                  Delete
                </ConfirmButton>
              </form>
            </div>

            <details>
              <summary>Edit question text</summary>
              <QuestionForm testId={test.id} questionId={question.id} text={question.text} />
            </details>

            <ul className="lesson-list">
              {answers.map((answer) => (
                <li key={answer.id} className="admin-inline-actions">
                  <span className={answer.is_correct ? "review-correct" : undefined}>
                    {answer.text} {answer.is_correct && "✓ correct"}
                  </span>
                  {!answer.is_correct && (
                    <form action={setCorrectAnswerAction.bind(null, test.id, answer.id, answer.text)}>
                      <button type="submit" className="btn-small">
                        Mark correct
                      </button>
                    </form>
                  )}
                  <form action={deleteAnswerAction.bind(null, test.id, answer.id)}>
                    <ConfirmButton className="btn-danger" confirmMessage="Delete this answer?">
                      Delete
                    </ConfirmButton>
                  </form>
                </li>
              ))}
            </ul>

            <AnswerForm testId={test.id} questionId={question.id} />
          </div>
        );
      })}

      <div className="admin-card">
        <h3>Add question</h3>
        <QuestionForm testId={test.id} />
      </div>
    </div>
  );
}
