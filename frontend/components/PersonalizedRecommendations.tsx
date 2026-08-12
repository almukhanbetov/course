"use client";

import { useState, useTransition } from "react";
import type { Recommendation } from "@/lib/api";
import { submitRecommendationFeedbackAction } from "@/lib/actions";
import { RecommendationCard } from "@/components/RecommendationCard";

// Owns the /dashboard "Рекомендуем вам" grid's own list state so a
// successful dismiss/not_interested can remove that card immediately, with
// no full page reload — RecommendationCard itself stays a presentational
// component either way, just rendered with feedback callbacks here that it
// doesn't get when /courses/[id] renders it directly for public similar
// courses (see RecommendationCard's own doc comment).
export function PersonalizedRecommendations({ initialRecommendations }: { initialRecommendations: Recommendation[] }) {
  const [recommendations, setRecommendations] = useState(initialRecommendations);
  const [pendingId, setPendingId] = useState<string | null>(null);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [, startTransition] = useTransition();

  function handleFeedback(courseId: string, action: "dismiss" | "not_interested") {
    setErrors((prev) => ({ ...prev, [courseId]: "" }));
    setPendingId(courseId);
    startTransition(async () => {
      const result = await submitRecommendationFeedbackAction(courseId, action);
      setPendingId(null);
      if (!result.ok) {
        setErrors((prev) => ({ ...prev, [courseId]: result.error ?? "Не удалось сохранить отметку" }));
        return;
      }
      // Success: remove immediately, no undo affordance yet (out of scope
      // for this session — see STAGE23_PROGRESS.md).
      setRecommendations((prev) => prev.filter((r) => r.course_id !== courseId));
    });
  }

  if (recommendations.length === 0) {
    return null;
  }

  return (
    <div className="course-grid">
      {recommendations.map((rec) => (
        <RecommendationCard
          key={rec.course_id}
          rec={rec}
          feedback={{
            pending: pendingId === rec.course_id,
            error: errors[rec.course_id],
            onDismiss: () => handleFeedback(rec.course_id, "dismiss"),
            onNotInterested: () => handleFeedback(rec.course_id, "not_interested"),
          }}
        />
      ))}
    </div>
  );
}
