package videos

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// renditionPreset is a candidate HLS quality tier. Presets are only ever
// used when preset.Height <= the probed source height — never upscaled
// (item 9/27): a 720p source yields 360p+720p, never 1080p.
type renditionPreset struct {
	Quality   string
	Height    int
	VideoKbps int
	AudioKbps int
}

var renditionPresets = []renditionPreset{
	{"360p", 360, 800, 96},
	{"720p", 720, 2500, 128},
	{"1080p", 1080, 5000, 160},
}

type renditionSpec struct {
	Quality              string
	Width, Height        int
	VideoKbps, AudioKbps int
}

type WorkerConfig struct {
	MaxAttempts        int
	SegmentDurationSec int
	RetryBackoff       time.Duration
}

// Worker holds everything cmd/video-worker needs to claim and process one
// transcoding job at a time. It never serves HTTP — see main.go for the
// tiny internal-only health listener, which is separate from this type.
type Worker struct {
	repo    *Repository
	storage VideoStorage
	cfg     WorkerConfig
	log     *slog.Logger
}

// slog.With, not a package-level var: this runs at NewWorker call time
// (after cmd/video-worker's main() has already called logging.Init()), so
// it correctly captures the JSON-configured default logger — a
// package-level var would instead capture whatever slog.Default() was
// *before* Init() ever ran.
func NewWorker(repo *Repository, storage VideoStorage, cfg WorkerConfig) *Worker {
	return &Worker{repo: repo, storage: storage, cfg: cfg, log: slog.With("service", "video-worker")}
}

// ClaimAndProcess claims at most one pending job and processes it fully
// (or records its failure/retry) before returning. It returns claimed=false
// when the queue was empty, so the caller's loop knows to sleep before
// polling again instead of busy-looping.
func (w *Worker) ClaimAndProcess(ctx context.Context) (claimed bool, err error) {
	job, err := w.repo.ClaimNextJob(ctx)
	if errors.Is(err, ErrNoJobAvailable) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	w.log.Info("claimed job", "job_id", job.ID, "video_id", job.LessonVideoID, "attempt", job.Attempts)

	if procErr := w.processJob(ctx, job); procErr != nil {
		w.log.Error("job failed", "job_id", job.ID, "video_id", job.LessonVideoID, "error", procErr)

		retried, markErr := w.repo.MarkJobFailedOrRetry(ctx, job.ID, job.Attempts, w.cfg.MaxAttempts, truncateError(procErr), w.cfg.RetryBackoff)
		if markErr != nil {
			return true, markErr
		}
		if retried {
			w.log.Warn("job will retry", "job_id", job.ID, "attempt", job.Attempts, "max_attempts", w.cfg.MaxAttempts)
		} else {
			w.log.Error("job exhausted retries, marking video failed", "job_id", job.ID, "video_id", job.LessonVideoID)
			if err := w.repo.MarkVideoFailed(ctx, job.LessonVideoID, humanReadableError(procErr)); err != nil {
				return true, err
			}
		}
		return true, nil
	}

	if err := w.repo.MarkJobCompleted(ctx, job.ID); err != nil {
		return true, err
	}
	w.log.Info("job completed", "job_id", job.ID, "video_id", job.LessonVideoID)
	return true, nil
}

func (w *Worker) processJob(ctx context.Context, job *VideoJob) error {
	video, err := w.repo.GetByID(ctx, job.LessonVideoID)
	if err != nil {
		return fmt.Errorf("load video: %w", err)
	}

	if err := w.repo.MarkVideoProcessing(ctx, video.ID); err != nil {
		return fmt.Errorf("mark processing: %w", err)
	}

	sourcePath, err := w.downloadSource(ctx, video.SourceObjectKey)
	if err != nil {
		return fmt.Errorf("download source: %w", err)
	}
	defer os.Remove(sourcePath)

	probe, err := probeVideo(ctx, sourcePath)
	if err != nil {
		return fmt.Errorf("probe source: %w", err)
	}

	renditions := selectRenditions(probe.Width, probe.Height)
	if len(renditions) == 0 {
		return errors.New("source resolution too small to transcode")
	}

	for _, r := range renditions {
		if err := w.repo.UpsertRendition(ctx, video.ID, r.Quality, r.Width, r.Height, r.VideoKbps, RenditionProcessing, nil); err != nil {
			return fmt.Errorf("record rendition %s: %w", r.Quality, err)
		}

		playlistKey, err := w.encodeRendition(ctx, video.ID, sourcePath, r, probe.FPS)
		if err != nil {
			_ = w.repo.UpsertRendition(ctx, video.ID, r.Quality, r.Width, r.Height, r.VideoKbps, RenditionFailed, nil)
			return fmt.Errorf("encode %s: %w", r.Quality, err)
		}

		if err := w.repo.UpsertRendition(ctx, video.ID, r.Quality, r.Width, r.Height, r.VideoKbps, RenditionReady, &playlistKey); err != nil {
			return fmt.Errorf("finalize rendition %s: %w", r.Quality, err)
		}
		w.log.Info("rendition ready", "video_id", video.ID, "quality", r.Quality)
	}

	masterKey, err := w.buildAndUploadMaster(ctx, video.ID, renditions)
	if err != nil {
		return fmt.Errorf("build master playlist: %w", err)
	}

	replaced, err := w.repo.ActivateVideo(ctx, video.ID, masterKey)
	if err != nil {
		return fmt.Errorf("activate video: %w", err)
	}

	if replaced != nil {
		w.log.Info("video activated, superseding old video", "video_id", video.ID, "superseded_video_id", replaced.ID)
		// Cleanup happens strictly after activation already committed —
		// a failure here must never undo a successful switch-over
		// (item 21: cleanup can be safely left for a later sweep).
		if err := w.storage.DeletePrefix(ctx, videoPrefix(replaced.ID)); err != nil {
			w.log.Warn("cleanup of superseded video storage failed", "video_id", replaced.ID, "error", err)
		}
		if err := w.repo.DeleteVideoRowByID(ctx, replaced.ID); err != nil {
			w.log.Warn("cleanup of superseded video row failed", "video_id", replaced.ID, "error", err)
		}
	}

	return nil
}

