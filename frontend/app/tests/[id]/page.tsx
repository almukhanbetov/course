import Link from "next/link";
import { notFound, redirect } from "next/navigation";
import { getTest, getMyAttemptDetail } from "@/lib/api";
import { getSessionToken } from "@/lib/session";
import { submitTestAction } from "@/lib/actions";

export default async function TestPage({
  params,
  searchParams,
}: {
  params: Promise<{ id: string }>;
  searchParams: Promise<{ attempt?: string; error?: string }>;
}) {
  const { id } = await params;
  const { attempt: attemptId, error: errorMessage } = await searchParams;

  const token = await getSessionToken();
  if (!token) {
    redirect("/login");
  }

  if (attemptId) {
    const detail = await getMyAttemptDetail(token, attemptId);
    if (!detail) {
      notFound();
    }

    const correctCount = detail.answers.filter((a) => a.correct).length;

    return (
      <main>
        <h1>{detail.test_title}</h1>
        <div className="status">
          <p>
            <strong>Результат:</strong> {detail.score}% (проходной балл {detail.passing_score}%)
          </p>
          <p>
            <span className={`badge ${detail.passed ? "badge-free" : ""}`}>
              {detail.passed ? "Тест пройден ✓" : "Тест не пройден"}
            </span>
          </p>
          <p className="my-course-meta">
            {correctCount} / {detail.total_questions} правильных ответов
          </p>
        </div>

        <h2>Разбор ответов</h2>
        {detail.answers.map((a, idx) => (
          <div key={a.question_id} className="test-review-item">
            <p>
              <strong>
                {idx + 1}. {a.question_text}
              </strong>
            </p>
            <p className={a.correct ? "review-correct" : "review-incorrect"}>
              Ваш ответ: {a.selected_answer_text} {a.correct ? "✓" : "✗"}
            </p>
            {!a.correct && <p className="review-correct">Правильный ответ: {a.correct_answer_text}</p>}
          </div>
        ))}

        <div className="lesson-actions">
          {!detail.passed && (
            <Link href={`/tests/${id}`} className="btn-primary">
              Пройти ещё раз
            </Link>
          )}
          <Link href="/dashboard/courses" className="btn-secondary">
            Мои курсы
          </Link>
        </div>
      </main>
    );
  }

  const access = await getTest(token, id);

  if (access.kind === "not_found") {
    notFound();
  }

  if (access.kind === "not_enrolled") {
    return (
      <main>
        <p role="alert">Вы не записаны на курс, которому принадлежит этот тест.</p>
      </main>
    );
  }

  if (access.kind === "lessons_not_completed") {
    return (
      <main>
        <p role="alert">Сначала завершите все уроки курса — итоговый тест пока недоступен.</p>
        <Link href="/dashboard/courses">Мои курсы</Link>
      </main>
    );
  }

  if (access.kind === "error") {
    return (
      <main>
        <p role="alert">Не удалось загрузить тест: {access.message}</p>
      </main>
    );
  }

  const { test } = access;

  return (
    <main>
      <h1>{test.title}</h1>
      {test.description && <p className="subtitle">{test.description}</p>}
      <p className="my-course-meta">Проходной балл: {test.passing_score}%</p>

      {errorMessage && <p role="alert">{decodeURIComponent(errorMessage)}</p>}

      <form action={submitTestAction.bind(null, id)} className="test-form">
        {test.questions.map((question, idx) => (
          <fieldset key={question.id} className="test-question">
            <legend>
              {idx + 1}. {question.text}
            </legend>
            {question.answers.map((answer) => (
              <label key={answer.id} className="test-option">
                <input type="radio" name={`q-${question.id}`} value={answer.id} required />
                {answer.text}
              </label>
            ))}
          </fieldset>
        ))}
        <button type="submit" className="btn-primary">
          Завершить тест
        </button>
      </form>
    </main>
  );
}
