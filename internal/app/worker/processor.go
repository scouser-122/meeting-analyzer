package worker

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/scouser-122/meeting-analyzer/internal/app/service"
	"github.com/scouser-122/meeting-analyzer/internal/client/processors"
	"github.com/scouser-122/meeting-analyzer/internal/config"
	"github.com/scouser-122/meeting-analyzer/internal/domain/model"
)

type MeetingProcessor struct {
	meetingService       *service.MeetingsService
	tasksService         *service.TasksService
	transcriptionService *service.TranscriptionService
	summaryService       *service.SummaryService
	audioProcessor       processors.AudioProcessor
	summarizeProcessor   processors.SummarizeProcessor
	Meetins              chan *model.Meeting
}

func NewMeetingProcessor(
	meetingService *service.MeetingsService,
	tasksService *service.TasksService,
	transcriptionService *service.TranscriptionService,
	summaryService *service.SummaryService,
	audioProcessor processors.AudioProcessor,
	summarizeProcessor processors.SummarizeProcessor,
	serverConfig *config.ServerConfig,
) *MeetingProcessor {
	return &MeetingProcessor{
		meetingService:       meetingService,
		tasksService:         tasksService,
		transcriptionService: transcriptionService,
		summaryService:       summaryService,
		audioProcessor:       audioProcessor,
		summarizeProcessor:   summarizeProcessor,
		Meetins:              make(chan *model.Meeting, *serverConfig.ProcessorLimit),
	}
}

func (m *MeetingProcessor) Run() {
	stopChanSend := make(chan struct{})
	go m.ProccessorContinousWorker(stopChanSend)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)
	go func() {
		sig := <-sigChan
		slog.Info("shutdown signal received. stopping worker...", "sig", sig)
		close(stopChanSend)
	}()
}

const maxInFlight = 3

func (m *MeetingProcessor) ProccessorContinousWorker(stopCh chan struct{}) {
	var wg sync.WaitGroup
	wg.Add(1)
	semMaxLimit := make(chan struct{}, maxInFlight)
	for {
		stopProcessing := false
		select {
		case <-stopCh:
			slog.Info("stop processing meetings")
			wg.Done()
			stopProcessing = true
		default:
			meeting := <-m.Meetins
			semMaxLimit <- struct{}{}
			go func(meeting *model.Meeting) {
				defer func() { <-semMaxLimit }()
				m.processMeeting(meeting)
			}(meeting)
		}
		if stopProcessing {
			break
		}
	}
	wg.Wait()
	slog.Info("processor worker stopped")
}

func (m *MeetingProcessor) processMeeting(meeting *model.Meeting) {
	slog.Info("start process meeting", "name", *meeting.Name, "meetingID", meeting.ID)

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

	summary, err := m.summarizeProcessor.SummarizeTranscription(meeting, transcriptionText)
	if err != nil {
		slog.Error("processor can't summarize transcription", "err", err, "meetingID", meeting.ID)
		errMessage := err.Error()
		m.tasksService.UpdateStatus(ctx, taskID, model.TaskStatusFailed, &errMessage)
		return
	}
	m.tasksService.UpdateStatus(ctx, taskID, model.TaskStatusSummarized, nil)

	err = m.summaryService.AddNewSummary(ctx, &model.Summary{
		ID:        uuid.New().String(),
		MeetingID: meeting.ID,
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

	slog.Info("meeting successfully processed", "name", *meeting.Name, "meetingID", meeting.ID)
}
