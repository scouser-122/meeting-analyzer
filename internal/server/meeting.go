package server

import (
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"slices"

	"github.com/scouser-122/meeting-analyzer/internal/config"
	"github.com/scouser-122/meeting-analyzer/internal/domain/model"
	"github.com/scouser-122/meeting-analyzer/internal/logger"
	"github.com/scouser-122/meeting-analyzer/internal/models"
	"github.com/scouser-122/meeting-analyzer/internal/service"
	"github.com/scouser-122/meeting-analyzer/internal/worker"
)

// MeetingsHandler specifies http request handler for requests to Meetings service
type MeetingsHandler struct {
	meetingsService      *service.MeetingsService
	tasksService         *service.TasksService
	transcriptionService *service.TranscriptionService
	summaryService       *service.SummaryService
	meetingProcessor     *worker.MeetingProcessor
	maxUploadSize        int64
}

// NewMeetingsHandler creates and returns pointer to new MeetingsHandler
func NewMeetingsHandler(
	meetingsService *service.MeetingsService,
	tasksService *service.TasksService,
	transcriptionService *service.TranscriptionService,
	summaryService *service.SummaryService,
	meetingProcessor *worker.MeetingProcessor,
	serverConfig *config.ServerConfig,
) *MeetingsHandler {
	return &MeetingsHandler{
		meetingsService:      meetingsService,
		tasksService:         tasksService,
		transcriptionService: transcriptionService,
		summaryService:       summaryService,
		meetingProcessor:     meetingProcessor,
		maxUploadSize:        *serverConfig.MaxUploadSize,
	}
}

// HandleLoad processes meetings load request
func (h *MeetingsHandler) HandleLoad(res http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		res.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	logger := logger.GetSlogLoggerFromContext(req.Context())

	res.Header().Set("Content-Type", "application/json")

	// Cap total request size before we touch anything
	req.Body = http.MaxBytesReader(res, req.Body, h.maxUploadSize)

	// 32MB is the in-memory threshold for form parsing; larger parts
	// still stream to a temp file, this just controls the buffer.
	if err := req.ParseMultipartForm(32 << 20); err != nil {
		if errors.Is(err, io.EOF) {
			logger.Error("file too large", "err", err)
			res.WriteHeader(http.StatusRequestEntityTooLarge)
			res.Write(models.NewErrorResponseBuffer("file too large"))
			return
		}
		logger.Error("invalid multipart form", "err", err)
		res.WriteHeader(http.StatusBadRequest)
		res.Write(models.NewErrorResponseBuffer("invalid multipart form"))
		return
	}

	file, header, err := req.FormFile("file")
	if err != nil {
		logger.Error("missing 'file' field", "err", err)
		res.WriteHeader(http.StatusBadRequest)
		res.Write(models.NewErrorResponseBuffer("missing 'file' field"))
		return
	}
	defer file.Close()

	meetingData := model.Meeting{}
	if raw := req.FormValue("metadata"); raw != "" {
		if err = json.Unmarshal([]byte(raw), &meetingData); err != nil {
			logger.Error("invalid metadata JSON", "err", err)
			res.WriteHeader(http.StatusBadRequest)
			res.Write(models.NewErrorResponseBuffer("invalid metadata JSON"))
			return
		}
	}

	meeting, err := h.meetingsService.CreateFromAudioFile(req.Context(), &meetingData, file, header)
	if err != nil {
		handleServiceError(err, res)
		return
	}

	if !h.meetingProcessor.ProcessMeeting(meeting) {
		logger.Error("processing channel full, task rejected")
		res.WriteHeader(http.StatusTooManyRequests)
		res.Write(models.NewErrorResponseBuffer("can't upload meeting. try again later"))
		return
	}

	logger.Info("meeting file upload succesfully, and sent to processing queue", slog.String("meetingID", meeting.ID))
	res.WriteHeader(http.StatusAccepted)
	res.Write(models.NewSuccessResponseBufferWithData("Файл с записью встречи успешно загружен и запущена его обработка", meeting))
}

