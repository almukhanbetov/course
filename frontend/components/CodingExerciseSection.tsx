"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import Editor from "@monaco-editor/react";
import type { CodeSubmission, StudentCodingExercise, StudentTestCaseExample } from "@/lib/api";
import { TERMINAL_SUBMISSION_STATUSES } from "@/lib/api";
import { getCodeSubmissionAction, listCodeAttemptsAction, runCodeAction, submitCodeAction } from "@/lib/actions";
import { CodeSubmissionStatusBadge } from "@/components/CodeSubmissionStatusBadge";

// Item: "никогда не запускай код автоматически — ни при загрузке страницы,
// ни при вводе текста, ни при автосохранении, только явные Run/Submit".
// This component enforces that structurally: the editor's onChange only
// ever calls setSourceCode (local state), and runCodeAction/submitCodeAction
// are wired exclusively to the two buttons' onClick handlers below — there
// is no useEffect that depends on sourceCode.
const POLL_MS = 1500;

function editorLanguage(language: StudentCodingExercise["language"]): string {
  return language === "javascript" ? "javascript" : language;
}

export function CodingExerciseSection({
  exercise,
  examples,
  initialAttempts,
}: {
  exercise: StudentCodingExercise;
  examples: StudentTestCaseExample[];
  initialAttempts: CodeSubmission[];
}) {
  const latestSubmit = initialAttempts.find((a) => a.mode === "submit");
  const [sourceCode, setSourceCode] = useState(latestSubmit?.source_code ?? exercise.starter_code);
  const [current, setCurrent] = useState<CodeSubmission | null>(null);
  const [attempts, setAttempts] = useState<CodeSubmission[]>(initialAttempts);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const pollTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);

  useEffect(() => {
    return () => {
      if (pollTimer.current) clearTimeout(pollTimer.current);
    };
  }, []);

  const refreshAttempts = useCallback(async () => {
    const result = await listCodeAttemptsAction(exercise.id);
    if (result.attempts) setAttempts(result.attempts);
  }, [exercise.id]);

  const poll = useCallback(
    function poll(submissionId: string) {
      pollTimer.current = setTimeout(async () => {
        const result = await getCodeSubmissionAction(submissionId);
        if (result.error) {
          setError(result.error);
          setBusy(false);
          return;
        }
        if (!result.submission) return;
        setCurrent(result.submission);
        if (TERMINAL_SUBMISSION_STATUSES.has(result.submission.status)) {
          setBusy(false);
          void refreshAttempts();
        } else {
          poll(submissionId);
        }
      }, POLL_MS);
    },
    [refreshAttempts],
  );

  async function handleRun() {
    setError(null);
    setBusy(true);
    const result = await runCodeAction(exercise.id, sourceCode);
    if (result.error) {
      setError(result.error);
      setBusy(false);
      return;
    }
    if (result.submission) {
      setCurrent(result.submission);
      poll(result.submission.id);
    }
  }

  async function handleSubmit() {
    setError(null);
    setBusy(true);
    const result = await submitCodeAction(exercise.id, sourceCode);
    if (result.error) {
      setError(result.error);
      setBusy(false);
      return;
    }
    if (result.submission) {
      setCurrent(result.submission);
      poll(result.submission.id);
    }
  }

  return (
    <section className="admin-card">
      <h2>Практика кода</h2>
      <h3>{exercise.title}</h3>
      {exercise.description && <p>{exercise.description}</p>}
      {exercise.required && <p className="my-course-meta">Обязательное упражнение — для завершения урока нужно успешно пройти его через «Отправить решение».</p>}
      <p className="my-course-meta">
        Язык: {exercise.language} · лимит времени: {(exercise.time_limit_ms / 1000).toFixed(1)} с · лимит памяти: {exercise.memory_limit_mb} МБ
      </p>

      {examples.length > 0 && (
        <div className="my-2">
          <p className="my-course-meta">Пример:</p>
          {examples.map((ex) => (
            <div key={ex.id} className="admin-card">
              {ex.input != null && (
                <pre>
                  <strong>Ввод:</strong>{"\n"}
                  {ex.input}
                </pre>
              )}
              <pre>
                <strong>Ожидаемый вывод:</strong>{"\n"}
                {ex.expected_output}
              </pre>
            </div>
          ))}
        </div>
      )}

      <div className="my-2" style={{ border: "1px solid var(--border-color, #333)" }}>
        <Editor
          height="360px"
          language={editorLanguage(exercise.language)}
          value={sourceCode}
          onChange={(value) => setSourceCode(value ?? "")}
          theme="vs-dark"
          options={{ minimap: { enabled: false }, fontSize: 14, automaticLayout: true }}
        />
      </div>

      <div className="admin-inline-actions my-2">
        <button type="button" className="btn-secondary" onClick={handleRun} disabled={busy}>
          {busy ? "Выполняется…" : "Запустить"}
        </button>
        <button type="button" className="btn-primary" onClick={handleSubmit} disabled={busy}>
          {busy ? "Выполняется…" : "Отправить решение"}
        </button>
      </div>

      {error && <p role="alert">{error}</p>}

      {current && (
        <div className="admin-card my-2">
          <div className="admin-inline-actions">
            <CodeSubmissionStatusBadge status={current.status} />
            {current.mode === "submit" && current.total_tests > 0 && (
              <span className="my-course-meta">
                Пройдено тестов: {current.passed_tests} / {current.total_tests}
              </span>
            )}
          </div>
          {current.compile_output && (
            <pre role="alert" className="my-2">
              {current.compile_output}
            </pre>
          )}
          {current.mode === "run" && current.stdout != null && (
            <pre className="my-2">{current.stdout || "(программа не вывела ничего в stdout)"}</pre>
          )}
        </div>
      )}

      {attempts.length > 0 && (
        <details className="my-2">
          <summary>История попыток ({attempts.length})</summary>
          <ul className="lesson-list">
            {attempts.map((a) => (
              <li key={a.id} className="admin-inline-actions">
                <CodeSubmissionStatusBadge status={a.status} />
                <span className="my-course-meta">
                  {a.mode === "submit" ? "Отправка" : "Запуск"} · {new Date(a.created_at).toLocaleString("ru-RU")}
                  {a.mode === "submit" && a.total_tests > 0 ? ` · ${a.passed_tests}/${a.total_tests}` : ""}
                </span>
              </li>
            ))}
          </ul>
        </details>
      )}
    </section>
  );
}
