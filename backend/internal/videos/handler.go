package videos

import (
	"errors"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"lms-backend/internal/authctx"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes wires the student-facing playback endpoint and the
// authorized HLS proxy. Neither ever exposes storage credentials or a raw
// bucket URL — the first hands out a same-origin proxy path, the second is
// that path.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup, requireAuth gin.HandlerFunc) {
	rg.GET("/lessons/:id/video", requireAuth, h.GetPlayback)
	rg.GET("/video-stream/:videoId/*proxyPath", requireAuth, h.StreamVideo)
}

// RegisterAdminRoutes wires upload/replace, delete, a metadata lookup, and
// the processing-status poll used by the Admin CMS lesson editor.
func (h *Handler) RegisterAdminRoutes(admin *gin.RouterGroup) {
	admin.POST("/lessons/:id/video", h.UploadVideo)
	admin.GET("/lessons/:id/video", h.GetVideoMetadata)
	admin.GET("/lessons/:id/video/status", h.GetProcessingStatus)
	admin.DELETE("/lessons/:id/video", h.DeleteVideo)
}

func respondError(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{"error": gin.H{"code": code, "message": message}})
}

func parseLessonID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_LESSON_ID", "lesson id must be a valid UUID")
		return uuid.UUID{}, false
	}
	return id, true
}

func (h *Handler) UploadVideo(c *gin.Context) {
	lessonID, ok := parseLessonID(c)
	if !ok {
		return
	}

	// Cap the raw request body before multipart parsing even begins — a
	// client can't burn disk/memory past the configured limit just by
	// declaring a small Content-Length and streaming more, since this
	// applies to actual bytes read. The 1MB pad covers multipart boundary
	// overhead and non-file form fields, not the video itself.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, h.service.MaxUploadBytes()+1<<20)

	fileHeader, err := c.FormFile("video")
	if err != nil {
		if errors.Is(err, http.ErrMissingFile) {
			respondError(c, http.StatusBadRequest, "INVALID_BODY", "a \"video\" file field is required")
			return
		}
		// Any other failure this early is almost certainly the
		// MaxBytesReader cutoff kicking in mid-parse.
		respondError(c, http.StatusRequestEntityTooLarge, "VIDEO_TOO_LARGE", "video file exceeds the maximum upload size")
		return
	}

	contentType := fileHeader.Header.Get("Content-Type")

	file, err := fileHeader.Open()
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to read uploaded file")
		return
	}
	defer file.Close()

	video, err := h.service.UploadVideo(c.Request.Context(), lessonID, UploadInput{
		Filename:     fileHeader.Filename,
		DeclaredMIME: contentType,
		SizeBytes:    fileHeader.Size,
		Body:         file,
	})

	switch {
	case err == nil:
		c.JSON(http.StatusCreated, video)
	case errors.Is(err, ErrLessonNotFound):
		respondError(c, http.StatusNotFound, "LESSON_NOT_FOUND", "lesson not found")
	case errors.Is(err, ErrTooLarge):
		respondError(c, http.StatusRequestEntityTooLarge, "VIDEO_TOO_LARGE", "video file exceeds the maximum upload size")
	case errors.Is(err, ErrInvalidContentType):
		respondError(c, http.StatusUnprocessableEntity, "INVALID_VIDEO_TYPE", "only video/mp4 and video/webm are accepted")
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to upload video")
	}
}

func (h *Handler) GetVideoMetadata(c *gin.Context) {
	lessonID, ok := parseLessonID(c)
	if !ok {
		return
	}

	video, err := h.service.GetVideoMetadata(c.Request.Context(), lessonID)
	switch {
	case err == nil:
		c.JSON(http.StatusOK, video)
	case errors.Is(err, ErrVideoNotFound):
		respondError(c, http.StatusNotFound, "VIDEO_NOT_FOUND", "this lesson has no video")
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get video metadata")
	}
}

func (h *Handler) GetProcessingStatus(c *gin.Context) {
	lessonID, ok := parseLessonID(c)
	if !ok {
		return
	}

	status, err := h.service.GetProcessingStatus(c.Request.Context(), lessonID)
	switch {
	case err == nil:
		c.JSON(http.StatusOK, status)
	case errors.Is(err, ErrVideoNotFound):
		respondError(c, http.StatusNotFound, "VIDEO_NOT_FOUND", "this lesson has no video")
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get processing status")
	}
}

