package worker

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/scouser-122/meeting-analyzer/internal/client"
	"github.com/scouser-122/meeting-analyzer/internal/config"
	"github.com/scouser-122/meeting-analyzer/internal/domain/model"
	"github.com/scouser-122/meeting-analyzer/internal/domain/repository"
	"github.com/scouser-122/meeting-analyzer/internal/logger"
	"github.com/scouser-122/meeting-analyzer/internal/models"
	"github.com/scouser-122/meeting-analyzer/internal/service"
)

// MeetingProcessor processes uploaded meeting audio files asynchronously.
type MeetingProcessor struct {
	meetingService       *service.MeetingsService
	tasksService         *service.TasksService
	transcriptionService *service.TranscriptionService
	summaryService       *service.SummaryService
	repositoryUtils      repository.RepositoryUtils
	audioProcessor       client.AudioProcessor
	llmClient            client.LLMClient
	meetingsCh           chan *model.Meeting
	maxWorkers           int
	processTimeout       time.Duration
	ctx                  context.Context
	cancel               context.CancelFunc
	wg                   sync.WaitGroup
}

// NewMeetingProcessor creates a new MeetingProcessor with a default channel buffer.
func NewMeetingProcessor(
	meetingService *service.MeetingsService,
	tasksService *service.TasksService,
	transcriptionService *service.TranscriptionService,
	summaryService *service.SummaryService,
	repositorUtils repository.RepositoryUtils,
	audioProcessor client.AudioProcessor,
	summarizeProcessor client.LLMClient,
	serverConfig *config.ServerConfig,
) *MeetingProcessor {
	return NewMeetingProcessorWithBuffer(
		meetingService,
		tasksService,
		transcriptionService,
		summaryService,
		repositorUtils,
		audioProcessor,
		summarizeProcessor,
		serverConfig,
		10,
	)
}

// NewMeetingProcessorWithBuffer создаёт MeetingProcessor с указанным размером буфера канала.
// Полезно для тестирования сценария переполненного канала.
func NewMeetingProcessorWithBuffer(
	meetingService *service.MeetingsService,
	tasksService *service.TasksService,
	transcriptionService *service.TranscriptionService,
	summaryService *service.SummaryService,
	repositorUtils repository.RepositoryUtils,
	audioProcessor client.AudioProcessor,
	summarizeProcessor client.LLMClient,
	serverConfig *config.ServerConfig,
	bufferSize int,
) *MeetingProcessor {
	ctx, cancel := context.WithCancel(context.Background())
	return &MeetingProcessor{
		meetingService:       meetingService,
		tasksService:         tasksService,
		transcriptionService: transcriptionService,
		summaryService:       summaryService,
		repositoryUtils:      repositorUtils,
		audioProcessor:       audioProcessor,
		llmClient:            summarizeProcessor,
		meetingsCh:           make(chan *model.Meeting, bufferSize),
		maxWorkers:           *serverConfig.ProcessorLimit,
		processTimeout:       time.Duration(*serverConfig.ProcessorTimeout) * time.Second,
		ctx:                  ctx,
		cancel:               cancel,
	}
}

// Run starts the worker pool that processes meetings.
func (m *MeetingProcessor) Run() {
	for i := 0; i < m.maxWorkers; i++ {
		m.wg.Add(1)
		go m.worker(i)
	}
}

// Shutdown signals the processor to stop and waits for workers to finish.
func (m *MeetingProcessor) Shutdown() {
	slog.Info("meeting processor shutdown signal received. stopping worker...")
	close(m.meetingsCh)
	m.wg.Wait()
	m.cancel()
}

// ProcessMeeting submits a meeting to the processing queue. Returns false if the queue is full.
func (m *MeetingProcessor) ProcessMeeting(meeting *model.Meeting) bool {
	select {
	case m.meetingsCh <- meeting:
		return true
	default:
		return false // channel full, task rejected
	}
}

func (m *MeetingProcessor) worker(id int) {
	defer m.wg.Done()

	for {
		select {
		case <-m.ctx.Done():
			slog.Info("processor worker stopping", "id", id)
			return
		case meeting, ok := <-m.meetingsCh:
			if !ok {
				slog.Info("prcessor worker task channel closed", "id", id)
				return
			}
			m.processMeetingInWorker(id, meeting)
		}
	}
}

