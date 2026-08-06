package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/scouser-122/meeting-analyzer/internal/client"
	"github.com/scouser-122/meeting-analyzer/internal/logger"
	"github.com/scouser-122/meeting-analyzer/internal/models"
	"github.com/scouser-122/meeting-analyzer/internal/service"
)

// ChatHandler specifies http request handler for chat requests
type ChatHandler struct {
	llmClient            client.LLMClient
	meetingsService      *service.MeetingsService
	tasksService         *service.TasksService
	transcriptionService *service.TranscriptionService
	summaryService       *service.SummaryService
}

// NewChatHandler creates and returns pointer to new ChatHandler
func NewChatHandler(
	llmClient client.LLMClient,
	meetingsService *service.MeetingsService,
	tasksService *service.TasksService,
	transcriptionService *service.TranscriptionService,
	summaryService *service.SummaryService,
) *ChatHandler {
	return &ChatHandler{
		llmClient:            llmClient,
		meetingsService:      meetingsService,
		tasksService:         tasksService,
		transcriptionService: transcriptionService,
		summaryService:       summaryService,
	}
}

// HandleChat processes chat request
func (h *ChatHandler) HandleChat(res http.ResponseWriter, req *http.Request) {
	logger := logger.GetSlogLoggerFromContext(req.Context())

	res.Header().Set("Content-Type", "application/json")

	bodyBuf, err := io.ReadAll(req.Body)
	if err != nil {
		logger.Error("cannot read request body", "err", err)
		res.WriteHeader(http.StatusBadRequest)
		res.Write(models.NewErrorResponseBuffer(models.UnexpectedErrorMessage))
		return
	}

	var request models.ChatRequest
	if err := json.Unmarshal(bodyBuf, &request); err != nil {
		logger.Error("cannot decode request json body", "err", err)
		res.WriteHeader(http.StatusBadRequest)
		res.Write(models.NewErrorResponseBuffer(models.UnexpectedErrorMessage))
		return
	}

	intent, err := h.llmClient.ExtractIntent(request.Question)
	if err != nil {
		logger.Error("llm client extract intenr error", "err", err)
		res.WriteHeader(http.StatusInternalServerError)
		res.Write(models.NewErrorResponseBuffer(models.UnexpectedErrorMessage))
		return
	}

	var chatResponse models.ChatResponse

	if !intent.IsMeetingQuery {
		chatResponse.Answer = "Похоже, вопрос не про встречу. Спросите, например: «найди выжимку по встрече где обсуждали X»."
	} else {
		transcriptions, err := h.transcriptionService.FindByKeyWords(req.Context(), request.UserID, intent.Keywords, intent.Topic)
		if err != nil {
			handleServiceError(err, res)
			return
		}

		if len(transcriptions) > 0 {
			t := transcriptions[0]
			summary, err := h.summaryService.GetSummary(req.Context(), t.MeetingID)
			if err != nil {
				handleServiceError(err, res)
				return
			}
			if summary == nil {
				chatResponse.Answer = "Не удалось найти встречу по указанной теме"
			} else {
				meeting, err := h.meetingsService.GetByID(req.Context(), t.MeetingID)
				if err != nil {
					handleServiceError(err, res)
					return
				}
				chatResponse.Answer = fmt.Sprintf("Название встречи: «%s». Дата создания: %s. Краткая выжимка: %s", *meeting.MeetingName, meeting.UpdatedAt, *summary)
			}
		} else {
			chatResponse.Answer = "Не удалось найти встречу по указанной теме"
		}
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	if err := enc.Encode(chatResponse); err != nil {
		logger.Error("error encoding response ", "err", err)
		res.WriteHeader(http.StatusInternalServerError)
		res.Write(models.NewErrorResponseBuffer(models.UnexpectedErrorMessage))
		return
	}

	logger.Info("meetings list successfully obtained")
	res.WriteHeader(http.StatusOK)
	res.Write(buf.Bytes())
}