func (w *Worker) downloadSource(ctx context.Context, key string) (string, error) {
	body, _, _, err := w.storage.GetObject(ctx, key)
	if err != nil {
		return "", err
	}
	defer body.Close()

	f, err := os.CreateTemp("", "video-source-*.mp4")
	if err != nil {
		return "", err
	}
	defer f.Close()

	if _, err := io.Copy(f, body); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

type probeResult struct {
	Width, Height int
	FPS           float64
}

// probeVideo shells out to ffprobe rather than parsing container formats by
// hand — see the "4. FFmpeg" requirement to verify ffprobe availability at
// image-build time; this is the runtime counterpart.
func probeVideo(ctx context.Context, path string) (*probeResult, error) {
	cmd := exec.CommandContext(ctx, "ffprobe", "-v", "error", "-select_streams", "v:0",
		"-show_entries", "stream=width,height,r_frame_rate", "-of", "json", path)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe: %w", err)
	}

	var parsed struct {
		Streams []struct {
			Width      int    `json:"width"`
			Height     int    `json:"height"`
			RFrameRate string `json:"r_frame_rate"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		return nil, fmt.Errorf("parse ffprobe output: %w", err)
	}
	if len(parsed.Streams) == 0 {
		return nil, errors.New("no video stream found in source")
	}

	s := parsed.Streams[0]
	if s.Width <= 0 || s.Height <= 0 {
		return nil, errors.New("could not determine source video resolution")
	}

	fps := 30.0
	if parts := strings.SplitN(s.RFrameRate, "/", 2); len(parts) == 2 {
		num, err1 := strconv.ParseFloat(parts[0], 64)
		den, err2 := strconv.ParseFloat(parts[1], 64)
		if err1 == nil && err2 == nil && den != 0 && num/den > 0 {
			fps = num / den
		}
	}

	return &probeResult{Width: s.Width, Height: s.Height, FPS: fps}, nil
}

// selectRenditions picks every preset that fits without upscaling, scaling
// each rendition's width to preserve the source's own aspect ratio (rather
// than assuming 16:9). If the source is smaller than even the lowest
// preset, one custom rendition matching the source's own (even-clamped)
// dimensions is produced instead of producing nothing.
func selectRenditions(sourceWidth, sourceHeight int) []renditionSpec {
	var selected []renditionSpec
	for _, p := range renditionPresets {
		if p.Height > sourceHeight {
			continue
		}
		width := int(math.Round(float64(sourceWidth)*float64(p.Height)/float64(sourceHeight)/2)) * 2
		selected = append(selected, renditionSpec{
			Quality: p.Quality, Width: width, Height: p.Height,
			VideoKbps: p.VideoKbps, AudioKbps: p.AudioKbps,
		})
	}

	if len(selected) == 0 {
		w := sourceWidth - sourceWidth%2
		h := sourceHeight - sourceHeight%2
		if w < 2 || h < 2 {
			return nil
		}
		// The DB's quality CHECK constraint only allows the three labels
		// above; a source smaller than 360p still gets labeled "360p" even
		// though it's encoded at its own (smaller) native resolution — an
		// acceptable label/reality mismatch for what is expected to be a
		// rare edge case (e.g. a tiny test clip), not real course content.
		selected = append(selected, renditionSpec{Quality: "360p", Width: w, Height: h, VideoKbps: 500, AudioKbps: 96})
	}

	return selected
}

// encodeRendition runs one FFmpeg invocation producing a complete VOD HLS
// rendition (playlist + segments) in a scratch directory, then uploads
// every produced file to MinIO under videos/<id>/hls/<quality>/. GOP size
// is locked to segment-duration * fps with scene-cut detection disabled
// (-sc_threshold 0), so every segment boundary lands exactly on a keyframe —
// required for clean HLS playback and for switching renditions mid-stream.
func (w *Worker) encodeRendition(ctx context.Context, videoID uuid.UUID, sourcePath string, r renditionSpec, fps float64) (string, error) {
	tmpDir, err := os.MkdirTemp("", "hls-"+r.Quality+"-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	gop := int(math.Round(fps * float64(w.cfg.SegmentDurationSec)))
	if gop < 1 {
		gop = 48
	}

	playlistPath := filepath.Join(tmpDir, "index.m3u8")
	segmentPattern := filepath.Join(tmpDir, "segment_%05d.ts")

	args := []string{
		"-y", "-i", sourcePath,
		"-vf", fmt.Sprintf("scale=%d:%d", r.Width, r.Height),
		"-c:v", "libx264", "-profile:v", "main", "-preset", "veryfast", "-crf", "20",
		"-sc_threshold", "0", "-g", strconv.Itoa(gop), "-keyint_min", strconv.Itoa(gop),
		"-b:v", fmt.Sprintf("%dk", r.VideoKbps),
		"-maxrate", fmt.Sprintf("%dk", r.VideoKbps*107/100),
		"-bufsize", fmt.Sprintf("%dk", r.VideoKbps*150/100),
		"-c:a", "aac", "-ar", "48000", "-b:a", fmt.Sprintf("%dk", r.AudioKbps), "-ac", "2",
		"-hls_time", strconv.Itoa(w.cfg.SegmentDurationSec),
		"-hls_playlist_type", "vod",
		"-hls_segment_filename", segmentPattern,
		"-f", "hls", playlistPath,
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("ffmpeg: %w: %s", err, lastLines(stderr.String(), 15))
	}

	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		return "", err
	}

	prefix := videoPrefix(videoID) + "hls/" + r.Quality + "/"
	var playlistKey string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		localPath := filepath.Join(tmpDir, e.Name())
		f, err := os.Open(localPath)
		if err != nil {
			return "", err
		}

		info, statErr := e.Info()
		if statErr != nil {
			f.Close()
			return "", statErr
		}

		contentType := "video/mp2t"
		if strings.HasSuffix(e.Name(), ".m3u8") {
			contentType = "application/vnd.apple.mpegurl"
		}

		key := prefix + e.Name()
		putErr := w.storage.PutObject(ctx, key, f, info.Size(), contentType)
		f.Close()
		if putErr != nil {
			return "", fmt.Errorf("upload %s: %w", e.Name(), putErr)
		}

		if e.Name() == "index.m3u8" {
			playlistKey = key
		}
	}

	if playlistKey == "" {
		return "", errors.New("ffmpeg did not produce a playlist")
	}
	return playlistKey, nil
}

// buildAndUploadMaster writes a standard HLS master playlist listing every
// successfully-encoded rendition with its bandwidth/resolution, so the
// player (native HLS or hls.js) can switch between them automatically.
func (w *Worker) buildAndUploadMaster(ctx context.Context, videoID uuid.UUID, renditions []renditionSpec) (string, error) {
	var sb strings.Builder
	sb.WriteString("#EXTM3U\n#EXT-X-VERSION:3\n")
	for _, r := range renditions {
		bandwidth := (r.VideoKbps + r.AudioKbps) * 1000
		fmt.Fprintf(&sb, "#EXT-X-STREAM-INF:BANDWIDTH=%d,RESOLUTION=%dx%d\n%s/index.m3u8\n", bandwidth, r.Width, r.Height, r.Quality)
	}

	content := sb.String()
	key := videoPrefix(videoID) + "hls/master.m3u8"
	if err := w.storage.PutObject(ctx, key, strings.NewReader(content), int64(len(content)), "application/vnd.apple.mpegurl"); err != nil {
		return "", err
	}
	return key, nil
}

// truncateError is what gets stored in video_jobs.last_error — an
// operations-facing field never serialized by any API, so it can keep
// enough of the raw ffmpeg/ffprobe diagnostic to be useful without needing
// the same scrubbing as anything student- or even admin-facing.
func truncateError(err error) string {
	msg := err.Error()
	const max = 2000
	if len(msg) > max {
		return msg[:max]
	}
	return msg
}

// humanReadableError is what ends up in lesson_videos.processing_error,
// which IS exposed via the admin status API (item 18: "не показывай...
// весь raw FFmpeg stderr") — so it collapses everything down to one of a
// small number of short, non-technical messages.
func humanReadableError(err error) string {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "no video stream"), strings.Contains(msg, "ffprobe"), strings.Contains(msg, "Invalid data found"):
		return "the uploaded file could not be read as a valid video"
	case strings.Contains(msg, "too small to transcode"):
		return "the video's resolution is too low to process"
	default:
		return "video processing failed"
	}
}

func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, " | ")
}
