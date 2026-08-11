"use client";

import { useActionState } from "react";
import {
  createInstructorCodingExerciseAction,
  createInstructorTestCaseAction,
  deleteInstructorCodingExerciseAction,
  deleteInstructorTestCaseAction,
  updateInstructorCodingExerciseAction,
  updateInstructorTestCaseAction,
} from "@/lib/instructor-actions";
import { ConfirmButton } from "@/components/ConfirmButton";
import type { FormState } from "@/lib/actions";
import type { InstructorCodingExercise, InstructorTestCase } from "@/lib/instructor-api";

const initialState: FormState = { error: null };

function ExerciseForm({
  courseId,
  lessonId,
  exercise,
}: {
  courseId: string;
  lessonId: string;
  exercise: InstructorCodingExercise | null;
}) {
  const action = exercise
    ? updateInstructorCodingExerciseAction.bind(null, courseId, exercise.id)
    : createInstructorCodingExerciseAction.bind(null, courseId, lessonId);
  const [state, formAction, pending] = useActionState(action, initialState);

  return (
    <form action={formAction} className="admin-form">
      <label>
        Название упражнения
        <input type="text" name="title" defaultValue={exercise?.title} required />
      </label>
      <label>
        Описание задачи (условие)
        <textarea name="description" rows={3} defaultValue={exercise?.description} />
      </label>
      <label>
        Язык
        <select name="language" defaultValue={exercise?.language ?? "python"}>
          <option value="go">Go</option>
          <option value="python">Python</option>
          <option value="javascript">JavaScript (Node.js)</option>
        </select>
      </label>
      <label>
        Стартовый код (виден студенту)
        <textarea name="starter_code" rows={6} defaultValue={exercise?.starter_code} style={{ fontFamily: "monospace" }} />
      </label>
      <label>
        Эталонное решение (только для инструктора, никогда не видно студенту)
        <textarea name="solution_code" rows={6} defaultValue={exercise?.solution_code} style={{ fontFamily: "monospace" }} />
      </label>
      <label>
        Лимит времени, мс
        <input type="number" name="time_limit_ms" min={100} defaultValue={exercise?.time_limit_ms ?? 5000} required />
      </label>
      <label>
        Лимит памяти, МБ
        <input type="number" name="memory_limit_mb" min={16} defaultValue={exercise?.memory_limit_mb ?? 128} required />
      </label>
      <label className="admin-form-checkbox">
        <input type="checkbox" name="required" defaultChecked={exercise?.required ?? true} />
        Обязательное (влияет на завершение урока)
      </label>
      <label className="admin-form-checkbox">
        <input type="checkbox" name="published" defaultChecked={exercise?.published} />
        Опубликовано (видно студентам)
      </label>
      {state.error && <p role="alert">{state.error}</p>}
      <button type="submit" className="btn-small" disabled={pending}>
        {pending ? "..." : exercise ? "Сохранить упражнение" : "Создать упражнение"}
      </button>
    </form>
  );
}

function TestCaseForm({
  courseId,
  exerciseId,
  testCase,
  nextPosition,
}: {
  courseId: string;
  exerciseId: string;
  testCase: InstructorTestCase | null;
  nextPosition: number;
}) {
  const action = testCase
    ? updateInstructorTestCaseAction.bind(null, courseId, testCase.id)
    : createInstructorTestCaseAction.bind(null, courseId, exerciseId);
  const [state, formAction, pending] = useActionState(action, initialState);

  return (
    <form action={formAction} className="admin-form">
      <label>
        Ввод (stdin), необязательно
        <textarea name="input" rows={2} defaultValue={testCase?.input} style={{ fontFamily: "monospace" }} />
      </label>
      <label>
        Ожидаемый вывод
        <textarea name="expected_output" rows={2} defaultValue={testCase?.expected_output} required style={{ fontFamily: "monospace" }} />
      </label>
      <label>
        Позиция
        <input type="number" name="position" min={1} defaultValue={testCase?.position ?? nextPosition} required />
      </label>
      <label className="admin-form-checkbox">
        <input type="checkbox" name="hidden" defaultChecked={testCase?.hidden ?? true} />
        Скрытый тест (студент не видит ввод/вывод, только пройден/не пройден)
      </label>
      {state.error && <p role="alert">{state.error}</p>}
      <button type="submit" className="btn-small" disabled={pending}>
        {pending ? "..." : testCase ? "Сохранить тест" : "Добавить тест"}
      </button>
    </form>
  );
}

export function InstructorCodingExerciseEditor({
  courseId,
  lessonId,
  exercise,
  testCases,
}: {
  courseId: string;
  lessonId: string;
  exercise: InstructorCodingExercise | null;
  testCases: InstructorTestCase[];
}) {
  return (
    <div className="video-upload-block">
      <ExerciseForm courseId={courseId} lessonId={lessonId} exercise={exercise} />

      {exercise && (
        <>
          <form action={deleteInstructorCodingExerciseAction.bind(null, courseId, exercise.id)} className="mt-3">
            <ConfirmButton className="btn-danger" confirmMessage="Удалить это упражнение? Возможно только если ещё нет решений студентов.">
              Удалить упражнение
            </ConfirmButton>
          </form>

          <h4 className="mt-3">Тестовые случаи ({testCases.length})</h4>
          <ul className="lesson-list">
            {testCases.map((tc) => (
              <li key={tc.id}>
                <details>
                  <summary>
                    #{tc.position} {tc.hidden ? <span className="badge">скрытый</span> : <span className="badge badge-free">пример</span>}
                  </summary>
                  <TestCaseForm courseId={courseId} exerciseId={exercise.id} testCase={tc} nextPosition={tc.position} />
                  <form action={deleteInstructorTestCaseAction.bind(null, courseId, tc.id)} className="mt-3">
                    <ConfirmButton className="btn-danger" confirmMessage="Удалить этот тест?">
                      Удалить тест
                    </ConfirmButton>
                  </form>
                </details>
              </li>
            ))}
          </ul>

          <details className="mt-3">
            <summary>Добавить тестовый случай</summary>
            <TestCaseForm courseId={courseId} exerciseId={exercise.id} testCase={null} nextPosition={testCases.length + 1} />
          </details>
        </>
      )}
    </div>
  );
}
