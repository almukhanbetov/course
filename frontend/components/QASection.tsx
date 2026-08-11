"use client";

import { useState, useTransition } from "react";
import type { QAAnswerView, QAQuestionView } from "@/lib/api";
import {
  answerQuestionAction,
  askQuestionAction,
  deleteAnswerAction,
  deleteQuestionAction,
  loadMoreQuestionsAction,
} from "@/lib/actions";

function formatDate(iso: string): string {
  return new Date(iso).toLocaleString("ru-RU", { day: "2-digit", month: "2-digit", year: "numeric", hour: "2-digit", minute: "2-digit" });
}

// The initial page is fetched server-side in the lesson page (matching how
// the assignment/coding-exercise sections already work there) — this
// component only ever fetches client-side for what the student explicitly
// triggers: asking, answering, deleting, and "load more" pagination. Every
// mutation updates local state directly from the action's own response
// instead of refetching the whole list, so the rest of the lesson (video
// position, progress) is never disturbed.
export function QASection({
  lessonId,
  currentUserId,
  currentUserName,
  initialQuestions,
  initialPage,
  initialTotalPages,
}: {
  lessonId: string;
  currentUserId: string;
  currentUserName: string;
  initialQuestions: QAQuestionView[];
  initialPage: number;
  initialTotalPages: number;
}) {
  const [questions, setQuestions] = useState<QAQuestionView[]>(initialQuestions);
  const [page, setPage] = useState(initialPage);
  const [totalPages, setTotalPages] = useState(initialTotalPages);

  const [newQuestionBody, setNewQuestionBody] = useState("");
  const [askError, setAskError] = useState<string | null>(null);

  const [openAnswerFormId, setOpenAnswerFormId] = useState<string | null>(null);
  const [answerDrafts, setAnswerDrafts] = useState<Record<string, string>>({});
  const [answerErrors, setAnswerErrors] = useState<Record<string, string>>({});

  const [deleteErrors, setDeleteErrors] = useState<Record<string, string>>({});
  const [loadMoreError, setLoadMoreError] = useState<string | null>(null);

  // One shared transition: every Q&A mutation disables every button while
  // any one of them is in flight, which is a simple, safe way to avoid
  // overlapping races (e.g. deleting a question while it's mid-answer).
  // pendingKey identifies which specific control shows its own "..." label.
  const [isPending, startTransition] = useTransition();
  const [pendingKey, setPendingKey] = useState<string | null>(null);

  function handleAsk() {
    const body = newQuestionBody.trim();
    if (!body) {
      setAskError("Введите текст вопроса.");
      return;
    }
    setAskError(null);
    setPendingKey("ask");
    startTransition(async () => {
      const result = await askQuestionAction(lessonId, body);
      if (result.error || !result.data) {
        setAskError(result.error ?? "Не удалось задать вопрос");
        return;
      }
      const created: QAQuestionView = { ...result.data, display_name: currentUserName, answers: [] };
      setQuestions((prev) => [created, ...prev]);
      setNewQuestionBody("");
    });
  }

  function handleDeleteQuestion(questionId: string) {
    if (!confirm("Удалить этот вопрос?")) return;
    setDeleteErrors((prev) => ({ ...prev, [questionId]: "" }));
    setPendingKey(`delete-q:${questionId}`);
    startTransition(async () => {
      const result = await deleteQuestionAction(questionId);
      if (!result.ok) {
        setDeleteErrors((prev) => ({ ...prev, [questionId]: result.error ?? "Не удалось удалить вопрос" }));
        return;
      }
      setQuestions((prev) => prev.filter((q) => q.id !== questionId));
    });
  }

  function handleSubmitAnswer(questionId: string) {
    const body = (answerDrafts[questionId] ?? "").trim();
    if (!body) {
      setAnswerErrors((prev) => ({ ...prev, [questionId]: "Введите текст ответа." }));
      return;
    }
    setAnswerErrors((prev) => ({ ...prev, [questionId]: "" }));
    setPendingKey(`answer:${questionId}`);
    startTransition(async () => {
      const result = await answerQuestionAction(questionId, body);
      if (result.error || !result.data) {
        setAnswerErrors((prev) => ({ ...prev, [questionId]: result.error ?? "Не удалось отправить ответ" }));
        return;
      }
      const created: QAAnswerView = { ...result.data, display_name: currentUserName };
      setQuestions((prev) =>
        prev.map((q) => (q.id === questionId ? { ...q, answers: [...q.answers, created] } : q)),
      );
      setAnswerDrafts((prev) => ({ ...prev, [questionId]: "" }));
      setOpenAnswerFormId(null);
    });
  }

  function handleDeleteAnswer(questionId: string, answerId: string) {
    if (!confirm("Удалить этот ответ?")) return;
    setDeleteErrors((prev) => ({ ...prev, [answerId]: "" }));
    setPendingKey(`delete-a:${answerId}`);
    startTransition(async () => {
      const result = await deleteAnswerAction(answerId);
      if (!result.ok) {
        setDeleteErrors((prev) => ({ ...prev, [answerId]: result.error ?? "Не удалось удалить ответ" }));
        return;
      }
      setQuestions((prev) =>
        prev.map((q) => (q.id === questionId ? { ...q, answers: q.answers.filter((a) => a.id !== answerId) } : q)),
      );
    });
  }

  function handleLoadMore() {
    setLoadMoreError(null);
    setPendingKey("load-more");
    startTransition(async () => {
      const result = await loadMoreQuestionsAction(lessonId, page + 1);
      if (result.error || !result.data) {
        setLoadMoreError(result.error ?? "Не удалось загрузить вопросы");
        return;
      }
      setQuestions((prev) => [...prev, ...result.data!.items]);
      setPage(result.data.page);
      setTotalPages(result.data.total_pages);
    });
  }

  return (
    <section className="qa-section">
      <h2>Вопросы к уроку</h2>

      <div className="qa-form">
        <textarea
          placeholder="Задайте вопрос по этому уроку..."
          value={newQuestionBody}
          onChange={(e) => setNewQuestionBody(e.target.value)}
          disabled={isPending && pendingKey === "ask"}
        />
        {askError && <p role="alert">{askError}</p>}
        <div className="qa-form-actions">
          <button type="button" className="btn-primary" onClick={handleAsk} disabled={isPending}>
            {isPending && pendingKey === "ask" ? "Отправка..." : "Задать вопрос"}
          </button>
        </div>
      </div>

      {questions.length === 0 ? (
        <div className="empty-state">
          <p>Пока нет вопросов к этому уроку. Будьте первым!</p>
        </div>
      ) : (
        <ul className="qa-question-list">
          {questions.map((q) => (
            <li key={q.id} className="qa-question-item">
              <div className="qa-question-header">
                <span>
                  <strong>{q.display_name}</strong> · <span className="qa-answer-body">{formatDate(q.created_at)}</span>
                </span>
                {q.user_id === currentUserId && (
                  <button
                    type="button"
                    className="btn-small"
                    onClick={() => handleDeleteQuestion(q.id)}
                    disabled={isPending}
                  >
                    {isPending && pendingKey === `delete-q:${q.id}` ? "Удаление..." : "Удалить"}
                  </button>
                )}
              </div>
              <p className="qa-question-body">{q.body}</p>
              {deleteErrors[q.id] && <p role="alert">{deleteErrors[q.id]}</p>}

              {q.answers.length > 0 && (
                <ul className="qa-answer-list">
                  {q.answers.map((a) => (
                    <li key={a.id} className="qa-answer-item">
                      <div className="qa-answer-header">
                        <span>
                          <strong>{a.display_name}</strong>
                          {a.is_instructor_answer && <span className="badge badge-instructor">Преподаватель</span>}
                          {" · "}
                          {formatDate(a.created_at)}
                        </span>
                        {a.user_id === currentUserId && (
                          <button
                            type="button"
                            className="btn-small"
                            onClick={() => handleDeleteAnswer(q.id, a.id)}
                            disabled={isPending}
                          >
                            {isPending && pendingKey === `delete-a:${a.id}` ? "Удаление..." : "Удалить"}
                          </button>
                        )}
                      </div>
                      <p className="qa-answer-body">{a.body}</p>
                      {deleteErrors[a.id] && <p role="alert">{deleteErrors[a.id]}</p>}
                    </li>
                  ))}
                </ul>
              )}

              {openAnswerFormId === q.id ? (
                <div className="qa-form">
                  <textarea
                    placeholder="Ваш ответ..."
                    value={answerDrafts[q.id] ?? ""}
                    onChange={(e) => setAnswerDrafts((prev) => ({ ...prev, [q.id]: e.target.value }))}
                    disabled={isPending && pendingKey === `answer:${q.id}`}
                  />
                  {answerErrors[q.id] && <p role="alert">{answerErrors[q.id]}</p>}
                  <div className="qa-form-actions">
                    <button
                      type="button"
                      className="btn-primary"
                      onClick={() => handleSubmitAnswer(q.id)}
                      disabled={isPending}
                    >
                      {isPending && pendingKey === `answer:${q.id}` ? "Отправка..." : "Ответить"}
                    </button>
                    <button
                      type="button"
                      className="btn-secondary"
                      onClick={() => setOpenAnswerFormId(null)}
                      disabled={isPending}
                    >
                      Отмена
                    </button>
                  </div>
                </div>
              ) : (
                <button
                  type="button"
                  className="btn-small mt-3"
                  onClick={() => setOpenAnswerFormId(q.id)}
                  disabled={isPending}
                >
                  Ответить
                </button>
              )}
            </li>
          ))}
        </ul>
      )}

      {page < totalPages && (
        <div className="qa-form-actions mt-3">
          <button type="button" className="btn-secondary" onClick={handleLoadMore} disabled={isPending}>
            {isPending && pendingKey === "load-more" ? "Загрузка..." : "Показать ещё вопросы"}
          </button>
        </div>
      )}
      {loadMoreError && <p role="alert">{loadMoreError}</p>}
    </section>
  );
}