func (m *MeetingProcessor) processMeetingInWorker(workerID int, meeting *model.Meeting) {
	pLogger := slog.Default().With(
		slog.String("meeting_id", meeting.ID),
		slog.String("meeting_name", *meeting.MeetingName),
	)
	ctx := context.WithValue(m.ctx, logger.LoggerKey, pLogger)
	ctx, cancel := context.WithTimeout(ctx, m.processTimeout)
	defer cancel()

	pLogger.Info("start process meeting")

	task, err := m.tasksService.GetByMeetingID(ctx, meeting.ID)
	if err != nil {
		pLogger.Error("processor can't obtain task", "err", err)
		return
	}
	taskID := task.ID

	switch task.Status {
	case model.TaskStatusProcessing, model.TaskStatusCompleted:
		pLogger.Info("meeting processing skipped", "status", task.Status)
		return
	}

	err = m.tasksService.UpdateStatus(ctx, taskID, model.TaskStatusProcessing, nil)
	if err != nil {
		pLogger.Error("processor can't update task status", "err", err)
		return
	}

	var transcriptionText string
	if task.Status == model.TaskStatusTranscribed || task.Status == model.TaskStatusSummarized {
		var transcription *model.Transcription
		transcription, err = m.transcriptionService.GetByMeetingID(ctx, meeting.ID)
		if err != nil {
			pLogger.Error("processor can't load existing transcription", "err", err)
			m.markTaskFailed(ctx, taskID, err)
			return
		}
		transcriptionText = transcription.Text
	} else if task.Status == model.TaskStatusFailed {
		var transcription *model.Transcription
		transcription, err = m.transcriptionService.GetByMeetingID(ctx, meeting.ID)
		if err != nil {
			var customErr *models.CustomErr
			if errors.As(err, &customErr) && customErr.Code == models.ErrCodeTranscriptionNotFound {
				// no transcription yet, will transcribe from audio
			} else {
				pLogger.Error("processor can't get transcription for failed task", "err", err)
				return
			}
		} else {
			transcriptionText = transcription.Text
			err = m.tasksService.UpdateStatus(ctx, taskID, model.TaskStatusTranscribed, nil)
			if err != nil {
				pLogger.Error("processor can't update task status", "err", err)
				return
			}
		}
	}
	if transcriptionText == "" {
		transcriptionText, err = m.transcribeAudio(ctx, taskID, meeting)
		if err != nil {
			pLogger.Error("processor can't transcribe audio", "err", err)
			m.markTaskFailed(m.ctx, taskID, err)
			return
		}
		m.cleanUpAfterProcessing(ctx, meeting)
	}

	if task.Status != model.TaskStatusSummarized {
		err = m.summarizeTranscription(ctx, taskID, transcriptionText, meeting)
		if err != nil {
			pLogger.Error("processor can't summarize transcription", "err", err)
			m.markTaskFailed(ctx, taskID, err)
			return
		}
	}

	err = m.tasksService.UpdateStatus(ctx, taskID, model.TaskStatusCompleted, nil)
	if err != nil {
		pLogger.Error("processor can't update task status", "err", err)
		return
	}

	m.cleanUpAfterProcessing(ctx, meeting)
	pLogger.Info("meeting successfully processed")
}

// RetryMeeting schedules a failed or partially processed meeting for reprocessing.
func (m *MeetingProcessor) RetryMeeting(ctx context.Context, meetingID string) error {
	task, err := m.tasksService.GetByMeetingID(ctx, meetingID)
	if err != nil {
		return err
	}

	switch task.Status {
	case model.TaskStatusFailed, model.TaskStatusTranscribed, model.TaskStatusSummarized:
		// allowed to retry
	case model.TaskStatusProcessing:
		return models.NewCustomErr(models.ErrCodeMeetingAlreadyProcessing, "Встреча уже обрабатывается", http.StatusConflict, nil)
	case model.TaskStatusCompleted:
		return models.NewCustomErr(models.ErrCodeMeetingAlreadyCompleted, "Обработка встречи уже завершена", http.StatusConflict, nil)
	default:
		return models.NewCustomErrf(models.ErrCodeMeetingCannotRetry, http.StatusConflict, nil, "Нельзя повторить обработку встречи в статусе %s", task.Status)
	}

	meeting, err := m.meetingService.GetByID(ctx, meetingID)
	if err != nil {
		return err
	}

	if !m.ProcessMeeting(meeting) {
		return models.NewCustomErr(models.ErrCodeQueueFull, "Сервер перегружен. Попробуйте повторить позже", http.StatusTooManyRequests, nil)
	}

	return nil
}

func (m *MeetingProcessor) transcribeAudio(ctx context.Context, taskID string, meeting *model.Meeting) (string, error) {
	transcriptionText, err := m.audioProcessor.TranscribeAudio(ctx, meeting)
	if err != nil {
		return "", err
	}

	tx, err := m.repositoryUtils.CreateTransaction(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	err = m.transcriptionService.AddNewTranscription(ctx, &model.Transcription{
		ID:        uuid.New().String(),
		MeetingID: meeting.ID,
		UserID:    meeting.UserID,
		Text:      transcriptionText,
		CreatedAt: time.Now(),
	})
	if err != nil {
		return "", err
	}

	err = m.tasksService.UpdateStatus(ctx, taskID, model.TaskStatusTranscribed, nil)
	if err != nil {
		return "", err
	}

	err = m.repositoryUtils.CommitTransaction(ctx, tx)
	if err != nil {
		return "", err
	}

	return transcriptionText, nil
}

func (m *MeetingProcessor) summarizeTranscription(
	ctx context.Context,
	taskID string,
	transcriptionText string,
	meeting *model.Meeting,
) error {
	summary, err := m.llmClient.SummarizeTranscription(ctx, meeting, transcriptionText)
	if err != nil {
		return err
	}

	tx, err := m.repositoryUtils.CreateTransaction(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	err = m.summaryService.AddNewSummary(ctx, &model.Summary{
		ID:        uuid.New().String(),
		MeetingID: meeting.ID,
		UserID:    meeting.UserID,
		Text:      summary,
		CreatedAt: time.Now(),
	})
	if err != nil {
		return err
	}

	m.tasksService.UpdateStatus(ctx, taskID, model.TaskStatusSummarized, nil)

	err = m.repositoryUtils.CommitTransaction(ctx, tx)
	if err != nil {
		return err
	}

	return nil
}

func (m *MeetingProcessor) markTaskFailed(ctx context.Context, taskID string, err error) {
	errMessage := err.Error()
	err = m.tasksService.UpdateStatus(ctx, taskID, model.TaskStatusFailed, &errMessage)
	if err != nil {
		logger := logger.GetSlogLoggerFromContext(ctx)
		logger.Error("processor can't update task status", "err", err)
	}
}

func (m *MeetingProcessor) cleanUpAfterProcessing(ctx context.Context, meeting *model.Meeting) {
	err := m.meetingService.DeleteMeetingAudioFile(m.ctx, meeting)
	if err != nil {
		logger := logger.GetSlogLoggerFromContext(ctx)
		logger.Error("can't delete meeting audio file", "err", err)
	}
}
