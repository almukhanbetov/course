import Link from "next/link";
import { notFound, redirect } from "next/navigation";
import { getSessionToken } from "@/lib/session";
import { instructorGetTest } from "@/lib/instructor-api";
import { InstructorTestForm } from "@/components/instructor/InstructorTestForm";
import { InstructorQuestionForm } from "@/components/instructor/InstructorQuestionForm";
import { InstructorAnswerForm } from "@/components/instructor/InstructorAnswerForm";
import { ConfirmButton } from "@/components/ConfirmButton";
import {
  deleteInstructorTestAction,
  deleteInstructorQuestionAction,
  deleteInstructorAnswerAction,
  setInstructorCorrectAnswerAction,
} from "@/lib/instructor-actions";

export default async function InstructorTestDetailPage({
  params,
}: {
  params: Promise<{ id: string; testId: string }>;
}) {
  const { id: courseId, testId } = await params;

  const token = await getSessionToken();
  if (!token) {
    redirect("/login");
  }

  const test = await instructorGetTest(token, testId);
  if (!test) {
    notFound();
  }

  const questions = [...test.questions].sort((a, b) => a.position - b.position);

  return (
    <div>
      <Link href={`/instructor/courses/${courseId}`}>← Курс</Link>
      <h1>{test.title}</h1>

      <div className="admin-card">
        <h2>Параметры теста</h2>
        <InstructorTestForm courseId={courseId} test={test} />
        <form action={deleteInstructorTestAction.bind(null, courseId, test.id)} className="mt-3">
          <ConfirmButton className="btn-danger" confirmMessage="Удалить тест и все его вопросы?">
            Удалить тест
          </ConfirmButton>
        </form>
      </div>

      <h2>Вопросы</h2>
      {questions.length === 0 && <p>Вопросов пока нет.</p>}

      {questions.map((question) => {
        const answers = [...question.answers].sort((a, b) => a.position - b.position);

        return (
          <div key={question.id} className="admin-card">
            <div className="admin-inline-actions">
              <strong>
                {question.position}. {question.text}
              </strong>
              <form action={deleteInstructorQuestionAction.bind(null, courseId, test.id, question.id)}>
                <ConfirmButton className="btn-danger" confirmMessage="Удалить вопрос и его ответы?">
                  Удалить
                </ConfirmButton>
              </form>
            </div>

            <details>
              <summary>Изменить текст вопроса</summary>
              <InstructorQuestionForm courseId={courseId} testId={test.id} questionId={question.id} text={question.text} />
            </details>

            <ul className="lesson-list">
              {answers.map((answer) => (
                <li key={answer.id} className="admin-inline-actions">
                  <span className={answer.is_correct ? "review-correct" : undefined}>
                    {answer.text} {answer.is_correct && "✓ правильный"}
                  </span>
                  {!answer.is_correct && (
                    <form action={setInstructorCorrectAnswerAction.bind(null, courseId, test.id, answer.id, answer.text)}>
                      <button type="submit" className="btn-small">
                        Сделать правильным
                      </button>
                    </form>
                  )}
                  <form action={deleteInstructorAnswerAction.bind(null, courseId, test.id, answer.id)}>
                    <ConfirmButton className="btn-danger" confirmMessage="Удалить этот ответ?">
                      Удалить
                    </ConfirmButton>
                  </form>
                </li>
              ))}
            </ul>

            <InstructorAnswerForm courseId={courseId} testId={test.id} questionId={question.id} />
          </div>
        );
      })}

      <div className="admin-card">
        <h3>Добавить вопрос</h3>
        <InstructorQuestionForm courseId={courseId} testId={test.id} />
      </div>
    </div>
  );
}
