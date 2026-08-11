"use client";

import { useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { getLessonVideoUrlAction, saveLessonProgressAction } from "@/lib/actions";

// Progress is persisted at most this often while playing, plus on pause and
// on ended — the backend is authoritative either way, this just keeps write
// volume sane (see video-platform skill: "without sending an excessive
// number of writes"). This player only ever forwards currentTime/ended
// events to that existing endpoint — it does not reimplement any
// completion/threshold logic of its own (see internal/learning).
const SAVE_INTERVAL_MS = 12_000;

// While the backend reports "processing", re-check on this interval — the
// worker typically finishes a short lesson video in well under a minute, so
// this stays a plain poll rather than needing a websocket.
const PROCESSING_POLL_MS = 5_000;

type PlayerState =
  | { kind: "loading" }
  | { kind: "processing" }
  | { kind: "failed"; message: string }
  | { kind: "unavailable"; message: string }
  | { kind: "ready"; url: string; streamType: "hls" | "mp4" };

export function LessonVideoPlayer({
  lessonId,
  initialProgressSeconds,
}: {
  lessonId: string;
  initialProgressSeconds: number;
}) {
  const router = useRouter();
  const videoRef = useRef<HTMLVideoElement>(null);
  const lastSaveAtRef = useRef(0);
  const restoredRef = useRef(false);

  const [state, setState] = useState<PlayerState>({ kind: "loading" });
  const [playbackError, setPlaybackError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    let pollTimer: ReturnType<typeof setTimeout> | undefined;

    async function poll() {
      const result = await getLessonVideoUrlAction(lessonId);
      if (cancelled) return;

      if (result.error) {
        setState({ kind: "unavailable", message: result.error });
        return;
      }
      if (result.status === "processing") {
        setState({ kind: "processing" });
        pollTimer = setTimeout(poll, PROCESSING_POLL_MS);
        return;
      }
      if (result.status === "failed") {
        setState({ kind: "failed", message: "Не удалось обработать видео. Обратитесь к администратору." });
        return;
      }
      if (result.status === "ready" && result.url && result.streamType) {
        setState({ kind: "ready", url: result.url, streamType: result.streamType });
        return;
      }
      setState({ kind: "unavailable", message: "Видео недоступно" });
    }

    void poll();
    return () => {
      cancelled = true;
      if (pollTimer) clearTimeout(pollTimer);
    };
  }, [lessonId]);

  // Attach the video source: native <video src> for legacy MP4, hls.js (or
  // native HLS on Safari) for HLS — see canPlayType check below.
  useEffect(() => {
    if (state.kind !== "ready") return;
    const video = videoRef.current;
    if (!video) return;

    if (state.streamType === "mp4") {
      video.src = state.url;
      return;
    }

    if (video.canPlayType("application/vnd.apple.mpegurl")) {
      // Safari (and some other WebKit browsers) play HLS natively.
      video.src = state.url;
      return;
    }

    let hls: import("hls.js").default | undefined;
    let destroyed = false;

    import("hls.js").then(({ default: Hls }) => {
      if (destroyed) return;
      if (!Hls.isSupported()) {
        setPlaybackError("Этот браузер не поддерживает воспроизведение видео (HLS).");
        return;
      }
      hls = new Hls();
      hls.loadSource(state.url);
      hls.attachMedia(video);
      hls.on(Hls.Events.ERROR, (_event, data) => {
        if (data.fatal) {
          setPlaybackError("Ошибка воспроизведения видео.");
        }
      });
    });

    return () => {
      destroyed = true;
      hls?.destroy();
    };
  }, [state]);

  function handleLoadedMetadata() {
    const video = videoRef.current;
    if (!video || restoredRef.current) return;
    restoredRef.current = true;
    if (initialProgressSeconds > 0 && initialProgressSeconds < video.duration) {
      video.currentTime = initialProgressSeconds;
    }
  }

  async function persist(progressSeconds: number, markCompleted: boolean) {
    const result = await saveLessonProgressAction(lessonId, progressSeconds, markCompleted);
    if (result.completed) {
      router.refresh(); // re-render the server page so sidebar checkmarks / progress bar update
    }
  }

  function handleTimeUpdate() {
    const video = videoRef.current;
    if (!video) return;
    const now = Date.now();
    if (now - lastSaveAtRef.current >= SAVE_INTERVAL_MS) {
      lastSaveAtRef.current = now;
      void persist(video.currentTime, false);
    }
  }

  function handlePause() {
    const video = videoRef.current;
    if (!video) return;
    lastSaveAtRef.current = Date.now();
    void persist(video.currentTime, false);
  }

  function handleEnded() {
    const video = videoRef.current;
    if (!video) return;
    void persist(video.duration || video.currentTime, true);
  }

  switch (state.kind) {
    case "loading":
      return <div className="video-placeholder">Загрузка видео…</div>;
    case "processing":
      return <div className="video-placeholder">Видео обрабатывается, попробуйте через минуту…</div>;
    case "failed":
      return (
        <div className="video-placeholder" role="alert">
          {state.message}
        </div>
      );
    case "unavailable":
      return (
        <div className="video-placeholder" role="alert">
          {state.message}
        </div>
      );
  }

  return (
    <div>
      <video
        ref={videoRef}
        controls
        className="lesson-video"
        onLoadedMetadata={handleLoadedMetadata}
        onTimeUpdate={handleTimeUpdate}
        onPause={handlePause}
        onEnded={handleEnded}
        onError={() => setPlaybackError("Ошибка воспроизведения видео.")}
      />
      {playbackError && (
        <p role="alert" className="my-course-meta">
          {playbackError}
        </p>
      )}
    </div>
  );
}