func (h *Handler) DeleteVideo(c *gin.Context) {
	lessonID, ok := parseLessonID(c)
	if !ok {
		return
	}

	err := h.service.DeleteVideo(c.Request.Context(), lessonID)
	switch {
	case err == nil:
		c.Status(http.StatusNoContent)
	case errors.Is(err, ErrVideoNotFound):
		respondError(c, http.StatusNotFound, "VIDEO_NOT_FOUND", "this lesson has no video")
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to delete video")
	}
}

func (h *Handler) GetPlayback(c *gin.Context) {
	userID, ok := authctx.UserID(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid authorization header")
		return
	}
	lessonID, ok := parseLessonID(c)
	if !ok {
		return
	}

	playback, err := h.service.GetPlayback(c.Request.Context(), userID, lessonID)
	switch {
	case err == nil:
		c.JSON(http.StatusOK, playback)
	case errors.Is(err, ErrLessonNotFound):
		respondError(c, http.StatusNotFound, "LESSON_NOT_FOUND", "lesson not found")
	case errors.Is(err, ErrNotEnrolled):
		respondError(c, http.StatusForbidden, "NOT_ENROLLED", "you are not enrolled in the course this lesson belongs to")
	case errors.Is(err, ErrAccessRequired):
		respondError(c, http.StatusForbidden, "COURSE_ACCESS_REQUIRED", "an active subscription is required to access this video")
	case errors.Is(err, ErrVideoNotFound):
		respondError(c, http.StatusNotFound, "VIDEO_NOT_FOUND", "this lesson has no video yet")
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get video")
	}
}

// hlsPathPattern is an allow-list, not a blacklist: only these exact shapes
// are ever turned into a storage key, so "../", absolute paths, null bytes,
// or any other traversal trick simply never matches and is rejected before
// a single byte of the path reaches the object key builder.
var hlsPathPattern = regexp.MustCompile(`^(master\.m3u8|(360p|720p|1080p)/(index\.m3u8|segment_\d{5}\.ts))$`)

// StreamVideo is the backend-authorized HLS proxy (item 12): every playlist
// and segment request is independently re-authorized (enrollment + access
// policy) against the course the requested videoId actually belongs to —
// not whatever course the caller claims — and the object key is built
// entirely from the validated videoId and an allow-listed relative path, so
// a client can never supply an arbitrary bucket key.
func (h *Handler) StreamVideo(c *gin.Context) {
	userID, ok := authctx.UserID(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid authorization header")
		return
	}

	videoID, err := uuid.Parse(c.Param("videoId"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_VIDEO_ID", "video id must be a valid UUID")
		return
	}

	relPath := strings.TrimPrefix(c.Param("proxyPath"), "/")
	if !hlsPathPattern.MatchString(relPath) {
		respondError(c, http.StatusBadRequest, "INVALID_PATH", "invalid HLS resource path")
		return
	}

	video, err := h.service.AuthorizeStream(c.Request.Context(), userID, videoID)
	switch {
	case err == nil:
		// fall through to streaming below
	case errors.Is(err, ErrVideoNotFound), errors.Is(err, ErrLessonNotFound):
		respondError(c, http.StatusNotFound, "VIDEO_NOT_FOUND", "video not found")
		return
	case errors.Is(err, ErrVideoNotReady):
		respondError(c, http.StatusConflict, "VIDEO_NOT_READY", "this video is not ready yet")
		return
	case errors.Is(err, ErrNotEnrolled):
		respondError(c, http.StatusForbidden, "NOT_ENROLLED", "you are not enrolled in the course this video belongs to")
		return
	case errors.Is(err, ErrAccessRequired):
		respondError(c, http.StatusForbidden, "COURSE_ACCESS_REQUIRED", "an active subscription is required to access this video")
		return
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to authorize video")
		return
	}

	objectKey := videoPrefix(video.ID) + "hls/" + relPath

	body, size, err := h.service.StreamObject(c.Request.Context(), objectKey)
	if err != nil {
		respondError(c, http.StatusNotFound, "VIDEO_NOT_FOUND", "video resource not found")
		return
	}
	defer body.Close()

	c.Header("Content-Type", contentTypeFor(relPath))
	if size > 0 {
		c.Header("Content-Length", strconv.FormatInt(size, 10))
	}
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, body)
}

func contentTypeFor(relPath string) string {
	switch {
	case strings.HasSuffix(relPath, ".m3u8"):
		return "application/vnd.apple.mpegurl"
	case strings.HasSuffix(relPath, ".ts"):
		return "video/mp2t"
	default:
		return "application/octet-stream"
	}
}
