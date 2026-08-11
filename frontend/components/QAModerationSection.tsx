"use client";

import { useState, useTransition } from "react";
import type { QAAnswerView, QAQuestionView } from "@/lib/api";
import { answerQuestionAction, deleteAnswerAction, deleteQuestionAction } from "@/lib/actions";

// One entry per lesson that actually has at least one question — lessons
// with none are never fetched into this shape by the page (see
// app/instructor/questions/page.tsx / app/admin/questions/page.tsx), so an
// empty ModerationGroup never exists here to begin with.
export interface ModerationGroup {
  courseId: string;
  courseTitle: string;
  lessonId: string;
  lessonTitle: string;
  questions: QAQuestionView[];
}

function formatDate(iso: string): string {
  return new Date(iso).toLocaleString("ru-RU", { day: "2-digit", month: "2-digit", year: "numeric", hour: "2-digit", minute: "2-digit" });
}

function updateQuestion(groups: ModerationGroup[], questionId: string, updater: (q: QAQuestionView) => QAQuestionView): ModerationGroup[] {
  return groups.map((g) => ({
    ...g,
    questions: g.questions.map((q) => (q.id === questionId ? updater(q) : q)),
  }));
}

// Removing the last question of a group drops the group entirely — a
// lesson with zero remaining questions has nothing left to show, matching
// how the page never fetches/renders empty lessons in the first place.
function removeQuestion(groups: ModerationGroup[], questionId: string): ModerationGroup[] {
  return groups
    .map((g) => ({ ...g, questions: g.questions.filter((q) => q.id !== questionId) }))
    .filter((g) => g.questions.length > 0);
}

// Instructor/admin moderation view (Stage 20B2). Deliberately does NOT
// include an "ask a question" form (moderators don't ask) and does NOT
// include a hide/show toggle — the backend has no endpoint for that yet
// (see STAGE20_PROGRESS.md's Stage 20B2 section), so this only ever wires
// up actions the existing Stage 20A backend actually authorizes for this
// caller: answering any question in a course they own/administer (reusing
// the exact same action as the student-facing QASection), and deleting
// only their own prior question/answer — never anyone else's, since the
// backend's delete is strictly own-content-only regardless of role.
export function QAModerationSection({
  groups: initialGroups,
  currentUserId,
  currentUserName,
}: {
  groups: ModerationGroup[];
  currentUserId: string;
  currentUserName: string;
}) {
  const [groups, setGroups] = useState<ModerationGroup[]>(initialGroups);

  const [openAnswerFormId, setOpenAnswerFormId] = useState<string | null>(null);
  const [answerDrafts, setAnswerDrafts] = useState<Record<string, string>>({});
  const [answerErrors, setAnswerErrors] = useState<Record<string, string>>({});
  const [deleteErrors, setDeleteErrors] = useState<Record<string, string>>({});

  const [isPending, startTransition] = useTransition();
  const [pendingKey, setPendingKey] = useState<string | null>(null);

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
      setGroups((prev) => removeQuestion(prev, questionId));
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
      setGroups((prev) => updateQuestion(prev, questionId, (q) => ({ ...q, answers: [...q.answers, created] })));
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
      setGroups((prev) => updateQuestion(prev, questionId, (q) => ({ ...q, answers: q.answers.filter((a) => a.id !== answerId) })));
    });
  }

  if (groups.length === 0) {
    return (
      <div className="empty-state">
        <p>Пока нет вопросов по вашим курсам.</p>
      </div>
    );
  }

  return (
    <div>
      {groups.map((group) => (
        <section key={group.lessonId} className="qa-moderation-group">
          <div className="qa-moderation-group-header">
            <span className="my-course-meta">{group.courseTitle}</span>
            <h3>{group.lessonTitle}</h3>
          </div>

          <ul className="qa-question-list">
            {group.questions.map((q) => (
              <li key={q.id} className="qa-question-item">
                <div className="qa-question-header">
                  <span>
                    <strong>{q.display_name}</strong> · <span className="qa-answer-body">{formatDate(q.created_at)}</span>{" "}
                    <span className="badge">{q.published ? "Опубликован" : "Скрыт"}</span>
                  </span>
                  {q.user_id === currentUserId && (
                    <button type="button" className="btn-small" onClick={() => handleDeleteQuestion(q.id)} disabled={isPending}>
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
                            {formatDate(a.created_at)} · <span className="badge">{a.published ? "Опубликован" : "Скрыт"}</span>
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
                      <button type="button" className="btn-primary" onClick={() => handleSubmitAnswer(q.id)} disabled={isPending}>
                        {isPending && pendingKey === `answer:${q.id}` ? "Отправка..." : "Ответить"}
                      </button>
                      <button type="button" className="btn-secondary" onClick={() => setOpenAnswerFormId(null)} disabled={isPending}>
                        Отмена
                      </button>
                    </div>
                  </div>
                ) : (
                  <button type="button" className="btn-small mt-3" onClick={() => setOpenAnswerFormId(q.id)} disabled={isPending}>
                    Ответить
                  </button>
                )}
              </li>
            ))}
          </ul>
        </section>
      ))}
    </div>
  );
}
