package worker

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/scouser-122/meeting-analyzer/internal/client"
	"github.com/scouser-122/meeting-analyzer/internal/config"
	"github.com/scouser-122/meeting-analyzer/internal/domain/model"
	"github.com/scouser-122/meeting-analyzer/internal/service"
)

type MeetingProcessor struct {
	meetingService       *service.MeetingsService
	tasksService         *service.TasksService
	transcriptionService *service.TranscriptionService
	summaryService       *service.SummaryService
	audioProcessor       client.AudioProcessor
	llmClient            client.LLMClient
	processorLimit       int64
	Meetins              chan *model.Meeting
	stopChan             chan struct{}
	waitGroup            sync.WaitGroup
}

func NewMeetingProcessor(
	meetingService *service.MeetingsService,
	tasksService *service.TasksService,
	transcriptionService *service.TranscriptionService,
	summaryService *service.SummaryService,
	audioProcessor client.AudioProcessor,
	summarizeProcessor client.LLMClient,
	serverConfig *config.ServerConfig,
) *MeetingProcessor {
	return &MeetingProcessor{
		meetingService:       meetingService,
		tasksService:         tasksService,
		transcriptionService: transcriptionService,
		summaryService:       summaryService,
		audioProcessor:       audioProcessor,
		llmClient:            summarizeProcessor,
		Meetins:              make(chan *model.Meeting, 10),
		processorLimit:       *serverConfig.ProcessorLimit,
	}
}

func (m *MeetingProcessor) Run() {
	m.stopChan = make(chan struct{})
	m.waitGroup.Add(1)
	go m.ProccessorContinousWorker()
}

func (m *MeetingProcessor) Shutdown() {
	slog.Info("meeting processor shutdown signal received. stopping worker...")
	close(m.stopChan)
	m.waitGroup.Wait()
}

func (m *MeetingProcessor) ProcessMeeting(meeting *model.Meeting) {
	go func() {
		m.Meetins <- meeting
	}()
}

func (m *MeetingProcessor) ProccessorContinousWorker() {
	semMaxLimit := make(chan struct{}, m.processorLimit)
	for {
		stopProcessing := false
		select {
		case <-m.stopChan:
			slog.Info("stop processing meetings")
			if len(m.Meetins) > 0 {
				var stopWaitGroup sync.WaitGroup
				for meeting := range m.Meetins {
					slog.Info("meetings channel len: ", "len", len(m.Meetins))
					stopWaitGroup.Add(1)
					semMaxLimit <- struct{}{}
					go func(meeting *model.Meeting) {
						defer func() { <-semMaxLimit }()
						m.processMeetingInWorker(meeting)
						stopWaitGroup.Done()
					}(meeting)
					if len(m.Meetins) == 0 {
						slog.Info("stop read meetings channel")
						break
					}
				}
				stopWaitGroup.Wait()
			}
			m.waitGroup.Done()
			stopProcessing = true
		default:
			if len(m.Meetins) > 0 {
				meeting := <-m.Meetins
				semMaxLimit <- struct{}{}
				go func(meeting *model.Meeting) {
					defer func() { <-semMaxLimit }()
					m.processMeetingInWorker(meeting)
				}(meeting)
			} else {
				time.Sleep(100 * time.Millisecond)
			}
		}
		if stopProcessing {
			break
		}
	}
	slog.Info("processor worker stopped")
}

func (m *MeetingProcessor) processMeetingInWorker(meeting *model.Meeting) {
	slog.Info("start process meeting", "name", *meeting.MeetingName, "meetingID", meeting.ID)

	ctx := context.Background()

	task, err := m.tasksService.CreateNewTask(ctx, meeting.ID)
	if err != nil {
		slog.Error("processor can't create task", "err", err, "meetingID", meeting.ID)
		return
	}
	taskID := task.ID
	m.tasksService.UpdateStatus(ctx, taskID, model.TaskStatusProcessing, nil)

	transcriptionText, err := m.audioProcessor.TranscribeAudio(meeting)
	if err != nil {
		slog.Error("processor can't transcribe audio", "err", err, "meetingID", meeting.ID)
		errMessage := err.Error()
		m.tasksService.UpdateStatus(ctx, taskID, model.TaskStatusFailed, &errMessage)
		return
	}

	err = m.transcriptionService.AddNewTranscription(ctx, &model.Transcription{
		ID:        uuid.New().String(),
		MeetingID: meeting.ID,
		UserID:    meeting.UserID,
		Text:      transcriptionText,
		CreatedAt: time.Now(),
	})
	if err != nil {
		slog.Error("processor can't save transcription", "err", err, "meetingID", meeting.ID)
		errMessage := err.Error()
		m.tasksService.UpdateStatus(ctx, taskID, model.TaskStatusFailed, &errMessage)
		return
	}
	m.tasksService.UpdateStatus(ctx, taskID, model.TaskStatusTranscribed, nil)

	summary, err := m.llmClient.SummarizeTranscription(meeting, transcriptionText)
	if err != nil {
		slog.Error("LLM client can't summarize transcription", "err", err, "meetingID", meeting.ID)
		errMessage := err.Error()
		m.tasksService.UpdateStatus(ctx, taskID, model.TaskStatusFailed, &errMessage)
		return
	}
	m.tasksService.UpdateStatus(ctx, taskID, model.TaskStatusSummarized, nil)

	err = m.summaryService.AddNewSummary(ctx, &model.Summary{
		ID:        uuid.New().String(),
		MeetingID: meeting.ID,
		UserID:    meeting.UserID,
		Text:      summary,
		CreatedAt: time.Now(),
	})
	if err != nil {
		slog.Error("processor can't save summarization", "err", err, "meetingID", meeting.ID)
		errMessage := err.Error()
		m.tasksService.UpdateStatus(ctx, taskID, model.TaskStatusFailed, &errMessage)
		return
	}

	m.tasksService.UpdateStatus(ctx, taskID, model.TaskStatusCompleted, nil)

	slog.Info("meeting successfully processed", "name", *meeting.MeetingName, "meetingID", meeting.ID)
}
