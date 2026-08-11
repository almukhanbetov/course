"use client";

import { useActionState } from "react";
import { uploadInstructorLessonVideoAction, deleteInstructorLessonVideoAction } from "@/lib/instructor-actions";
import { ConfirmButton } from "@/components/ConfirmButton";
import { InstructorVideoProcessingStatusPanel } from "@/components/instructor/InstructorVideoProcessingStatusPanel";
import type { FormState } from "@/lib/actions";
import type { AdminLessonVideo } from "@/lib/admin-api";

const initialState: FormState = { error: null };

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

export function InstructorLessonVideoUpload({
  courseId,
  lessonId,
  video,
}: {
  courseId: string;
  lessonId: string;
  video: AdminLessonVideo | null;
}) {
  const action = uploadInstructorLessonVideoAction.bind(null, courseId, lessonId);
  const [state, formAction, pending] = useActionState(action, initialState);

  return (
    <div className="video-upload-block">
      {video ? (
        <div className="admin-inline-actions">
          <span>
            🎬 {video.original_filename} — {formatBytes(video.size_bytes)} — <span className="badge">{video.status}</span>
          </span>
          <form action={deleteInstructorLessonVideoAction.bind(null, courseId, lessonId)}>
            <ConfirmButton className="btn-danger" confirmMessage="Удалить видео этого урока?">
              Удалить видео
            </ConfirmButton>
          </form>
        </div>
      ) : (
        <p className="my-course-meta">Видео ещё не загружено.</p>
      )}

      <InstructorVideoProcessingStatusPanel lessonId={lessonId} />

      <form action={formAction} className="admin-inline-actions my-2">
        <input type="file" name="video" accept="video/mp4,video/webm" required />
        <button type="submit" className="btn-small" disabled={pending}>
          {pending ? "Загрузка…" : video ? "Заменить видео" : "Загрузить видео"}
        </button>
      </form>
      {state.error && <p role="alert">{state.error}</p>}
    </div>
  );
}
