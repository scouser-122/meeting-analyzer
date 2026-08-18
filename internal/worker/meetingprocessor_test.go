package worker

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/scouser-122/meeting-analyzer/internal/client"
	"github.com/scouser-122/meeting-analyzer/internal/config"
	"github.com/scouser-122/meeting-analyzer/internal/domain/model"
	"github.com/scouser-122/meeting-analyzer/internal/domain/repository"
	"github.com/scouser-122/meeting-analyzer/internal/models"
	"github.com/scouser-122/meeting-analyzer/internal/service"
)

// ─────────────────────────────────────────────────────────────────────────────
// Заглушки репозиториев и клиентов.
// Все заглушки потокобезопасны (используют мьютексы), чтобы race detector
// фиксировал только реальные гонки в коде MeetingProcessor, а не в самом тесте.
// ─────────────────────────────────────────────────────────────────────────────

type fakeMeetingRepository struct {
	mu       sync.Mutex
	meetings map[string]*model.Meeting
}

func newFakeMeetingRepository() *fakeMeetingRepository {
	return &fakeMeetingRepository{meetings: make(map[string]*model.Meeting)}
}

func (f *fakeMeetingRepository) put(m *model.Meeting) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.meetings[m.ID] = cloneMeeting(m)
}

func (f *fakeMeetingRepository) Create(ctx context.Context, userID string) (*model.Meeting, error) {
	m := &model.Meeting{ID: uuid.New().String(), UserID: userID, CreatedAt: time.Now()}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.meetings[m.ID] = m
	return cloneMeeting(m), nil
}

func (f *fakeMeetingRepository) GetByID(ctx context.Context, id string) (*model.Meeting, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	m, ok := f.meetings[id]
	if !ok {
		return nil, fmt.Errorf("meeting not found")
	}
	return cloneMeeting(m), nil
}

func (f *fakeMeetingRepository) GetByUserID(ctx context.Context, userID string) ([]*model.Meeting, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]*model.Meeting, 0)
	for _, m := range f.meetings {
		if m.UserID == userID {
			out = append(out, cloneMeeting(m))
		}
	}
	return out, nil
}

func (f *fakeMeetingRepository) FindByNameContains(ctx context.Context, userID, namePart string) ([]*model.Meeting, error) {
	return nil, nil
}

func (f *fakeMeetingRepository) Update(ctx context.Context, m *model.Meeting) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	c := cloneMeeting(m)
	c.UpdatedAt = time.Now()
	f.meetings[m.ID] = c
	return nil
}

func (f *fakeMeetingRepository) Delete(ctx context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.meetings, id)
	return nil
}

func cloneMeeting(m *model.Meeting) *model.Meeting {
	if m == nil {
		return nil
	}
	c := *m
	if m.MeetingName != nil {
		n := *m.MeetingName
		c.MeetingName = &n
	}
	if m.FilePath != nil {
		p := *m.FilePath
		c.FilePath = &p
	}
	if m.OriginalFilename != nil {
		o := *m.OriginalFilename
		c.OriginalFilename = &o
	}
	return &c
}

type fakeTaskRepository struct {
	mu   sync.Mutex
	byID map[string]*model.Task
}

func newFakeTaskRepository() *fakeTaskRepository {
	return &fakeTaskRepository{byID: make(map[string]*model.Task)}
}

