package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/scouser-122/meeting-analyzer/internal/app/models"
	"github.com/scouser-122/meeting-analyzer/internal/app/service"
	"github.com/scouser-122/meeting-analyzer/internal/app/worker"
	"github.com/scouser-122/meeting-analyzer/internal/config"
	"github.com/scouser-122/meeting-analyzer/internal/domain/model"
	"github.com/scouser-122/meeting-analyzer/internal/logger"
)

// MeetingsHandler specifies http request handler for requests to Meetings service
type MeetingsHandler struct {
	meetingsService  *service.MeetingsService
	tasksService     *service.TasksService
	summaryService   *service.SummaryService
	meetingProcessor *worker.MeetingProcessor
	maxUploadSize    int64
}

// NewMeetingsHandler creates and returns pointer to new MeetingsHandler
func NewMeetingsHandler(
	meetingsService *service.MeetingsService,
	tasksService *service.TasksService,
	summaryService *service.SummaryService,
	meetingProcessor *worker.MeetingProcessor,
	serverConfig *config.ServerConfig,
) *MeetingsHandler {
	return &MeetingsHandler{
		meetingsService:  meetingsService,
		tasksService:     tasksService,
		summaryService:   summaryService,
		meetingProcessor: meetingProcessor,
		maxUploadSize:    *serverConfig.MaxUploadSize,
	}
}

// HandleLoad processes meetings load request
func (h *MeetingsHandler) HandleLoad(res http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		res.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	logger := logger.GetSlogLoggerFromContext(req.Context())

	// Cap total request size before we touch anything
	req.Body = http.MaxBytesReader(res, req.Body, h.maxUploadSize)

	// 32MB is the in-memory threshold for form parsing; larger parts
	// still stream to a temp file, this just controls the buffer.
	if err := req.ParseMultipartForm(32 << 20); err != nil {
		if errors.Is(err, io.EOF) {
			http.Error(res, "file too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(res, "invalid multipart form", http.StatusBadRequest)
		return
	}

	file, header, err := req.FormFile("file")
	if err != nil {
		http.Error(res, "missing 'file' field", http.StatusBadRequest)
		return
	}
	defer file.Close()

	meetingData := model.Meeting{}
	if raw := req.FormValue("metadata"); raw != "" {
		if err := json.Unmarshal([]byte(raw), &meetingData); err != nil {
			http.Error(res, "invalid metadata JSON", http.StatusBadRequest)
			return
		}
	}

	meeting, err := h.meetingsService.Load(req.Context(), &meetingData, file, header)
	if err != nil {
		var customErr *models.CustomErr
		if errors.As(err, &customErr) {
			models.WriteResponseError(customErr, res)
			return
		} else {
			res.WriteHeader(http.StatusInternalServerError)
			res.Write(models.NewErrorResponseBuffer(models.UnexpectedErrorMessage))
			return
		}
	}

	h.meetingProcessor.Meetins <- meeting

	successMessage := "meeting file successfully uploaded"
	logger.Info(successMessage, slog.String("id", meeting.ID))
	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(http.StatusAccepted)
	res.Write(models.NewSuccessResponseBufferWithData(successMessage, meeting))
}

// HandleLoad processes meetings load request
func (h *MeetingsHandler) HandleList(res http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		res.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	logger := logger.GetSlogLoggerFromContext(req.Context())

	userID := req.URL.Query().Get("user_id")

	meetings, err := h.meetingsService.GetAllByUserID(req.Context(), userID)
	if err != nil {
		var customErr *models.CustomErr
		if errors.As(err, &customErr) {
			models.WriteResponseError(customErr, res)
			return
		} else {
			res.WriteHeader(http.StatusInternalServerError)
			res.Write(models.NewErrorResponseBuffer(models.UnexpectedErrorMessage))
			return
		}
	}

	meetingsData := make([]models.MeetingResponseData, len(meetings))
	for i := 0; i < len(meetings); i++ {
		meeting := meetings[i]
		status, err := h.tasksService.GetStatus(req.Context(), meeting.ID)
		if err != nil {
			logger.Error("error getting meeting status", "err", err)
			res.WriteHeader(http.StatusInternalServerError)
			res.Write(models.NewErrorResponseBuffer(models.UnexpectedErrorMessage))
		}
		summary, err := h.summaryService.GetSummary(req.Context(), meeting.ID)
		if err != nil {
			logger.Error("error getting meeting summary", "err", err)
			res.WriteHeader(http.StatusInternalServerError)
			res.Write(models.NewErrorResponseBuffer(models.UnexpectedErrorMessage))
		}
		meetingsData[i] = models.MeetingResponseData{
			ID:        meeting.ID,
			Name:      meeting.Name,
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
		return
	}

	res.Header().Set("Content-Type", "application/json")
	logger.Info("meetings list successfully obtained")
	res.WriteHeader(http.StatusOK)
	res.Write(buf.Bytes())
}
