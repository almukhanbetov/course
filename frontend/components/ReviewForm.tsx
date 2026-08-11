"use client";

import { useActionState } from "react";
import { createReviewAction, deleteReviewAction, updateReviewAction } from "@/lib/actions";
import type { FormState } from "@/lib/actions";
import type { MyReview } from "@/lib/api";
import { ConfirmButton } from "@/components/ConfirmButton";

const initialState: FormState = { error: null };

export function ReviewForm({ courseId, myReview }: { courseId: string; myReview: MyReview | null }) {
  const action = myReview ? updateReviewAction.bind(null, courseId) : createReviewAction.bind(null, courseId);
  const [state, formAction, pending] = useActionState(action, initialState);

  return (
    <div className="review-item">
      <h3>{myReview ? "Ваш отзыв" : "Оставить отзыв"}</h3>
      <form action={formAction} className="admin-form">
        <label>
          Оценка
          <select name="rating" defaultValue={myReview?.rating ?? 5}>
            <option value={5}>5 — отлично</option>
            <option value={4}>4 — хорошо</option>
            <option value={3}>3 — нормально</option>
            <option value={2}>2 — плохо</option>
            <option value={1}>1 — очень плохо</option>
          </select>
        </label>
        <label>
          Комментарий
          <textarea name="review_text" rows={3} defaultValue={myReview?.review_text} />
        </label>
        {state.error && <p role="alert">{state.error}</p>}
        <button type="submit" className="btn-primary" disabled={pending}>
          {pending ? "Сохранение..." : myReview ? "Сохранить" : "Отправить отзыв"}
        </button>
      </form>
      {myReview && (
        <form action={deleteReviewAction.bind(null, courseId)} className="mt-3">
          <ConfirmButton className="btn-danger" confirmMessage="Удалить ваш отзыв?">
            Удалить отзыв
          </ConfirmButton>
        </form>
      )}
    </div>
  );
}