func (f *fakeTaskRepository) Create(ctx context.Context, meetingID string) (*model.Task, error) {
	task := &model.Task{
		ID:        uuid.New().String(),
		MeetingID: meetingID,
		Status:    model.TaskStatusCreated,
		CreatedAt: time.Now(),
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byID[task.ID] = cloneTask(task)
	return cloneTask(task), nil
}

func (f *fakeTaskRepository) GetByID(ctx context.Context, id string) (*model.Task, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	task, ok := f.byID[id]
	if !ok {
		return nil, fmt.Errorf("task not found")
	}
	return cloneTask(task), nil
}

func (f *fakeTaskRepository) GetByMeetingID(ctx context.Context, meetingID string) (*model.Task, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, task := range f.byID {
		if task.MeetingID == meetingID {
			return cloneTask(task), nil
		}
	}
	return nil, fmt.Errorf("task not found")
}

func (f *fakeTaskRepository) UpdateStatus(ctx context.Context, id string, status model.TaskStatus, errorMessage *string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	task, ok := f.byID[id]
	if !ok {
		return fmt.Errorf("task not found")
	}
	task.Status = status
	task.ErrorMessage = cloneStringPtr(errorMessage)
	task.UpdatedAt = time.Now()
	return nil
}

func (f *fakeTaskRepository) DeleteByMeetingID(ctx context.Context, meetingID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for id, task := range f.byID {
		if task.MeetingID == meetingID {
			delete(f.byID, id)
			return nil
		}
	}
	return nil
}

func (f *fakeTaskRepository) statusByMeeting(meetingID string) (model.TaskStatus, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, task := range f.byID {
		if task.MeetingID == meetingID {
			return task.Status, true
		}
	}
	return "", false
}

func cloneTask(t *model.Task) *model.Task {
	if t == nil {
		return nil
	}
	c := *t
	c.ErrorMessage = cloneStringPtr(t.ErrorMessage)
	return &c
}

func cloneStringPtr(s *string) *string {
	if s == nil {
		return nil
	}
	c := *s
	return &c
}

type fakeTranscriptionRepository struct {
	mu   sync.Mutex
	data map[string]*model.Transcription
}

func newFakeTranscriptionRepository() *fakeTranscriptionRepository {
	return &fakeTranscriptionRepository{data: make(map[string]*model.Transcription)}
}

func (f *fakeTranscriptionRepository) Create(ctx context.Context, t *model.Transcription) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	c := *t
	f.data[t.MeetingID] = &c
	return nil
}

func (f *fakeTranscriptionRepository) GetByMeetingID(ctx context.Context, meetingID string) (*model.Transcription, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	t, ok := f.data[meetingID]
	if !ok {
		return nil, &models.CustomErr{Message: "transcription not found", HTTPStatus: http.StatusNotFound}
	}
	c := *t
	return &c, nil
}

func (f *fakeTranscriptionRepository) FindByTextContains(ctx context.Context, userID, textPart string) ([]*model.Transcription, error) {
	return nil, nil
}

func (f *fakeTranscriptionRepository) FindByKeyWords(ctx context.Context, userID string, keywords []string, topic string) ([]*model.Transcription, error) {
	return nil, nil
}

func (f *fakeTranscriptionRepository) DeleteByMeetingID(ctx context.Context, meetingID string) error {
	return nil
}

func (f *fakeTranscriptionRepository) get(meetingID string) (*model.Transcription, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	t, ok := f.data[meetingID]
	if !ok {
		return nil, false
	}
	c := *t
	return &c, true
}

type fakeSummaryRepository struct {
	mu   sync.Mutex
	data map[string]*model.Summary
}

func newFakeSummaryRepository() *fakeSummaryRepository {
	return &fakeSummaryRepository{data: make(map[string]*model.Summary)}
}

func (f *fakeSummaryRepository) Create(ctx context.Context, s *model.Summary) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	c := *s
	f.data[s.MeetingID] = &c
	return nil
}

func (f *fakeSummaryRepository) GetByMeetingID(ctx context.Context, meetingID string) (*model.Summary, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.data[meetingID]
	if !ok {
		return nil, fmt.Errorf("summary not found")
	}
	c := *s
	return &c, nil
}

func (f *fakeSummaryRepository) FindByTextContains(ctx context.Context, userID, textPart string) ([]*model.Summary, error) {
	return nil, nil
}

func (f *fakeSummaryRepository) DeleteByMeetingID(ctx context.Context, meetingID string) error {
	return nil
}

func (f *fakeSummaryRepository) get(meetingID string) (*model.Summary, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.data[meetingID]
	if !ok {
		return nil, false
	}
	c := *s
	return &c, true
}

type fakeUserRepository struct {
	mu    sync.Mutex
	users map[string]*model.User
}

func newFakeUserRepository() *fakeUserRepository {
	return &fakeUserRepository{users: make(map[string]*model.User)}
}

func (f *fakeUserRepository) Create(ctx context.Context, id string) (*model.User, error) {
	u := &model.User{ID: id, CreatedAt: time.Now()}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.users[id] = u
	return u, nil
}

func (f *fakeUserRepository) GetByID(ctx context.Context, id string) (*model.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.users[id]
	if !ok {
		return nil, fmt.Errorf("user not found")
	}
	return u, nil
}