// HandleList processes meetings list request
func (h *MeetingsHandler) HandleList(res http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		res.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	logger := logger.GetSlogLoggerFromContext(req.Context())

	res.Header().Set("Content-Type", "application/json")

	userID := req.URL.Query().Get("user_id")

	meetings, err := h.meetingsService.GetAllByUserID(req.Context(), userID)
	if err != nil {
		handleServiceError(err, res)
		return
	}

	meetingsData := make([]models.MeetingResponseData, len(meetings))
	for i := 0; i < len(meetings); i++ {
		meeting := meetings[i]
		status, err := h.tasksService.GetStatus(req.Context(), meeting.ID)
		if err != nil {
			handleServiceError(err, res)
			return
		}
		summary, err := h.summaryService.GetSummary(req.Context(), meeting.ID)
		if err != nil {
			handleServiceError(err, res)
			return
		}
		meetingsData[i] = models.MeetingResponseData{
			ID:        meeting.ID,
			Name:      meeting.MeetingName,
			CreatedAt: meeting.CreatedAt,
			Status:    string(status),
			Summary:   summary,
		}
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	if err := enc.Encode(meetingsData); err != nil {
		logger.Error("error encoding response ", "err", err)
		res.WriteHeader(http.StatusInternalServerError)
		res.Write(models.NewErrorResponseBuffer(models.UnexpectedErrorMessage))
		return
	}

	logger.Info("meetings list successfully obtained")
	res.WriteHeader(http.StatusOK)
	res.Write(buf.Bytes())
}

// HandleList processes meetings status request
func (h *MeetingsHandler) HandleStatus(res http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		res.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	logger := logger.GetSlogLoggerFromContext(req.Context())

	res.Header().Set("Content-Type", "application/json")

	meetingID := req.URL.Query().Get("meeting_id")
	meeting, err := h.meetingsService.GetByID(req.Context(), meetingID)
	if err != nil {
		handleServiceError(err, res)
		return
	}

	userID := req.URL.Query().Get("user_id")
	if meeting.UserID != userID {
		res.WriteHeader(http.StatusForbidden)
		res.Write(models.NewErrorResponseBuffer("meeting data belongs to another user"))
		return
	}

	task, err := h.tasksService.GetByMeetingID(req.Context(), meeting.ID)
	if err != nil {
		handleServiceError(err, res)
		return
	}
	meetingData := models.MeetingResponseData{
		ID:                  meeting.ID,
		Name:                meeting.MeetingName,
		CreatedAt:           meeting.CreatedAt,
		Status:              string(task.Status),
		UpdatedAt:           task.UpdatedAt,
		ProcessErrorMessage: task.ErrorMessage,
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	if err := enc.Encode(meetingData); err != nil {
		logger.Error("error encoding response ", "err", err)
		res.WriteHeader(http.StatusInternalServerError)
		res.Write(models.NewErrorResponseBuffer(models.UnexpectedErrorMessage))
		return
	}

	logger.Info("meeting status successfully obtained")
	res.WriteHeader(http.StatusOK)
	res.Write(buf.Bytes())
}

// HandleTranscription processes get transcription text request
func (h *MeetingsHandler) HandleTranscription(res http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		res.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	logger := logger.GetSlogLoggerFromContext(req.Context())

	res.Header().Set("Content-Type", "application/json")

	meetingID := req.URL.Query().Get("meeting_id")
	meeting, err := h.meetingsService.GetByID(req.Context(), meetingID)
	if err != nil {
		handleServiceError(err, res)
		return
	}

	userID := req.URL.Query().Get("user_id")
	if meeting.UserID != userID {
		res.WriteHeader(http.StatusForbidden)
		res.Write(models.NewErrorResponseBuffer("meeting data belongs to another user"))
		return
	}

	transcription, err := h.transcriptionService.GetByMeetingID(req.Context(), meeting.ID)
	if err != nil {
		handleServiceError(err, res)
		return
	}
	meetingData := models.TranscriptionResponseData{
		Text: transcription.Text,
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	if err := enc.Encode(meetingData); err != nil {
		logger.Error("error encoding response ", "err", err)
		res.WriteHeader(http.StatusInternalServerError)
		res.Write(models.NewErrorResponseBuffer(models.UnexpectedErrorMessage))
		return
	}

	logger.Info("transcription text successfully obtained")
	res.WriteHeader(http.StatusOK)
	res.Write(buf.Bytes())
}

// HandleFind processes meetings find request
func (h *MeetingsHandler) HandleFind(res http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		res.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	logger := logger.GetSlogLoggerFromContext(req.Context())

	res.Header().Set("Content-Type", "application/json")

	bodyBuf, err := io.ReadAll(req.Body)
	if err != nil {
		logger.Error("cannot read request body", "err", err)
		res.WriteHeader(http.StatusBadRequest)
		res.Write(models.NewErrorResponseBuffer(models.UnexpectedErrorMessage))
		return
	}

	var request models.FindMeetingRequest
	if err = json.Unmarshal(bodyBuf, &request); err != nil {
		logger.Error("cannot decode request json body", "err", err)
		res.WriteHeader(http.StatusBadRequest)
		res.Write(models.NewErrorResponseBuffer(models.UnexpectedErrorMessage))
		return
	}

	meetings, err := h.meetingsService.FindByNameContains(req.Context(), request.UserID, request.KeyWords)
	if err != nil {
		handleServiceError(err, res)
		return
	}

	transcriptions, err := h.transcriptionService.FindByTextContains(req.Context(), request.UserID, request.KeyWords)
	if err != nil {
		handleServiceError(err, res)
		return
	}

	for _, t := range transcriptions {
		if slices.ContainsFunc(meetings, func(m *model.Meeting) bool { return m.ID == t.MeetingID }) {
			continue
		}
		var meeting *model.Meeting
		meeting, err = h.meetingsService.GetByID(req.Context(), t.MeetingID)
		if err != nil {
			handleServiceError(err, res)
			return
		}
		meetings = append(meetings, meeting)
	}

	summaries, err := h.summaryService.FindByTextContains(req.Context(), request.UserID, request.KeyWords)
	if err != nil {
		handleServiceError(err, res)
		return
	}

	for _, s := range summaries {
		if slices.ContainsFunc(meetings, func(m *model.Meeting) bool { return m.ID == s.MeetingID }) {
			continue
		}
		meeting, err := h.meetingsService.GetByID(req.Context(), s.MeetingID)
		if err != nil {
			handleServiceError(err, res)
			return
		}
		meetings = append(meetings, meeting)
	}

	slices.SortFunc(meetings, func(a *model.Meeting, b *model.Meeting) int {
		return cmp.Compare(a.CreatedAt.UnixMilli(), b.CreatedAt.UnixMilli())
	})

	meetingsData := []models.MeetingResponseData{}
	for i := 0; i < len(meetings); i++ {
		meeting := meetings[i]
		status, err := h.tasksService.GetStatus(req.Context(), meeting.ID)
		if err != nil {
			handleServiceError(err, res)
			return
		}
		if status != model.TaskStatusCompleted {
			continue
		}
		summary, err := h.summaryService.GetSummary(req.Context(), meeting.ID)
		if err != nil {
			handleServiceError(err, res)
			return
		}
		meetingsData = append(meetingsData, models.MeetingResponseData{
			ID:        meeting.ID,
			Name:      meeting.MeetingName,
			CreatedAt: meeting.CreatedAt,
			Status:    string(status),
			Summary:   summary,
		})
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	if err := enc.Encode(meetingsData); err != nil {
		logger.Error("error encoding response ", "err", err)
		res.WriteHeader(http.StatusInternalServerError)
		res.Write(models.NewErrorResponseBuffer(models.UnexpectedErrorMessage))
		return
	}

	logger.Info("meetings list successfully obtained")
	res.WriteHeader(http.StatusOK)
	res.Write(buf.Bytes())
}