type fakeRepoUtils struct{}

func (fakeRepoUtils) CreateTransaction(ctx context.Context) (models.GenericTransaction, error) {
	return &fakeTransaction{}, nil
}

func (fakeRepoUtils) CommitTransaction(ctx context.Context, tx models.GenericTransaction) error {
	return nil
}

type fakeTransaction struct{}

func (*fakeTransaction) Rollback(ctx context.Context) error { return nil }
func (*fakeTransaction) Commit(ctx context.Context) error   { return nil }

type fakeAudioProcessor struct{}

func (fakeAudioProcessor) TranscribeAudio(ctx context.Context, meeting *model.Meeting) (string, error) {
	return "transcription for " + meeting.ID, nil
}

type fakeLLMClient struct{}

func (fakeLLMClient) SummarizeTranscription(ctx context.Context, meeting *model.Meeting, transcriptionText string) (string, error) {
	return "summary for " + meeting.ID, nil
}

func (fakeLLMClient) ExtractIntent(ctx context.Context, text string) (*models.QueryIntent, error) {
	return &models.QueryIntent{}, nil
}

// Compile-time проверки соответствия интерфейсам.
var (
	_ repository.MeetingRepository       = (*fakeMeetingRepository)(nil)
	_ repository.TaskRepository          = (*fakeTaskRepository)(nil)
	_ repository.TranscriptionRepository = (*fakeTranscriptionRepository)(nil)
	_ repository.SummaryRepository       = (*fakeSummaryRepository)(nil)
	_ repository.UserRepository          = (*fakeUserRepository)(nil)
	_ repository.RepositoryUtils         = fakeRepoUtils{}
	_ models.GenericTransaction          = (*fakeTransaction)(nil)
	_ client.AudioProcessor              = fakeAudioProcessor{}
	_ client.LLMClient                   = fakeLLMClient{}
)

// ─────────────────────────────────────────────────────────────────────────────
// Настройка тестового окружения
// ─────────────────────────────────────────────────────────────────────────────

type fakeState struct {
	processor      *MeetingProcessor
	meetings       *fakeMeetingRepository
	tasks          *fakeTaskRepository
	transcriptions *fakeTranscriptionRepository
	summaries      *fakeSummaryRepository
}

func newRaceTestState(t *testing.T) *fakeState {
	t.Helper()

	limit := 8
	timeout := 30
	uploadDir := t.TempDir()
	maxUpload := int64(10 << 20)
	serverConfig := &config.ServerConfig{
		ProcessorLimit:   &limit,
		ProcessorTimeout: &timeout,
		UploadFileDir:    &uploadDir,
		MaxUploadSize:    &maxUpload,
	}

	meetings := newFakeMeetingRepository()
	tasks := newFakeTaskRepository()
	transcriptions := newFakeTranscriptionRepository()
	summaries := newFakeSummaryRepository()
	users := newFakeUserRepository()
	repoUtils := fakeRepoUtils{}

	usersService := service.NewUsersService(users)
	tasksService := service.NewTasksService(tasks, repoUtils)
	transcriptionService := service.NewTranscriptionService(transcriptions, repoUtils)
	summaryService := service.NewSummaryService(summaries, repoUtils)
	meetingsService := service.NewMeetingsService(meetings, repoUtils, usersService, tasksService, transcriptionService, summaryService, serverConfig)

	processor := NewMeetingProcessorWithBuffer(
		meetingsService,
		tasksService,
		transcriptionService,
		summaryService,
		repoUtils,
		fakeAudioProcessor{},
		fakeLLMClient{},
		serverConfig,
		128,
	)

	return &fakeState{
		processor:      processor,
		meetings:       meetings,
		tasks:          tasks,
		transcriptions: transcriptions,
		summaries:      summaries,
	}
}

func newTestMeeting(t *testing.T, dir string, idx int) *model.Meeting {
	t.Helper()

	name := fmt.Sprintf("meeting-%d", idx)
	file := filepath.Join(dir, fmt.Sprintf("meeting-%d.mp3", idx))
	if err := os.WriteFile(file, []byte("fake audio"), 0o644); err != nil {
		t.Fatalf("create meeting file: %v", err)
	}

	return &model.Meeting{
		ID:          uuid.New().String(),
		UserID:      "user-1",
		MeetingName: &name,
		FilePath:    &file,
	}
}

func seedMeetingAndTask(t *testing.T, state *fakeState, meeting *model.Meeting) {
	t.Helper()

	state.meetings.put(meeting)
	if _, err := state.tasks.Create(context.Background(), meeting.ID); err != nil {
		t.Fatalf("seed task for meeting %s: %v", meeting.ID, err)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Моки для проверки таймаута.
// ─────────────────────────────────────────────────────────────────────────────

type slowAudioProcessor struct {
	delay    time.Duration
	blockCtx bool
}

func (s slowAudioProcessor) TranscribeAudio(ctx context.Context, meeting *model.Meeting) (string, error) {
	timer := time.NewTimer(s.delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-timer.C:
		return "transcription for " + meeting.ID, nil
	}
}

func newTimeoutTestState(t *testing.T, timeoutSec int, audio client.AudioProcessor) *fakeState {
	t.Helper()

	limit := 1
	timeout := timeoutSec
	uploadDir := t.TempDir()
	maxUpload := int64(10 << 20)
	serverConfig := &config.ServerConfig{
		ProcessorLimit:   &limit,
		ProcessorTimeout: &timeout,
		UploadFileDir:    &uploadDir,
		MaxUploadSize:    &maxUpload,
	}

	meetings := newFakeMeetingRepository()
	tasks := newFakeTaskRepository()
	transcriptions := newFakeTranscriptionRepository()
	summaries := newFakeSummaryRepository()
	users := newFakeUserRepository()
	repoUtils := fakeRepoUtils{}

	usersService := service.NewUsersService(users)
	tasksService := service.NewTasksService(tasks, repoUtils)
	transcriptionService := service.NewTranscriptionService(transcriptions, repoUtils)
	summaryService := service.NewSummaryService(summaries, repoUtils)
	meetingsService := service.NewMeetingsService(meetings, repoUtils, usersService, tasksService, transcriptionService, summaryService, serverConfig)

	processor := NewMeetingProcessorWithBuffer(
		meetingsService,
		tasksService,
		transcriptionService,
		summaryService,
		repoUtils,
		audio,
		fakeLLMClient{},
		serverConfig,
		128,
	)

	return &fakeState{
		processor:      processor,
		meetings:       meetings,
		tasks:          tasks,
		transcriptions: transcriptions,
		summaries:      summaries,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Тесты таймаута processMeetingInWorker
// ─────────────────────────────────────────────────────────────────────────────

func TestProcessMeetingInWorker_Timeout(t *testing.T) {
	type when struct {
		audioDelay time.Duration
		blockCtx   bool
		timeoutSec int
	}

	type want struct {
		status           model.TaskStatus
		errContains      string
		hasTranscription bool
		hasSummary       bool
	}

	var testsList = []struct {
		name string
		when when
		want want
	}{
		{
			name: "audio processor exceeds timeout",
			when: when{
				audioDelay: 1500 * time.Millisecond,
				blockCtx:   true,
				timeoutSec: 1,
			},
			want: want{
				status:           model.TaskStatusFailed,
				errContains:      "context deadline exceeded",
				hasTranscription: false,
				hasSummary:       false,
			},
		},
		{
			name: "audio processor finishes before timeout",
			when: when{
				audioDelay: 50 * time.Millisecond,
				blockCtx:   true,
				timeoutSec: 1,
			},
			want: want{
				status:           model.TaskStatusCompleted,
				errContains:      "",
				hasTranscription: true,
				hasSummary:       true,
			},
		},
	}

	for _, test := range testsList {
		t.Run(test.name, func(t *testing.T) {
			audio := slowAudioProcessor{delay: test.when.audioDelay, blockCtx: test.when.blockCtx}
			state := newTimeoutTestState(t, test.when.timeoutSec, audio)
			dir := t.TempDir()

			meeting := newTestMeeting(t, dir, 0)
			seedMeetingAndTask(t, state, meeting)

			state.processor.processMeetingInWorker(0, meeting)

			status, ok := state.tasks.statusByMeeting(meeting.ID)
			if !ok {
				t.Fatalf("task for meeting %s not found", meeting.ID)
			}
			if status != test.want.status {
				t.Errorf("status = %q, want %q", status, test.want.status)
			}

			_, hasTranscription := state.transcriptions.get(meeting.ID)
			if hasTranscription != test.want.hasTranscription {
				t.Errorf("hasTranscription = %v, want %v", hasTranscription, test.want.hasTranscription)
			}

			_, hasSummary := state.summaries.get(meeting.ID)
			if hasSummary != test.want.hasSummary {
				t.Errorf("hasSummary = %v, want %v", hasSummary, test.want.hasSummary)
			}

			if test.want.errContains != "" {
				state.tasks.mu.Lock()
				var task *model.Task
				for _, tt := range state.tasks.byID {
					if tt.MeetingID == meeting.ID {
						task = tt
						break
					}
				}
				state.tasks.mu.Unlock()

				if task == nil || task.ErrorMessage == nil {
					t.Errorf("expected error message containing %q, got nil", test.want.errContains)
				} else if !strings.Contains(*task.ErrorMessage, test.want.errContains) {
					t.Errorf("error message = %q, want containing %q", *task.ErrorMessage, test.want.errContains)
				}
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Тесты race condition
// ─────────────────────────────────────────────────────────────────────────────

// TestProcessMeetingInWorker_ConcurrentProcessingIsRaceFree напрямую вызывает
// processMeetingInWorker из многих горутин с разными встречами. Запуск с флагом
// -race выявит гонки данных в коде обработки при параллельной работе.
func TestProcessMeetingInWorker_ConcurrentProcessingIsRaceFree(t *testing.T) {
	const meetingCount = 64

	state := newRaceTestState(t)
	dir := t.TempDir()

	meetings := make([]*model.Meeting, 0, meetingCount)
	for i := 0; i < meetingCount; i++ {
		meeting := newTestMeeting(t, dir, i)
		seedMeetingAndTask(t, state, meeting)
		meetings = append(meetings, meeting)
	}

	var wg sync.WaitGroup
	for i, meeting := range meetings {
		wg.Add(1)
		go func(workerID int, m *model.Meeting) {
			defer wg.Done()
			state.processor.processMeetingInWorker(workerID, m)
		}(i%8, meeting)
	}
	wg.Wait()

	for _, meeting := range meetings {
		status, ok := state.tasks.statusByMeeting(meeting.ID)
		if !ok || status != model.TaskStatusCompleted {
			t.Errorf("meeting %s: status = %q (ok=%v), want %q", meeting.ID, status, ok, model.TaskStatusCompleted)
		}
		if _, ok := state.transcriptions.get(meeting.ID); !ok {
			t.Errorf("meeting %s: missing transcription", meeting.ID)
		}
		if _, ok := state.summaries.get(meeting.ID); !ok {
			t.Errorf("meeting %s: missing summary", meeting.ID)
		}
	}
}

// TestMeetingProcessor_ConcurrentWorkersIsRaceFree прогоняет встречи через
// реальный пул воркеров (Run/ProcessMeeting/Shutdown), что воспроизводит
// штатный сценарий параллельной обработки. Запуск с флагом -race проверяет
// отсутствие гонок в worker-цикле и processMeetingInWorker.
func TestMeetingProcessor_ConcurrentWorkersIsRaceFree(t *testing.T) {
	const meetingCount = 32

	state := newRaceTestState(t)
	dir := t.TempDir()

	meetings := make([]*model.Meeting, 0, meetingCount)
	for i := 0; i < meetingCount; i++ {
		meeting := newTestMeeting(t, dir, i)
		seedMeetingAndTask(t, state, meeting)
		meetings = append(meetings, meeting)
	}

	state.processor.Run()
	for _, meeting := range meetings {
		if !state.processor.ProcessMeeting(meeting) {
			t.Fatalf("failed to enqueue meeting %s", meeting.ID)
		}
	}
	state.processor.Shutdown()

	for _, meeting := range meetings {
		status, ok := state.tasks.statusByMeeting(meeting.ID)
		if !ok || status != model.TaskStatusCompleted {
			t.Errorf("meeting %s: status = %q (ok=%v), want %q", meeting.ID, status, ok, model.TaskStatusCompleted)
		}
	}
}

// failingAudioProcessor fails transcription on the first call for each meeting.
type failingAudioProcessor struct {
	mu       sync.Mutex
	attempts map[string]int
}

func (f *failingAudioProcessor) TranscribeAudio(ctx context.Context, meeting *model.Meeting) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.attempts[meeting.ID]++
	if f.attempts[meeting.ID] == 1 {
		return "", fmt.Errorf("transcription failed")
	}
	return "transcription for " + meeting.ID, nil
}

func newRetryTestState(t *testing.T) *fakeState {
	t.Helper()

	limit := 1
	timeout := 30
	uploadDir := t.TempDir()
	maxUpload := int64(10 << 20)
	serverConfig := &config.ServerConfig{
		ProcessorLimit:   &limit,
		ProcessorTimeout: &timeout,
		UploadFileDir:    &uploadDir,
		MaxUploadSize:    &maxUpload,
	}

	meetings := newFakeMeetingRepository()
	tasks := newFakeTaskRepository()
	transcriptions := newFakeTranscriptionRepository()
	summaries := newFakeSummaryRepository()
	users := newFakeUserRepository()
	repoUtils := fakeRepoUtils{}

	usersService := service.NewUsersService(users)
	tasksService := service.NewTasksService(tasks, repoUtils)
	transcriptionService := service.NewTranscriptionService(transcriptions, repoUtils)
	summaryService := service.NewSummaryService(summaries, repoUtils)
	meetingsService := service.NewMeetingsService(meetings, repoUtils, usersService, tasksService, transcriptionService, summaryService, serverConfig)

	processor := NewMeetingProcessorWithBuffer(
		meetingsService,
		tasksService,
		transcriptionService,
		summaryService,
		repoUtils,
		&failingAudioProcessor{attempts: make(map[string]int)},
		fakeLLMClient{},
		serverConfig,
		128,
	)

	return &fakeState{
		processor:      processor,
		meetings:       meetings,
		tasks:          tasks,
		transcriptions: transcriptions,
		summaries:      summaries,
	}
}

// TestProcessMeetingInWorker_RetriesFromFailedStatus verifies that a failed meeting
// keeps its audio file and can be retried successfully.
func TestProcessMeetingInWorker_RetriesFromFailedStatus(t *testing.T) {
	state := newRetryTestState(t)
	dir := t.TempDir()

	meeting := newTestMeeting(t, dir, 0)
	seedMeetingAndTask(t, state, meeting)

	state.processor.processMeetingInWorker(0, meeting)

	status, ok := state.tasks.statusByMeeting(meeting.ID)
	if !ok || status != model.TaskStatusFailed {
		t.Fatalf("expected status failed, got %q (ok=%v)", status, ok)
	}

	if _, err := os.Stat(*meeting.FilePath); err != nil {
		t.Fatalf("audio file should be preserved after failure: %v", err)
	}

	err := state.processor.RetryMeeting(context.Background(), meeting.ID)
	if err != nil {
		t.Fatalf("retry meeting: %v", err)
	}

	state.processor.processMeetingInWorker(0, meeting)

	status, ok = state.tasks.statusByMeeting(meeting.ID)
	if !ok || status != model.TaskStatusCompleted {
		t.Errorf("expected status completed after retry, got %q (ok=%v)", status, ok)
	}

	updatedMeeting, err := state.meetings.GetByID(context.Background(), meeting.ID)
	if err != nil {
		t.Fatalf("get updated meeting: %v", err)
	}
	if updatedMeeting.FilePath != nil && *updatedMeeting.FilePath != "" {
		t.Errorf("audio file path should be cleared after successful completion")
	}
}

// TestRetryMeeting_QueueFull verifies that RetryMeeting returns an error when the queue is full.
func TestRetryMeeting_QueueFull(t *testing.T) {
	state := newRetryTestState(t)
	dir := t.TempDir()

	meeting := newTestMeeting(t, dir, 0)
	seedMeetingAndTask(t, state, meeting)

	state.processor.processMeetingInWorker(0, meeting)

	// Fill the channel to capacity without starting workers.
	for i := 0; i < 128; i++ {
		state.processor.ProcessMeeting(meeting)
	}

	err := state.processor.RetryMeeting(context.Background(), meeting.ID)
	if err == nil || err.Error() != "processing queue is full" {
		t.Errorf("expected queue full error, got %v", err)
	}
}
