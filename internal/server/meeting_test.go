package server

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
	"unsafe"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pashagolub/pgxmock/v5"
	"github.com/scouser-122/meeting-analyzer/internal/config"
	"github.com/scouser-122/meeting-analyzer/internal/domain/model"
	"github.com/scouser-122/meeting-analyzer/internal/repository/postgres"
	"github.com/scouser-122/meeting-analyzer/internal/service"
	"github.com/scouser-122/meeting-analyzer/internal/storage/memory"
	"github.com/scouser-122/meeting-analyzer/internal/worker"
)

// meetingProcessorForTest создаёт заглушку MeetingProcessor с буферизованным каналом,
// чтобы можно было контролировать сценарий "канал заполнен".
type meetingProcessorForTest struct {
	channel chan *model.Meeting
}

func newMeetingProcessorForTest(bufferSize int) *meetingProcessorForTest {
	return &meetingProcessorForTest{channel: make(chan *model.Meeting, bufferSize)}
}

func (m *meetingProcessorForTest) ProcessMeeting(meeting *model.Meeting) bool {
	select {
	case m.channel <- meeting:
		return true
	default:
		return false
	}
}

// compile-time проверка соответствия сигнатуре метода оригинального MeetingProcessor.
var _ interface{ ProcessMeeting(*model.Meeting) bool } = (*meetingProcessorForTest)(nil)

// newMeetingProcessorWithBuffer создаёт реальный MeetingProcessor с заданным размером канала.
// Используется для теста сценария "канал заполнен".
func newMeetingProcessorWithBuffer(
	t *testing.T,
	mockDB pgxmock.PgxPoolIface,
	uploadDir string,
	bufferSize int,
) *worker.MeetingProcessor {
	t.Helper()

	serverConfig, fileStorage := newTestServerConfigAndStorage(uploadDir)

	db := &postgres.PostgresDatabase{}
	repoUtils := postgres.NewPostgresRepositoryUtils(db)
	userRepo := postgres.NewPostgresUserRepositoryFromPool(mockDB)
	usersService := service.NewUsersService(userRepo)
	meetingRepo := postgres.NewPostgresMeetingRepositoryFromPool(mockDB)
	taskRepo := postgres.NewPostgresTaskRepositoryFromPool(mockDB)
	transcriptionRepo := postgres.NewPostgresTranscriptionRepository(db)
	tasksService := service.NewTasksService(taskRepo, repoUtils)
	transcriptionService := service.NewTranscriptionService(transcriptionRepo, repoUtils)
	summaryRepo := postgres.NewPostgresSummaryRepository(db)
	summaryService := service.NewSummaryService(summaryRepo, repoUtils)
	meetingsService := service.NewMeetingsService(
		meetingRepo,
		repoUtils,
		usersService,
		tasksService,
		transcriptionService,
		summaryService,
		serverConfig,
		fileStorage,
	)

	return worker.NewMeetingProcessorWithBuffer(
		meetingsService,
		tasksService,
		nil,
		nil,
		repoUtils,
		nil,
		nil,
		serverConfig,
		bufferSize,
	)
}

// buildLoadRequest формирует multipart/form-data запрос для загрузки файла.
func buildLoadRequest(t *testing.T, metadata string, filename string, fileContent []byte) (*http.Request, error) {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	if metadata != "" {
		if err := writer.WriteField("metadata", metadata); err != nil {
			return nil, err
		}
	}

	if filename != "" {
		part, err := writer.CreateFormFile("file", filename)
		if err != nil {
			return nil, err
		}
		if _, err := part.Write(fileContent); err != nil {
			return nil, err
		}
	}

	if err := writer.Close(); err != nil {
		return nil, err
	}

	req := httptest.NewRequest(http.MethodPost, "/api/meetings/load", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req, nil
}

// newTestMeetingsHandler собирает обработчик с реальными сервисами и репозиториями на моке БД.
// Updated to support full services for HandleList tests (summary, transcription).
func newTestMeetingsHandler(
	t *testing.T,
	mockDB pgxmock.PgxPoolIface,
	uploadDir string,
	processor interface{ ProcessMeeting(*model.Meeting) bool },
) *MeetingsHandler {
	t.Helper()

	serverConfig, fileStorage := newTestServerConfigAndStorage(uploadDir)

	// repoUtils должен использовать тот же mockDB, чтобы транзакции работали через pgxmock.
	// PostgresDatabase.pool — неэкспортированное поле, поэтому используем unsafe.Pointer.
	db := &postgres.PostgresDatabase{}
	dbValue := reflect.ValueOf(db).Elem()
	poolField := dbValue.FieldByName("pool")
	if poolField.IsValid() && poolField.Kind() == reflect.Interface {
		reflect.NewAt(poolField.Type(), unsafe.Pointer(poolField.UnsafeAddr())).Elem().Set(reflect.ValueOf(mockDB))
	}
	repoUtils := postgres.NewPostgresRepositoryUtils(db)
	userRepo := postgres.NewPostgresUserRepositoryFromPool(mockDB)
	usersService := service.NewUsersService(userRepo)

	meetingRepo := postgres.NewPostgresMeetingRepositoryFromPool(mockDB)
	taskRepo := postgres.NewPostgresTaskRepositoryFromPool(mockDB)
	tasksService := service.NewTasksService(taskRepo, repoUtils)
	summaryRepo := postgres.NewPostgresSummaryRepository(db)
	summaryService := service.NewSummaryService(summaryRepo, repoUtils)
	transcriptionRepo := postgres.NewPostgresTranscriptionRepository(db)
	transcriptionService := service.NewTranscriptionService(transcriptionRepo, repoUtils)

	meetingsService := service.NewMeetingsService(
		meetingRepo,
		repoUtils,
		usersService,
		tasksService,
		transcriptionService,
		summaryService,
		serverConfig,
		fileStorage,
	)

	// Создаём реальный MeetingProcessor через конструктор с буфером по умолчанию.
	mp := worker.NewMeetingProcessorWithBuffer(
		meetingsService,
		tasksService,
		transcriptionService,
		summaryService,
		repoUtils,
		nil,
		nil,
		serverConfig,
		10,
	)

	// Если передан тестовый processor и это *worker.MeetingProcessor,
	// используем его напрямую вместо созданного выше.
	if realMP, ok := processor.(*worker.MeetingProcessor); ok {
		mp = realMP
	}

	return NewMeetingsHandler(
		meetingsService,
		tasksService,
		transcriptionService,
		summaryService,
		mp,
		serverConfig,
	)
}

func newTestServerConfigAndStorage(uploadDir string) (*config.ServerConfig, *memory.Storage) {
	serverConfig := &config.ServerConfig{}
	serverConfig.UploadFileDir = new(string)
	*serverConfig.UploadFileDir = uploadDir
	serverConfig.MaxUploadSize = new(int64)
	*serverConfig.MaxUploadSize = 10 << 20 // 10 MB
	serverConfig.ProcessorLimit = new(int)
	*serverConfig.ProcessorLimit = 5
	serverConfig.ProcessorTimeout = new(int)
	*serverConfig.ProcessorTimeout = 30
	return serverConfig, memory.NewStorage()
}

// processorFactory описывает функцию, создающую процессор встречи для теста.
type processorFactory func(t *testing.T, mockDB pgxmock.PgxPoolIface, uploadDir string) interface{ ProcessMeeting(*model.Meeting) bool }

// ─────────────────────────────────────────────────────────────────────────────
// Тесты для HandleLoad
// ─────────────────────────────────────────────────────────────────────────────

var handleLoadTests = []struct {
	name           string
	metadata       string
	filename       string
	fileContent    []byte
	setupMock      func(mock pgxmock.PgxPoolIface)
	processor      processorFactory
	expectedStatus int
	expectedBody   string
}{
	{
		name:        "Success - audio file uploaded and queued",
		metadata:    `{"user_id":"user123","name":"weekly standup"}`,
		filename:    "meeting.mp3",
		fileContent: []byte("fake audio content"),
		setupMock: func(mock pgxmock.PgxPoolIface) {
			// Проверка существования пользователя
			mock.ExpectQuery("SELECT \\* FROM users").
				WithArgs("user123").
				WillReturnRows(
					pgxmock.NewRows([]string{"id", "created_at"}).
						AddRow("user123", time.Now()),
				)

			// Начало транзакции
			mock.ExpectBegin()

			// Создание встречи
			meetingID := uuid.New().String()
			mock.ExpectExec("INSERT INTO meetings").
				WithArgs(pgxmock.AnyArg(), "user123", pgxmock.AnyArg(), pgxmock.AnyArg()).
				WillReturnResult(pgxmock.NewResult("INSERT", 1))
			mock.ExpectQuery("SELECT \\* FROM meetings").
				WithArgs(pgxmock.AnyArg()).
				WillReturnRows(
					pgxmock.NewRows([]string{
						"id", "user_id", "meeting_name", "file_path",
						"original_file_name", "created_at", "updated_at",
					}).AddRow(
						meetingID, "user123", nil, nil, nil, time.Now(), time.Now(),
					),
				)

			// Обновление встречи после сохранения файла
			mock.ExpectExec("UPDATE meetings SET").
				WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
				WillReturnResult(pgxmock.NewResult("UPDATE", 1))

			// Создание задачи
			taskID := uuid.New().String()
			mock.ExpectExec("INSERT INTO tasks").
				WithArgs(pgxmock.AnyArg(), meetingID, pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
				WillReturnResult(pgxmock.NewResult("INSERT", 1))
			mock.ExpectQuery("SELECT \\* FROM tasks").
				WithArgs(pgxmock.AnyArg()).
				WillReturnRows(
					pgxmock.NewRows([]string{"id", "meeting_id", "status", "error_message", "created_at", "updated_at"}).
						AddRow(taskID, meetingID, model.TaskStatusCreated, nil, time.Now(), time.Now()),
				)

			// Коммит транзакции
			mock.ExpectCommit()
		},
		processor: func(t *testing.T, mockDB pgxmock.PgxPoolIface, uploadDir string) interface{ ProcessMeeting(*model.Meeting) bool } {
			return newMeetingProcessorForTest(10)
		},
		expectedStatus: http.StatusAccepted,
		expectedBody:   `{"status":"ok","message":"Файл с записью встречи успешно загружен и запущена его обработка"`,
	},
	{
		name:        "Bad Request - missing 'file' field",
		metadata:    `{"user_id":"user123"}`,
		filename:    "",
		fileContent: nil,
		setupMock: func(mock pgxmock.PgxPoolIface) {
			// Никаких обращений к БД не ожидается
		},
		processor: func(t *testing.T, mockDB pgxmock.PgxPoolIface, uploadDir string) interface{ ProcessMeeting(*model.Meeting) bool } {
			return newMeetingProcessorForTest(10)
		},
		expectedStatus: http.StatusBadRequest,
		expectedBody:   `{"status":"error","message":"В запросе отсутствует файл"}`,
	},
	{
		name:        "Bad Request - invalid metadata JSON",
		metadata:    `{"user_id":}`,
		filename:    "meeting.mp3",
		fileContent: []byte("fake audio content"),
		setupMock: func(mock pgxmock.PgxPoolIface) {
			// Никаких обращений к БД не ожидается
		},
		processor: func(t *testing.T, mockDB pgxmock.PgxPoolIface, uploadDir string) interface{ ProcessMeeting(*model.Meeting) bool } {
			return newMeetingProcessorForTest(10)
		},
		expectedStatus: http.StatusBadRequest,
		expectedBody:   `{"status":"error","message":"Некорректные метаданные в запросе"}`,
	},
	{
		name:        "Bad Request - user id not specified",
		metadata:    `{"name":"meeting without user"}`,
		filename:    "meeting.mp3",
		fileContent: []byte("fake audio content"),
		setupMock: func(mock pgxmock.PgxPoolIface) {
			// Проверка пользователя не выполняется, т.к. user_id пустой
		},
		processor: func(t *testing.T, mockDB pgxmock.PgxPoolIface, uploadDir string) interface{ ProcessMeeting(*model.Meeting) bool } {
			return newMeetingProcessorForTest(10)
		},
		expectedStatus: http.StatusBadRequest,
		expectedBody:   `{"status":"error","message":"Не указан идентификатор пользователя"}`,
	},
	{
		name:        "Bad Request - user not found",
		metadata:    `{"user_id":"unknown_user","name":"meeting"}`,
		filename:    "meeting.mp3",
		fileContent: []byte("fake audio content"),
		setupMock: func(mock pgxmock.PgxPoolIface) {
			mock.ExpectQuery("SELECT \\* FROM users").
				WithArgs("unknown_user").
				WillReturnError(fmt.Errorf("user not found"))
		},
		processor: func(t *testing.T, mockDB pgxmock.PgxPoolIface, uploadDir string) interface{ ProcessMeeting(*model.Meeting) bool } {
			return newMeetingProcessorForTest(10)
		},
		expectedStatus: http.StatusInternalServerError,
		expectedBody:   `{"status":"error","message":"Произошла ошибка при обработке запроса. Попробуйте выполнить команду еще раз"}`,
	},
	{
		name:        "Unsupported Media Type - invalid file extension",
		metadata:    `{"user_id":"user123","name":"meeting"}`,
		filename:    "meeting.docx",
		fileContent: []byte("not an audio file"),
		setupMock: func(mock pgxmock.PgxPoolIface) {
			// Никаких обращений к БД не ожидается
		},
		processor: func(t *testing.T, mockDB pgxmock.PgxPoolIface, uploadDir string) interface{ ProcessMeeting(*model.Meeting) bool } {
			return newMeetingProcessorForTest(10)
		},
		expectedStatus: http.StatusUnsupportedMediaType,
		expectedBody:   `{"status":"error","message":"Неподдерживаемый формат файла. Загрузите аудиофайл (.mp3, .wav, .m4a, .ogg) или текстовую транскрипцию (.txt)"}`,
	},
	{
		name:        "Internal Server Error - database failure during meeting creation",
		metadata:    `{"user_id":"user123","name":"meeting"}`,
		filename:    "meeting.mp3",
		fileContent: []byte("fake audio content"),
		setupMock: func(mock pgxmock.PgxPoolIface) {
			mock.ExpectQuery("SELECT \\* FROM users").
				WithArgs("user123").
				WillReturnRows(
					pgxmock.NewRows([]string{"id", "created_at"}).
						AddRow("user123", time.Now()),
				)
			mock.ExpectBegin()
			mock.ExpectExec("INSERT INTO meetings").
				WithArgs(pgxmock.AnyArg(), "user123", pgxmock.AnyArg(), pgxmock.AnyArg()).
				WillReturnError(fmt.Errorf("connection refused"))
			mock.ExpectRollback()
		},
		processor: func(t *testing.T, mockDB pgxmock.PgxPoolIface, uploadDir string) interface{ ProcessMeeting(*model.Meeting) bool } {
			return newMeetingProcessorForTest(10)
		},
		expectedStatus: http.StatusInternalServerError,
		expectedBody:   `{"status":"error","message":"Произошла ошибка при обработке запроса. Попробуйте выполнить команду еще раз"}`,
	},
	{
		name:        "Too Many Requests - processing channel full",
		metadata:    `{"user_id":"user123","name":"meeting"}`,
		filename:    "meeting.mp3",
		fileContent: []byte("fake audio content"),
		setupMock: func(mock pgxmock.PgxPoolIface) {
			mock.ExpectQuery("SELECT \\* FROM users").
				WithArgs("user123").
				WillReturnRows(
					pgxmock.NewRows([]string{"id", "created_at"}).
						AddRow("user123", time.Now()),
				)
			mock.ExpectBegin()

			meetingID := uuid.New().String()
			mock.ExpectExec("INSERT INTO meetings").
				WithArgs(pgxmock.AnyArg(), "user123", pgxmock.AnyArg(), pgxmock.AnyArg()).
				WillReturnResult(pgxmock.NewResult("INSERT", 1))
			mock.ExpectQuery("SELECT \\* FROM meetings").
				WithArgs(pgxmock.AnyArg()).
				WillReturnRows(
					pgxmock.NewRows([]string{
						"id", "user_id", "meeting_name", "file_path",
						"original_file_name", "created_at", "updated_at",
					}).AddRow(
						meetingID, "user123", nil, nil, nil, time.Now(), time.Now(),
					),
				)

			mock.ExpectExec("UPDATE meetings SET").
				WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
				WillReturnResult(pgxmock.NewResult("UPDATE", 1))

			taskID := uuid.New().String()
			mock.ExpectExec("INSERT INTO tasks").
				WithArgs(pgxmock.AnyArg(), meetingID, pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
				WillReturnResult(pgxmock.NewResult("INSERT", 1))
			mock.ExpectQuery("SELECT \\* FROM tasks").
				WithArgs(pgxmock.AnyArg()).
				WillReturnRows(
					pgxmock.NewRows([]string{"id", "meeting_id", "status", "error_message", "created_at", "updated_at"}).
						AddRow(taskID, meetingID, model.TaskStatusCreated, nil, time.Now(), time.Now()),
				)

			mock.ExpectCommit()
		},
		processor: func(t *testing.T, mockDB pgxmock.PgxPoolIface, uploadDir string) interface{ ProcessMeeting(*model.Meeting) bool } {
			// Создаём реальный MeetingProcessor с нулевым буфером канала,
			// чтобы имитировать заполненность и получить StatusTooManyRequests.
			return newMeetingProcessorWithBuffer(t, mockDB, uploadDir, 0)
		},
		expectedStatus: http.StatusTooManyRequests,
		expectedBody:   `{"status":"error","message":"Сервер перегружен. Попробуйте загрузить файл позже"}`,
	},
}

func TestMeetingsHandler_HandleLoad(t *testing.T) {
	for _, tt := range handleLoadTests {
		t.Run(tt.name, func(t *testing.T) {
			mockDB, err := pgxmock.NewPool()
			if err != nil {
				t.Fatalf("failed to create mock pool: %v", err)
			}
			defer mockDB.Close()

			if tt.setupMock != nil {
				tt.setupMock(mockDB)
			}

			uploadDir, err := os.MkdirTemp("", "meeting-uploads-*")
			if err != nil {
				t.Fatalf("failed to create temp upload dir: %v", err)
			}
			defer os.RemoveAll(uploadDir)

			processor := tt.processor(t, mockDB, uploadDir)
			handler := newTestMeetingsHandler(t, mockDB, uploadDir, processor)

			req, err := buildLoadRequest(t, tt.metadata, tt.filename, tt.fileContent)
			if err != nil {
				t.Fatalf("failed to build load request: %v", err)
			}

			// Для теста "Payload Too Large" нужно, чтобы Content-Length был известен,
			// httptest.NewRequest уже устанавливает Content-Length.
			rr := httptest.NewRecorder()
			handler.HandleLoad(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rr.Code)
			}

			responseBody := strings.TrimSpace(rr.Body.String())
			if tt.expectedBody != "" {
				if !strings.HasPrefix(responseBody, tt.expectedBody) {
					t.Errorf("expected body to start with %q, got %q", tt.expectedBody, responseBody)
				}
			}

			if err := mockDB.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

// Дополнительный тест: проверка, что загруженный файл действительно сохраняется в хранилище.
func TestMeetingsHandler_HandleLoad_SavesFileToStorage(t *testing.T) {
	mockDB, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock pool: %v", err)
	}
	defer mockDB.Close()

	userID := "user123"
	meetingName := "important meeting"
	meetingID := uuid.New().String()
	taskID := uuid.New().String()
	now := time.Now()

	mockDB.ExpectQuery("SELECT \\* FROM users").
		WithArgs(userID).
		WillReturnRows(
			pgxmock.NewRows([]string{"id", "created_at"}).
				AddRow(userID, now),
		)
	mockDB.ExpectBegin()
	mockDB.ExpectExec("INSERT INTO meetings").
		WithArgs(pgxmock.AnyArg(), userID, pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mockDB.ExpectQuery("SELECT \\* FROM meetings").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(
			pgxmock.NewRows([]string{
				"id", "user_id", "meeting_name", "file_path",
				"original_file_name", "created_at", "updated_at",
			}).AddRow(
				meetingID, userID, nil, nil, nil, now, now,
			),
		)
	mockDB.ExpectExec("UPDATE meetings SET").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	mockDB.ExpectExec("INSERT INTO tasks").
		WithArgs(pgxmock.AnyArg(), meetingID, pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mockDB.ExpectQuery("SELECT \\* FROM tasks").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(
			pgxmock.NewRows([]string{"id", "meeting_id", "status", "error_message", "created_at", "updated_at"}).
				AddRow(taskID, meetingID, model.TaskStatusCreated, nil, now, now),
		)
	mockDB.ExpectCommit()

	uploadDir, err := os.MkdirTemp("", "meeting-uploads-*")
	if err != nil {
		t.Fatalf("failed to create temp upload dir: %v", err)
	}
	defer os.RemoveAll(uploadDir)

	processor := newMeetingProcessorForTest(10)
	handler := newTestMeetingsHandler(t, mockDB, uploadDir, processor)

	content := []byte("test audio bytes")
	req, err := buildLoadRequest(t, fmt.Sprintf(`{"user_id":"%s","name":"%s"}`, userID, meetingName), "meeting.mp3", content)
	if err != nil {
		t.Fatalf("failed to build load request: %v", err)
	}

	rr := httptest.NewRecorder()
	handler.HandleLoad(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d", http.StatusAccepted, rr.Code)
	}

	memStorage, ok := handler.meetingsService.FileStorage().(*memory.Storage)
	if !ok {
		t.Fatalf("expected memory storage")
	}

	var found bool
	for key, storedContent := range memStorage.GetAll() {
		if bytes.Equal(storedContent, content) {
			found = true
			t.Logf("file saved to storage with key: %s", key)
			break
		}
	}
	if !found {
		t.Errorf("uploaded file content not found in storage")
	}

	if err := mockDB.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// Вспомогательные заглушки для pgxmock, чтобы избежать предупреждений линтера.
var _ = pgconn.CommandTag{}
var _ = io.Discard

// ─────────────────────────────────────────────────────────────────────────────
// Тесты для HandleList
// ─────────────────────────────────────────────────────────────────────────────

type when struct {
	method    string
	userID    string
	setupMock func(mock pgxmock.PgxPoolIface)
}

type want struct {
	status int
	body   string // подстрока, которую ожидаем в теле ответа
}

var handleListTests = []struct {
	name string
	when when
	want want
}{
	{
		name: "Method Not Allowed - POST instead of GET",
		when: when{
			method: http.MethodPost,
			userID: "user1",
			setupMock: func(mock pgxmock.PgxPoolIface) {
				// Никаких обращений к БД не ожидается
			},
		},
		want: want{
			status: http.StatusMethodNotAllowed,
			body:   "",
		},
	},
	{
		name: "Success - empty meetings list",
		when: when{
			method: http.MethodGet,
			userID: "user1",
			setupMock: func(mock pgxmock.PgxPoolIface) {
				// GetAllByUserID → GetAllConditional: первый запрос возвращает пустой результат
				mock.ExpectQuery("SELECT \\* FROM meetings").
					WithArgs("user1", 10, 0).
					WillReturnRows(pgxmock.NewRows([]string{
						"id", "user_id", "meeting_name", "file_path",
						"original_file_name", "created_at", "updated_at",
					}))
			},
		},
		want: want{
			status: http.StatusOK,
			body:   "[]",
		},
	},
	{
		name: "Success - meetings with status and summary",
		when: when{
			method: http.MethodGet,
			userID: "user2",
			setupMock: func(mock pgxmock.PgxPoolIface) {
				meetingID1 := "meeting-id-1"
				meetingID2 := "meeting-id-2"
				taskID1 := "task-id-1"
				taskID2 := "task-id-2"
				summaryID1 := "summary-id-1"
				summaryID2 := "summary-id-2"
				now := time.Now()
				name1 := "Standup"
				name2 := "Retro"
				summaryText1 := "Summary of standup"
				summaryText2 := "Summary of retro"

				// GetAllByUserID → первая страница с двумя встречами
				mock.ExpectQuery("SELECT \\* FROM meetings").
					WithArgs("user2", 10, 0).
					WillReturnRows(pgxmock.NewRows([]string{
						"id", "user_id", "meeting_name", "file_path",
						"original_file_name", "created_at", "updated_at",
					}).
						AddRow(meetingID1, "user2", &name1, nil, nil, now, now).
						AddRow(meetingID2, "user2", &name2, nil, nil, now, now),
					)
				// GetAllByUserID → вторая страница пустая (конец пагинации)
				mock.ExpectQuery("SELECT \\* FROM meetings").
					WithArgs("user2", 10, 10).
					WillReturnRows(pgxmock.NewRows([]string{
						"id", "user_id", "meeting_name", "file_path",
						"original_file_name", "created_at", "updated_at",
					}))

				// GetStatus для meeting1 → GetByMeetingID → GetByParameter
				mock.ExpectQuery("SELECT \\* FROM tasks").
					WithArgs(meetingID1).
					WillReturnRows(pgxmock.NewRows([]string{
						"id", "meeting_id", "status", "error_message", "created_at", "updated_at",
					}).AddRow(taskID1, meetingID1, model.TaskStatusCompleted, nil, now, now))

				// GetSummary для meeting1 → GetByMeetingID → GetByParameter
				mock.ExpectQuery("SELECT \\* FROM summary").
					WithArgs(meetingID1).
					WillReturnRows(pgxmock.NewRows([]string{
						"id", "meeting_id", "user_id", "text", "created_at", "search_vector",
					}).AddRow(summaryID1, meetingID1, "user2", summaryText1, now, nil))

				// GetStatus для meeting2
				mock.ExpectQuery("SELECT \\* FROM tasks").
					WithArgs(meetingID2).
					WillReturnRows(pgxmock.NewRows([]string{
						"id", "meeting_id", "status", "error_message", "created_at", "updated_at",
					}).AddRow(taskID2, meetingID2, model.TaskStatusCompleted, nil, now, now))

				// GetSummary для meeting2
				mock.ExpectQuery("SELECT \\* FROM summary").
					WithArgs(meetingID2).
					WillReturnRows(pgxmock.NewRows([]string{
						"id", "meeting_id", "user_id", "text", "created_at", "search_vector",
					}).AddRow(summaryID2, meetingID2, "user2", summaryText2, now, nil))
			},
		},
		want: want{
			status: http.StatusOK,
			body:   `"status":"completed"`,
		},
	},
	{
		name: "Success - meeting without summary (summary not found)",
		when: when{
			method: http.MethodGet,
			userID: "user3",
			setupMock: func(mock pgxmock.PgxPoolIface) {
				meetingID := "meeting-id-3"
				taskID := "task-id-3"
				now := time.Now()
				name := "Planning"

				// GetAllByUserID → первая страница с одной встречей
				mock.ExpectQuery("SELECT \\* FROM meetings").
					WithArgs("user3", 10, 0).
					WillReturnRows(pgxmock.NewRows([]string{
						"id", "user_id", "meeting_name", "file_path",
						"original_file_name", "created_at", "updated_at",
					}).AddRow(meetingID, "user3", &name, nil, nil, now, now))
				// GetAllByUserID → вторая страница пустая
				mock.ExpectQuery("SELECT \\* FROM meetings").
					WithArgs("user3", 10, 10).
					WillReturnRows(pgxmock.NewRows([]string{
						"id", "user_id", "meeting_name", "file_path",
						"original_file_name", "created_at", "updated_at",
					}))

				// GetStatus
				mock.ExpectQuery("SELECT \\* FROM tasks").
					WithArgs(meetingID).
					WillReturnRows(pgxmock.NewRows([]string{
						"id", "meeting_id", "status", "error_message", "created_at", "updated_at",
					}).AddRow(taskID, meetingID, model.TaskStatusProcessing, nil, now, now))

				// GetSummary → summary not found (ErrNoRows)
				mock.ExpectQuery("SELECT \\* FROM summary").
					WithArgs(meetingID).
					WillReturnError(pgx.ErrNoRows)
			},
		},
		want: want{
			status: http.StatusOK,
			body:   `"status":"processing"`,
		},
	},
	{
		name: "Internal Server Error - DB error on GetAllByUserID",
		when: when{
			method: http.MethodGet,
			userID: "user4",
			setupMock: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery("SELECT \\* FROM meetings").
					WithArgs("user4", 10, 0).
					WillReturnError(fmt.Errorf("connection refused"))
			},
		},
		want: want{
			status: http.StatusInternalServerError,
			body:   `{"status":"error","message":"Произошла ошибка при обработке запроса. Попробуйте выполнить команду еще раз"}`,
		},
	},
	{
		name: "Internal Server Error - DB error on GetStatus",
		when: when{
			method: http.MethodGet,
			userID: "user5",
			setupMock: func(mock pgxmock.PgxPoolIface) {
				meetingID := "meeting-id-5"
				now := time.Now()
				name := "Meeting5"

				mock.ExpectQuery("SELECT \\* FROM meetings").
					WithArgs("user5", 10, 0).
					WillReturnRows(pgxmock.NewRows([]string{
						"id", "user_id", "meeting_name", "file_path",
						"original_file_name", "created_at", "updated_at",
					}).AddRow(meetingID, "user5", &name, nil, nil, now, now))
				mock.ExpectQuery("SELECT \\* FROM meetings").
					WithArgs("user5", 10, 10).
					WillReturnRows(pgxmock.NewRows([]string{
						"id", "user_id", "meeting_name", "file_path",
						"original_file_name", "created_at", "updated_at",
					}))

				// GetStatus → ошибка БД
				mock.ExpectQuery("SELECT \\* FROM tasks").
					WithArgs(meetingID).
					WillReturnError(fmt.Errorf("db timeout"))
			},
		},
		want: want{
			status: http.StatusInternalServerError,
			body:   `{"status":"error","message":"Произошла ошибка при обработке запроса. Попробуйте выполнить команду еще раз"}`,
		},
	},
	{
		name: "Internal Server Error - DB error on GetSummary",
		when: when{
			method: http.MethodGet,
			userID: "user6",
			setupMock: func(mock pgxmock.PgxPoolIface) {
				meetingID := "meeting-id-6"
				taskID := "task-id-6"
				now := time.Now()
				name := "Meeting6"

				mock.ExpectQuery("SELECT \\* FROM meetings").
					WithArgs("user6", 10, 0).
					WillReturnRows(pgxmock.NewRows([]string{
						"id", "user_id", "meeting_name", "file_path",
						"original_file_name", "created_at", "updated_at",
					}).AddRow(meetingID, "user6", &name, nil, nil, now, now))
				mock.ExpectQuery("SELECT \\* FROM meetings").
					WithArgs("user6", 10, 10).
					WillReturnRows(pgxmock.NewRows([]string{
						"id", "user_id", "meeting_name", "file_path",
						"original_file_name", "created_at", "updated_at",
					}))

				mock.ExpectQuery("SELECT \\* FROM tasks").
					WithArgs(meetingID).
					WillReturnRows(pgxmock.NewRows([]string{
						"id", "meeting_id", "status", "error_message", "created_at", "updated_at",
					}).AddRow(taskID, meetingID, model.TaskStatusCompleted, nil, now, now))

				// GetSummary → неожиданная ошибка (не ErrNoRows)
				mock.ExpectQuery("SELECT \\* FROM summary").
					WithArgs(meetingID).
					WillReturnError(fmt.Errorf("unexpected db error"))
			},
		},
		want: want{
			status: http.StatusInternalServerError,
			body:   `{"status":"error","message":"Произошла ошибка при обработке запроса. Попробуйте выполнить команду еще раз"}`,
		},
	},
}

func TestMeetingsHandler_HandleList(t *testing.T) {
	for _, tt := range handleListTests {
		t.Run(tt.name, func(t *testing.T) {
			// setup
			mockDB, err := pgxmock.NewPool()
			if err != nil {
				t.Fatalf("failed to create mock pool: %v", err)
			}
			defer mockDB.Close()

			if tt.when.setupMock != nil {
				tt.when.setupMock(mockDB)
			}

			uploadDir, err := os.MkdirTemp("", "meeting-list-test-*")
			if err != nil {
				t.Fatalf("failed to create temp upload dir: %v", err)
			}
			defer os.RemoveAll(uploadDir)

			handler := newTestMeetingsHandler(t, mockDB, uploadDir, newMeetingProcessorForTest(10))

			// when
			url := "/api/meetings/list"
			if tt.when.userID != "" {
				url += "?user_id=" + tt.when.userID
			}
			req := httptest.NewRequest(tt.when.method, url, nil)
			rr := httptest.NewRecorder()
			handler.HandleList(rr, req)

			// проверка ожидаемого результата
			if rr.Code != tt.want.status {
				t.Errorf("expected status %d, got %d", tt.want.status, rr.Code)
			}

			if tt.want.body != "" {
				responseBody := strings.TrimSpace(rr.Body.String())
				if !strings.Contains(responseBody, tt.want.body) {
					t.Errorf("expected body to contain %q, got %q", tt.want.body, responseBody)
				}
			}

			if err := mockDB.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled mock expectations: %v", err)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Тесты для HandleStatus
// ─────────────────────────────────────────────────────────────────────────────

type whenStatus struct {
	method    string
	meetingID string
	userID    string
	setupMock func(mock pgxmock.PgxPoolIface)
}

var handleStatusTests = []struct {
	name string
	when whenStatus
	want want
}{
	{
		name: "Method Not Allowed - POST instead of GET",
		when: whenStatus{
			method:    http.MethodPost,
			meetingID: "meeting-1",
			userID:    "user-1",
			setupMock: func(mock pgxmock.PgxPoolIface) {
				// Никаких обращений к БД не ожидается
			},
		},
		want: want{
			status: http.StatusMethodNotAllowed,
			body:   "",
		},
	},
	{
		name: "Success - meeting status returned",
		when: whenStatus{
			method:    http.MethodGet,
			meetingID: "meeting-1",
			userID:    "user-1",
			setupMock: func(mock pgxmock.PgxPoolIface) {
				now := time.Now()
				meetingID := "meeting-1"
				userID := "user-1"
				taskID := "task-1"
				name := "Weekly Standup"

				// GetByID → SELECT * FROM meetings WHERE id = $1
				mock.ExpectQuery("SELECT \\* FROM meetings").
					WithArgs(meetingID).
					WillReturnRows(pgxmock.NewRows([]string{
						"id", "user_id", "meeting_name", "file_path",
						"original_file_name", "created_at", "updated_at",
					}).AddRow(meetingID, userID, &name, nil, nil, now, now))

				// GetByMeetingID → SELECT * FROM tasks WHERE meeting_id = $1
				mock.ExpectQuery("SELECT \\* FROM tasks").
					WithArgs(meetingID).
					WillReturnRows(pgxmock.NewRows([]string{
						"id", "meeting_id", "status", "error_message", "created_at", "updated_at",
					}).AddRow(taskID, meetingID, model.TaskStatusCompleted, nil, now, now))
			},
		},
		want: want{
			status: http.StatusOK,
			body:   `"status":"completed"`,
		},
	},
	{
		name: "Success - meeting in processing status with error message",
		when: whenStatus{
			method:    http.MethodGet,
			meetingID: "meeting-2",
			userID:    "user-2",
			setupMock: func(mock pgxmock.PgxPoolIface) {
				now := time.Now()
				meetingID := "meeting-2"
				userID := "user-2"
				taskID := "task-2"
				name := "Retro"
				errMsg := "transcription failed"

				mock.ExpectQuery("SELECT \\* FROM meetings").
					WithArgs(meetingID).
					WillReturnRows(pgxmock.NewRows([]string{
						"id", "user_id", "meeting_name", "file_path",
						"original_file_name", "created_at", "updated_at",
					}).AddRow(meetingID, userID, &name, nil, nil, now, now))

				mock.ExpectQuery("SELECT \\* FROM tasks").
					WithArgs(meetingID).
					WillReturnRows(pgxmock.NewRows([]string{
						"id", "meeting_id", "status", "error_message", "created_at", "updated_at",
					}).AddRow(taskID, meetingID, model.TaskStatusFailed, &errMsg, now, now))
			},
		},
		want: want{
			status: http.StatusOK,
			body:   `"status":"failed"`,
		},
	},
	{
		name: "Not Found - meeting does not exist",
		when: whenStatus{
			method:    http.MethodGet,
			meetingID: "nonexistent-meeting",
			userID:    "user-1",
			setupMock: func(mock pgxmock.PgxPoolIface) {
				// GetByID → pgx.ErrNoRows → CustomErr{404}
				mock.ExpectQuery("SELECT \\* FROM meetings").
					WithArgs("nonexistent-meeting").
					WillReturnError(pgx.ErrNoRows)
			},
		},
		want: want{
			status: http.StatusNotFound,
			body:   `{"status":"error","message":"Встреча не найдена"}`,
		},
	},
	{
		name: "Forbidden - meeting belongs to another user",
		when: whenStatus{
			method:    http.MethodGet,
			meetingID: "meeting-3",
			userID:    "wrong-user",
			setupMock: func(mock pgxmock.PgxPoolIface) {
				now := time.Now()
				meetingID := "meeting-3"
				name := "Planning"

				// Встреча принадлежит "owner-user", а запрос от "wrong-user"
				mock.ExpectQuery("SELECT \\* FROM meetings").
					WithArgs(meetingID).
					WillReturnRows(pgxmock.NewRows([]string{
						"id", "user_id", "meeting_name", "file_path",
						"original_file_name", "created_at", "updated_at",
					}).AddRow(meetingID, "owner-user", &name, nil, nil, now, now))
			},
		},
		want: want{
			status: http.StatusForbidden,
			body:   `{"status":"error","message":"У вас нет доступа к этой встрече"}`,
		},
	},
	{
		name: "Internal Server Error - DB error on GetByID",
		when: whenStatus{
			method:    http.MethodGet,
			meetingID: "meeting-4",
			userID:    "user-4",
			setupMock: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery("SELECT \\* FROM meetings").
					WithArgs("meeting-4").
					WillReturnError(fmt.Errorf("connection refused"))
			},
		},
		want: want{
			status: http.StatusInternalServerError,
			body:   `{"status":"error","message":"Произошла ошибка при обработке запроса. Попробуйте выполнить команду еще раз"}`,
		},
	},
	{
		name: "Internal Server Error - DB error on GetByMeetingID (task)",
		when: whenStatus{
			method:    http.MethodGet,
			meetingID: "meeting-5",
			userID:    "user-5",
			setupMock: func(mock pgxmock.PgxPoolIface) {
				now := time.Now()
				meetingID := "meeting-5"
				userID := "user-5"
				name := "Meeting5"

				mock.ExpectQuery("SELECT \\* FROM meetings").
					WithArgs(meetingID).
					WillReturnRows(pgxmock.NewRows([]string{
						"id", "user_id", "meeting_name", "file_path",
						"original_file_name", "created_at", "updated_at",
					}).AddRow(meetingID, userID, &name, nil, nil, now, now))

				// GetByMeetingID → ошибка БД
				mock.ExpectQuery("SELECT \\* FROM tasks").
					WithArgs(meetingID).
					WillReturnError(fmt.Errorf("db timeout"))
			},
		},
		want: want{
			status: http.StatusInternalServerError,
			body:   `{"status":"error","message":"Произошла ошибка при обработке запроса. Попробуйте выполнить команду еще раз"}`,
		},
	},
}

func TestMeetingsHandler_HandleStatus(t *testing.T) {
	for _, tt := range handleStatusTests {
		t.Run(tt.name, func(t *testing.T) {
			// setup
			mockDB, err := pgxmock.NewPool()
			if err != nil {
				t.Fatalf("failed to create mock pool: %v", err)
			}
			defer mockDB.Close()

			if tt.when.setupMock != nil {
				tt.when.setupMock(mockDB)
			}

			uploadDir, err := os.MkdirTemp("", "meeting-status-test-*")
			if err != nil {
				t.Fatalf("failed to create temp upload dir: %v", err)
			}
			defer os.RemoveAll(uploadDir)

			handler := newTestMeetingsHandler(t, mockDB, uploadDir, newMeetingProcessorForTest(10))

			// when
			url := "/api/meetings/status"
			params := []string{}
			if tt.when.meetingID != "" {
				params = append(params, "meeting_id="+tt.when.meetingID)
			}
			if tt.when.userID != "" {
				params = append(params, "user_id="+tt.when.userID)
			}
			if len(params) > 0 {
				url += "?" + strings.Join(params, "&")
			}
			req := httptest.NewRequest(tt.when.method, url, nil)
			rr := httptest.NewRecorder()
			handler.HandleStatus(rr, req)

			// проверка ожидаемого результата
			if rr.Code != tt.want.status {
				t.Errorf("expected status %d, got %d", tt.want.status, rr.Code)
			}

			if tt.want.body != "" {
				responseBody := strings.TrimSpace(rr.Body.String())
				if !strings.Contains(responseBody, tt.want.body) {
					t.Errorf("expected body to contain %q, got %q", tt.want.body, responseBody)
				}
			}

			if err := mockDB.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled mock expectations: %v", err)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Тесты для HandleTranscription
// ─────────────────────────────────────────────────────────────────────────────

type whenTranscription struct {
	method    string
	meetingID string
	userID    string
	setupMock func(mock pgxmock.PgxPoolIface)
}

var handleTranscriptionTests = []struct {
	name string
	when whenTranscription
	want want
}{
	{
		name: "Method Not Allowed - POST instead of GET",
		when: whenTranscription{
			method:    http.MethodPost,
			meetingID: "meeting-1",
			userID:    "user-1",
			setupMock: func(mock pgxmock.PgxPoolIface) {
				// Никаких обращений к БД не ожидается
			},
		},
		want: want{
			status: http.StatusMethodNotAllowed,
			body:   "",
		},
	},
	{
		name: "Success - transcription text returned",
		when: whenTranscription{
			method:    http.MethodGet,
			meetingID: "meeting-1",
			userID:    "user-1",
			setupMock: func(mock pgxmock.PgxPoolIface) {
				now := time.Now()
				meetingID := "meeting-1"
				userID := "user-1"
				transcriptionID := "transcription-1"
				name := "Weekly Standup"
				text := "Обсудили план на неделю. Задачи распределены."

				// GetByID → SELECT * FROM meetings WHERE id = $1
				mock.ExpectQuery("SELECT \\* FROM meetings").
					WithArgs(meetingID).
					WillReturnRows(pgxmock.NewRows([]string{
						"id", "user_id", "meeting_name", "file_path",
						"original_file_name", "created_at", "updated_at",
					}).AddRow(meetingID, userID, &name, nil, nil, now, now))

				// GetByMeetingID → SELECT * FROM transcriptions WHERE meeting_id = $1
				mock.ExpectQuery("SELECT \\* FROM transcriptions").
					WithArgs(meetingID).
					WillReturnRows(pgxmock.NewRows([]string{
						"id", "meeting_id", "user_id", "text", "created_at", "search_vector",
					}).AddRow(transcriptionID, meetingID, userID, text, now, nil))
			},
		},
		want: want{
			status: http.StatusOK,
			body:   `"text":"Обсудили план на неделю. Задачи распределены."`,
		},
	},
	{
		name: "Not Found - meeting does not exist",
		when: whenTranscription{
			method:    http.MethodGet,
			meetingID: "nonexistent-meeting",
			userID:    "user-1",
			setupMock: func(mock pgxmock.PgxPoolIface) {
				// GetByID → pgx.ErrNoRows → CustomErr{404}
				mock.ExpectQuery("SELECT \\* FROM meetings").
					WithArgs("nonexistent-meeting").
					WillReturnError(pgx.ErrNoRows)
			},
		},
		want: want{
			status: http.StatusNotFound,
			body:   `{"status":"error","message":"Встреча не найдена"}`,
		},
	},
	{
		name: "Forbidden - meeting belongs to another user",
		when: whenTranscription{
			method:    http.MethodGet,
			meetingID: "meeting-2",
			userID:    "wrong-user",
			setupMock: func(mock pgxmock.PgxPoolIface) {
				now := time.Now()
				meetingID := "meeting-2"
				name := "Planning"

				// Встреча принадлежит "owner-user", а запрос от "wrong-user"
				mock.ExpectQuery("SELECT \\* FROM meetings").
					WithArgs(meetingID).
					WillReturnRows(pgxmock.NewRows([]string{
						"id", "user_id", "meeting_name", "file_path",
						"original_file_name", "created_at", "updated_at",
					}).AddRow(meetingID, "owner-user", &name, nil, nil, now, now))
			},
		},
		want: want{
			status: http.StatusForbidden,
			body:   `{"status":"error","message":"У вас нет доступа к этой встрече"}`,
		},
	},
	{
		name: "Not Found - transcription does not exist yet",
		when: whenTranscription{
			method:    http.MethodGet,
			meetingID: "meeting-3",
			userID:    "user-3",
			setupMock: func(mock pgxmock.PgxPoolIface) {
				now := time.Now()
				meetingID := "meeting-3"
				userID := "user-3"
				name := "Retro"

				mock.ExpectQuery("SELECT \\* FROM meetings").
					WithArgs(meetingID).
					WillReturnRows(pgxmock.NewRows([]string{
						"id", "user_id", "meeting_name", "file_path",
						"original_file_name", "created_at", "updated_at",
					}).AddRow(meetingID, userID, &name, nil, nil, now, now))

				// GetByMeetingID → ErrNoRows → CustomErr{404, "Транскрипция не найдена"}
				mock.ExpectQuery("SELECT \\* FROM transcriptions").
					WithArgs(meetingID).
					WillReturnError(pgx.ErrNoRows)
			},
		},
		want: want{
			status: http.StatusNotFound,
			body:   `{"status":"error","message":"Транскрипция не найдена"}`,
		},
	},
	{
		name: "Internal Server Error - DB error on GetByID (meeting)",
		when: whenTranscription{
			method:    http.MethodGet,
			meetingID: "meeting-4",
			userID:    "user-4",
			setupMock: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery("SELECT \\* FROM meetings").
					WithArgs("meeting-4").
					WillReturnError(fmt.Errorf("connection refused"))
			},
		},
		want: want{
			status: http.StatusInternalServerError,
			body:   `{"status":"error","message":"Произошла ошибка при обработке запроса. Попробуйте выполнить команду еще раз"}`,
		},
	},
	{
		name: "Internal Server Error - DB error on GetByMeetingID (transcription)",
		when: whenTranscription{
			method:    http.MethodGet,
			meetingID: "meeting-5",
			userID:    "user-5",
			setupMock: func(mock pgxmock.PgxPoolIface) {
				now := time.Now()
				meetingID := "meeting-5"
				userID := "user-5"
				name := "Meeting5"

				mock.ExpectQuery("SELECT \\* FROM meetings").
					WithArgs(meetingID).
					WillReturnRows(pgxmock.NewRows([]string{
						"id", "user_id", "meeting_name", "file_path",
						"original_file_name", "created_at", "updated_at",
					}).AddRow(meetingID, userID, &name, nil, nil, now, now))

				// GetByMeetingID → неожиданная ошибка БД
				mock.ExpectQuery("SELECT \\* FROM transcriptions").
					WithArgs(meetingID).
					WillReturnError(fmt.Errorf("db timeout"))
			},
		},
		want: want{
			status: http.StatusInternalServerError,
			body:   `{"status":"error","message":"Произошла ошибка при обработке запроса. Попробуйте выполнить команду еще раз"}`,
		},
	},
}

func TestMeetingsHandler_HandleTranscription(t *testing.T) {
	for _, tt := range handleTranscriptionTests {
		t.Run(tt.name, func(t *testing.T) {
			// setup
			mockDB, err := pgxmock.NewPool()
			if err != nil {
				t.Fatalf("failed to create mock pool: %v", err)
			}
			defer mockDB.Close()

			if tt.when.setupMock != nil {
				tt.when.setupMock(mockDB)
			}

			uploadDir, err := os.MkdirTemp("", "meeting-transcription-test-*")
			if err != nil {
				t.Fatalf("failed to create temp upload dir: %v", err)
			}
			defer os.RemoveAll(uploadDir)

			handler := newTestMeetingsHandler(t, mockDB, uploadDir, newMeetingProcessorForTest(10))

			// when
			url := "/api/meetings/transcription"
			params := []string{}
			if tt.when.meetingID != "" {
				params = append(params, "meeting_id="+tt.when.meetingID)
			}
			if tt.when.userID != "" {
				params = append(params, "user_id="+tt.when.userID)
			}
			if len(params) > 0 {
				url += "?" + strings.Join(params, "&")
			}
			req := httptest.NewRequest(tt.when.method, url, nil)
			rr := httptest.NewRecorder()
			handler.HandleTranscription(rr, req)

			// проверка ожидаемого результата
			if rr.Code != tt.want.status {
				t.Errorf("expected status %d, got %d", tt.want.status, rr.Code)
			}

			if tt.want.body != "" {
				responseBody := strings.TrimSpace(rr.Body.String())
				if !strings.Contains(responseBody, tt.want.body) {
					t.Errorf("expected body to contain %q, got %q", tt.want.body, responseBody)
				}
			}

			if err := mockDB.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled mock expectations: %v", err)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Тесты для HandleFind
// ─────────────────────────────────────────────────────────────────────────────

// meetingCols — колонки таблицы meetings для pgxmock
var meetingCols = []string{
	"id", "user_id", "meeting_name", "file_path",
	"original_file_name", "created_at", "updated_at",
}

// taskCols — колонки таблицы tasks для pgxmock
var taskCols = []string{
	"id", "meeting_id", "status", "error_message", "created_at", "updated_at",
}

// summaryCols — колонки таблицы summary для pgxmock
var summaryCols = []string{
	"id", "meeting_id", "user_id", "text", "created_at", "search_vector",
}

type whenFind struct {
	method    string
	body      string
	setupMock func(mock pgxmock.PgxPoolIface)
}

var handleFindTests = []struct {
	name string
	when whenFind
	want want
}{
	{
		name: "Method Not Allowed - GET instead of POST",
		when: whenFind{
			method: http.MethodGet,
			body:   `{"user_id":"user1","key_words":"standup"}`,
			setupMock: func(mock pgxmock.PgxPoolIface) {
				// Никаких обращений к БД не ожидается
			},
		},
		want: want{
			status: http.StatusMethodNotAllowed,
			body:   "",
		},
	},
	{
		name: "Bad Request - invalid JSON body",
		when: whenFind{
			method: http.MethodPost,
			body:   `{invalid json}`,
			setupMock: func(mock pgxmock.PgxPoolIface) {
				// Никаких обращений к БД не ожидается
			},
		},
		want: want{
			status: http.StatusBadRequest,
			body:   `{"status":"error"`,
		},
	},
	{
		name: "Success - empty result (no matches anywhere)",
		when: whenFind{
			method: http.MethodPost,
			body:   `{"user_id":"user1","key_words":"nonexistent"}`,
			setupMock: func(mock pgxmock.PgxPoolIface) {
				// FindByNameContains → пустой результат
				mock.ExpectQuery("SELECT \\* FROM meetings").
					WithArgs("user1", "nonexistent", 10, 0).
					WillReturnRows(pgxmock.NewRows(meetingCols))

				// FindByTextContains (transcriptions) → пустой результат
				mock.ExpectQuery("FROM transcriptions").
					WithArgs("user1", "nonexistent").
					WillReturnRows(pgxmock.NewRows([]string{
						"id", "meeting_id", "user_id", "text", "created_at",
					}))

				// FindByTextContains (summary) → пустой результат
				mock.ExpectQuery("FROM summary").
					WithArgs("user1", "nonexistent").
					WillReturnRows(pgxmock.NewRows([]string{
						"id", "meeting_id", "user_id", "text", "created_at",
					}))
			},
		},
		want: want{
			status: http.StatusOK,
			body:   "[]",
		},
	},
	{
		name: "Success - found by meeting name, completed status",
		when: whenFind{
			method: http.MethodPost,
			body:   `{"user_id":"user2","key_words":"standup"}`,
			setupMock: func(mock pgxmock.PgxPoolIface) {
				now := time.Now()
				meetingID := "meeting-find-1"
				taskID := "task-find-1"
				summaryID := "summary-find-1"
				name := "Weekly Standup"
				summaryText := "Обсудили задачи"

				// FindByNameContains → одна встреча
				mock.ExpectQuery("SELECT \\* FROM meetings").
					WithArgs("user2", "standup", 10, 0).
					WillReturnRows(pgxmock.NewRows(meetingCols).
						AddRow(meetingID, "user2", &name, nil, nil, now, now))
				// вторая страница пустая
				mock.ExpectQuery("SELECT \\* FROM meetings").
					WithArgs("user2", "standup", 10, 10).
					WillReturnRows(pgxmock.NewRows(meetingCols))

				// FindByTextContains (transcriptions) → пустой результат
				mock.ExpectQuery("FROM transcriptions").
					WithArgs("user2", "standup").
					WillReturnRows(pgxmock.NewRows([]string{
						"id", "meeting_id", "user_id", "text", "created_at",
					}))

				// FindByTextContains (summary) → пустой результат
				mock.ExpectQuery("FROM summary").
					WithArgs("user2", "standup").
					WillReturnRows(pgxmock.NewRows([]string{
						"id", "meeting_id", "user_id", "text", "created_at",
					}))

				// GetStatus для meeting-find-1 → completed
				mock.ExpectQuery("SELECT \\* FROM tasks").
					WithArgs(meetingID).
					WillReturnRows(pgxmock.NewRows(taskCols).
						AddRow(taskID, meetingID, model.TaskStatusCompleted, nil, now, now))

				// GetSummary для meeting-find-1
				mock.ExpectQuery("SELECT \\* FROM summary").
					WithArgs(meetingID).
					WillReturnRows(pgxmock.NewRows(summaryCols).
						AddRow(summaryID, meetingID, "user2", summaryText, now, nil))
			},
		},
		want: want{
			status: http.StatusOK,
			body:   `"status":"completed"`,
		},
	},
	{
		name: "Success - meeting found but status not completed, excluded from result",
		when: whenFind{
			method: http.MethodPost,
			body:   `{"user_id":"user4","key_words":"retro"}`,
			setupMock: func(mock pgxmock.PgxPoolIface) {
				now := time.Now()
				meetingID := "meeting-find-3"
				taskID := "task-find-3"
				name := "Retro"

				// FindByNameContains → одна встреча
				mock.ExpectQuery("SELECT \\* FROM meetings").
					WithArgs("user4", "retro", 10, 0).
					WillReturnRows(pgxmock.NewRows(meetingCols).
						AddRow(meetingID, "user4", &name, nil, nil, now, now))
				mock.ExpectQuery("SELECT \\* FROM meetings").
					WithArgs("user4", "retro", 10, 10).
					WillReturnRows(pgxmock.NewRows(meetingCols))

				// FindByTextContains (transcriptions) → пустой результат
				mock.ExpectQuery("FROM transcriptions").
					WithArgs("user4", "retro").
					WillReturnRows(pgxmock.NewRows([]string{
						"id", "meeting_id", "user_id", "text", "created_at",
					}))

				// FindByTextContains (summary) → пустой результат
				mock.ExpectQuery("FROM summary").
					WithArgs("user4", "retro").
					WillReturnRows(pgxmock.NewRows([]string{
						"id", "meeting_id", "user_id", "text", "created_at",
					}))

				// GetStatus → processing (не completed → встреча исключается)
				mock.ExpectQuery("SELECT \\* FROM tasks").
					WithArgs(meetingID).
					WillReturnRows(pgxmock.NewRows(taskCols).
						AddRow(taskID, meetingID, model.TaskStatusProcessing, nil, now, now))
			},
		},
		want: want{
			status: http.StatusOK,
			body:   "[]",
		},
	},
	{
		name: "Internal Server Error - DB error on FindByNameContains",
		when: whenFind{
			method: http.MethodPost,
			body:   `{"user_id":"user5","key_words":"test"}`,
			setupMock: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery("SELECT \\* FROM meetings").
					WithArgs("user5", "test", 10, 0).
					WillReturnError(fmt.Errorf("db connection error"))
			},
		},
		want: want{
			status: http.StatusInternalServerError,
			body:   `{"status":"error","message":"Произошла ошибка при обработке запроса. Попробуйте выполнить команду еще раз"}`,
		},
	},
	{
		name: "Internal Server Error - DB error on GetSummary in final loop",
		when: whenFind{
			method: http.MethodPost,
			body:   `{"user_id":"user6","key_words":"test"}`,
			setupMock: func(mock pgxmock.PgxPoolIface) {
				now := time.Now()
				meetingID := "meeting-err-1"
				taskID := "task-err-1"
				name := "Test Meeting"

				// FindByNameContains → одна встреча
				mock.ExpectQuery("SELECT \\* FROM meetings").
					WithArgs("user6", "test", 10, 0).
					WillReturnRows(pgxmock.NewRows(meetingCols).
						AddRow(meetingID, "user6", &name, nil, nil, now, now))
				mock.ExpectQuery("SELECT \\* FROM meetings").
					WithArgs("user6", "test", 10, 10).
					WillReturnRows(pgxmock.NewRows(meetingCols))

				// FindByTextContains (transcriptions) → пустой результат
				mock.ExpectQuery("FROM transcriptions").
					WithArgs("user6", "test").
					WillReturnRows(pgxmock.NewRows([]string{
						"id", "meeting_id", "user_id", "text", "created_at",
					}))

				// FindByTextContains (summary) → пустой результат
				mock.ExpectQuery("FROM summary").
					WithArgs("user6", "test").
					WillReturnRows(pgxmock.NewRows([]string{
						"id", "meeting_id", "user_id", "text", "created_at",
					}))

				// GetStatus → completed
				mock.ExpectQuery("SELECT \\* FROM tasks").
					WithArgs(meetingID).
					WillReturnRows(pgxmock.NewRows(taskCols).
						AddRow(taskID, meetingID, model.TaskStatusCompleted, nil, now, now))

				// GetSummary → неожиданная ошибка БД
				mock.ExpectQuery("SELECT \\* FROM summary").
					WithArgs(meetingID).
					WillReturnError(fmt.Errorf("db timeout"))
			},
		},
		want: want{
			status: http.StatusInternalServerError,
			body:   `{"status":"error","message":"Произошла ошибка при обработке запроса. Попробуйте выполнить команду еще раз"}`,
		},
	},
	{
		name: "Internal Server Error - DB error on GetStatus",
		when: whenFind{
			method: http.MethodPost,
			body:   `{"user_id":"user7","key_words":"meeting"}`,
			setupMock: func(mock pgxmock.PgxPoolIface) {
				now := time.Now()
				meetingID := "meeting-find-7"
				name := "Meeting7"

				// FindByNameContains → одна встреча
				mock.ExpectQuery("SELECT \\* FROM meetings").
					WithArgs("user7", "meeting", 10, 0).
					WillReturnRows(pgxmock.NewRows(meetingCols).
						AddRow(meetingID, "user7", &name, nil, nil, now, now))
				mock.ExpectQuery("SELECT \\* FROM meetings").
					WithArgs("user7", "meeting", 10, 10).
					WillReturnRows(pgxmock.NewRows(meetingCols))

				// FindByTextContains (transcriptions) → пустой результат
				mock.ExpectQuery("FROM transcriptions").
					WithArgs("user7", "meeting").
					WillReturnRows(pgxmock.NewRows([]string{
						"id", "meeting_id", "user_id", "text", "created_at",
					}))

				// FindByTextContains (summary) → пустой результат
				mock.ExpectQuery("FROM summary").
					WithArgs("user7", "meeting").
					WillReturnRows(pgxmock.NewRows([]string{
						"id", "meeting_id", "user_id", "text", "created_at",
					}))

				// GetStatus → ошибка БД
				mock.ExpectQuery("SELECT \\* FROM tasks").
					WithArgs(meetingID).
					WillReturnError(fmt.Errorf("db timeout"))
			},
		},
		want: want{
			status: http.StatusInternalServerError,
			body:   `{"status":"error","message":"Произошла ошибка при обработке запроса. Попробуйте выполнить команду еще раз"}`,
		},
	},
}

// ─────────────────────────────────────────────────────────────────────────────
// Тесты для HandleDelete
// ─────────────────────────────────────────────────────────────────────────────

type whenDelete struct {
	method    string
	meetingID string
	userID    string
	setupMock func(mock pgxmock.PgxPoolIface)
}

var handleDeleteTests = []struct {
	name string
	when whenDelete
	want want
}{
	{
		name: "Method Not Allowed - GET instead of DELETE",
		when: whenDelete{
			method:    http.MethodGet,
			meetingID: "meeting-1",
			userID:    "user-1",
			setupMock: func(mock pgxmock.PgxPoolIface) {
				// Никаких обращений к БД не ожидается
			},
		},
		want: want{
			status: http.StatusMethodNotAllowed,
			body:   "",
		},
	},
	{
		name: "Bad Request - missing meeting_id",
		when: whenDelete{
			method:    http.MethodDelete,
			meetingID: "",
			userID:    "user-1",
			setupMock: func(mock pgxmock.PgxPoolIface) {
				// Никаких обращений к БД не ожидается
			},
		},
		want: want{
			status: http.StatusBadRequest,
			body:   `{"status":"error","message":"Не указан идентификатор встречи"}`,
		},
	},
	{
		name: "Bad Request - missing user_id",
		when: whenDelete{
			method:    http.MethodDelete,
			meetingID: "meeting-1",
			userID:    "",
			setupMock: func(mock pgxmock.PgxPoolIface) {
				// Никаких обращений к БД не ожидается
			},
		},
		want: want{
			status: http.StatusBadRequest,
			body:   `{"status":"error","message":"Не указан идентификатор пользователя"}`,
		},
	},
	{
		name: "Success - meeting deleted",
		when: whenDelete{
			method:    http.MethodDelete,
			meetingID: "meeting-1",
			userID:    "user-1",
			setupMock: func(mock pgxmock.PgxPoolIface) {
				now := time.Now()
				meetingID := "meeting-1"
				userID := "user-1"
				name := "Weekly Standup"

				// GetByID
				mock.ExpectQuery("SELECT \\* FROM meetings").
					WithArgs(meetingID).
					WillReturnRows(pgxmock.NewRows([]string{
						"id", "user_id", "meeting_name", "file_path",
						"original_file_name", "created_at", "updated_at",
					}).AddRow(meetingID, userID, &name, nil, nil, now, now))

				// Delete task
				mock.ExpectExec("DELETE FROM tasks").
					WithArgs(meetingID).
					WillReturnResult(pgxmock.NewResult("DELETE", 1))

				// Delete transcription
				mock.ExpectExec("DELETE FROM transcriptions").
					WithArgs(meetingID).
					WillReturnResult(pgxmock.NewResult("DELETE", 1))

				// Delete summary
				mock.ExpectExec("DELETE FROM summary").
					WithArgs(meetingID).
					WillReturnResult(pgxmock.NewResult("DELETE", 1))

				// Delete meeting
				mock.ExpectExec("DELETE FROM meetings").
					WithArgs(meetingID).
					WillReturnResult(pgxmock.NewResult("DELETE", 1))
			},
		},
		want: want{
			status: http.StatusOK,
			body:   `{"status":"ok","message":"Встреча успешно удалена"}`,
		},
	},
	{
		name: "Not Found - meeting does not exist",
		when: whenDelete{
			method:    http.MethodDelete,
			meetingID: "nonexistent-meeting",
			userID:    "user-1",
			setupMock: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery("SELECT \\* FROM meetings").
					WithArgs("nonexistent-meeting").
					WillReturnError(pgx.ErrNoRows)
			},
		},
		want: want{
			status: http.StatusNotFound,
			body:   `{"status":"error","message":"Встреча не найдена"}`,
		},
	},
	{
		name: "Forbidden - meeting belongs to another user",
		when: whenDelete{
			method:    http.MethodDelete,
			meetingID: "meeting-3",
			userID:    "wrong-user",
			setupMock: func(mock pgxmock.PgxPoolIface) {
				now := time.Now()
				meetingID := "meeting-3"
				name := "Planning"

				mock.ExpectQuery("SELECT \\* FROM meetings").
					WithArgs(meetingID).
					WillReturnRows(pgxmock.NewRows([]string{
						"id", "user_id", "meeting_name", "file_path",
						"original_file_name", "created_at", "updated_at",
					}).AddRow(meetingID, "owner-user", &name, nil, nil, now, now))
			},
		},
		want: want{
			status: http.StatusForbidden,
			body:   `{"status":"error","message":"У вас нет доступа к этой встрече"}`,
		},
	},
	{
		name: "Internal Server Error - DB error on GetByID",
		when: whenDelete{
			method:    http.MethodDelete,
			meetingID: "meeting-4",
			userID:    "user-4",
			setupMock: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery("SELECT \\* FROM meetings").
					WithArgs("meeting-4").
					WillReturnError(fmt.Errorf("connection refused"))
			},
		},
		want: want{
			status: http.StatusInternalServerError,
			body:   `{"status":"error","message":"Произошла ошибка при обработке запроса. Попробуйте выполнить команду еще раз"}`,
		},
	},
}

func TestMeetingsHandler_HandleDelete(t *testing.T) {
	for _, tt := range handleDeleteTests {
		t.Run(tt.name, func(t *testing.T) {
			mockDB, err := pgxmock.NewPool()
			if err != nil {
				t.Fatalf("failed to create mock pool: %v", err)
			}
			defer mockDB.Close()

			if tt.when.setupMock != nil {
				tt.when.setupMock(mockDB)
			}

			uploadDir, err := os.MkdirTemp("", "meeting-delete-test-*")
			if err != nil {
				t.Fatalf("failed to create temp upload dir: %v", err)
			}
			defer os.RemoveAll(uploadDir)

			handler := newTestMeetingsHandler(t, mockDB, uploadDir, newMeetingProcessorForTest(10))

			url := "/api/meetings/delete"
			params := []string{}
			if tt.when.meetingID != "" {
				params = append(params, "meeting_id="+tt.when.meetingID)
			}
			if tt.when.userID != "" {
				params = append(params, "user_id="+tt.when.userID)
			}
			if len(params) > 0 {
				url += "?" + strings.Join(params, "&")
			}
			req := httptest.NewRequest(tt.when.method, url, nil)
			rr := httptest.NewRecorder()
			handler.HandleDelete(rr, req)

			if rr.Code != tt.want.status {
				t.Errorf("expected status %d, got %d", tt.want.status, rr.Code)
			}

			if tt.want.body != "" {
				responseBody := strings.TrimSpace(rr.Body.String())
				if !strings.Contains(responseBody, tt.want.body) {
					t.Errorf("expected body to contain %q, got %q", tt.want.body, responseBody)
				}
			}

			if err := mockDB.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled mock expectations: %v", err)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Тесты для HandleFind
// ─────────────────────────────────────────────────────────────────────────────

// ─────────────────────────────────────────────────────────────────────────────
// Тесты для HandleRetry
// ─────────────────────────────────────────────────────────────────────────────

type whenRetry struct {
	method    string
	body      string
	setupMock func(mock pgxmock.PgxPoolIface)
}

var handleRetryTests = []struct {
	name string
	when whenRetry
	want want
}{
	{
		name: "Method Not Allowed - GET instead of POST",
		when: whenRetry{
			method: http.MethodGet,
			body:   `{"user_id":"user1","meeting_id":"meeting-1"}`,
			setupMock: func(mock pgxmock.PgxPoolIface) {
				// Никаких обращений к БД не ожидается
			},
		},
		want: want{
			status: http.StatusMethodNotAllowed,
			body:   "",
		},
	},
	{
		name: "Bad Request - invalid JSON body",
		when: whenRetry{
			method: http.MethodPost,
			body:   `{invalid json}`,
			setupMock: func(mock pgxmock.PgxPoolIface) {
				// Никаких обращений к БД не ожидается
			},
		},
		want: want{
			status: http.StatusBadRequest,
			body:   `{"status":"error","message":"Некорректное тело запроса"}`,
		},
	},
	{
		name: "Bad Request - missing meeting_id",
		when: whenRetry{
			method: http.MethodPost,
			body:   `{"user_id":"user1"}`,
			setupMock: func(mock pgxmock.PgxPoolIface) {
				// Никаких обращений к БД не ожидается
			},
		},
		want: want{
			status: http.StatusBadRequest,
			body:   `{"status":"error","message":"Не указан идентификатор встречи"}`,
		},
	},
	{
		name: "Bad Request - missing user_id",
		when: whenRetry{
			method: http.MethodPost,
			body:   `{"meeting_id":"meeting-1"}`,
			setupMock: func(mock pgxmock.PgxPoolIface) {
				// Никаких обращений к БД не ожидается
			},
		},
		want: want{
			status: http.StatusBadRequest,
			body:   `{"status":"error","message":"Не указан идентификатор пользователя"}`,
		},
	},
	{
		name: "Not Found - meeting does not exist",
		when: whenRetry{
			method: http.MethodPost,
			body:   `{"user_id":"user1","meeting_id":"nonexistent-meeting"}`,
			setupMock: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery("SELECT \\* FROM meetings").
					WithArgs("nonexistent-meeting").
					WillReturnError(pgx.ErrNoRows)
			},
		},
		want: want{
			status: http.StatusNotFound,
			body:   `{"status":"error","message":"Встреча не найдена"}`,
		},
	},
	{
		name: "Forbidden - meeting belongs to another user",
		when: whenRetry{
			method: http.MethodPost,
			body:   `{"user_id":"wrong-user","meeting_id":"meeting-1"}`,
			setupMock: func(mock pgxmock.PgxPoolIface) {
				now := time.Now()
				meetingID := "meeting-1"
				name := "Planning"

				mock.ExpectQuery("SELECT \\* FROM meetings").
					WithArgs(meetingID).
					WillReturnRows(pgxmock.NewRows([]string{
						"id", "user_id", "meeting_name", "file_path",
						"original_file_name", "created_at", "updated_at",
					}).AddRow(meetingID, "owner-user", &name, nil, nil, now, now))
			},
		},
		want: want{
			status: http.StatusForbidden,
			body:   `{"status":"error","message":"У вас нет доступа к этой встрече"}`,
		},
	},
	{
		name: "Success - retry scheduled for failed meeting",
		when: whenRetry{
			method: http.MethodPost,
			body:   `{"user_id":"user1","meeting_id":"meeting-1"}`,
			setupMock: func(mock pgxmock.PgxPoolIface) {
				now := time.Now()
				meetingID := "meeting-1"
				userID := "user1"
				name := "Weekly Standup"

				mock.ExpectQuery("SELECT \\* FROM meetings").
					WithArgs(meetingID).
					WillReturnRows(pgxmock.NewRows([]string{
						"id", "user_id", "meeting_name", "file_path",
						"original_file_name", "created_at", "updated_at",
					}).AddRow(meetingID, userID, &name, nil, nil, now, now))

				mock.ExpectQuery("SELECT \\* FROM tasks").
					WithArgs(meetingID).
					WillReturnRows(pgxmock.NewRows([]string{
						"id", "meeting_id", "status", "error_message", "created_at", "updated_at",
					}).AddRow("task-1", meetingID, model.TaskStatusFailed, nil, now, now))

				mock.ExpectQuery("SELECT \\* FROM meetings").
					WithArgs(meetingID).
					WillReturnRows(pgxmock.NewRows([]string{
						"id", "user_id", "meeting_name", "file_path",
						"original_file_name", "created_at", "updated_at",
					}).AddRow(meetingID, userID, &name, nil, nil, now, now))
			},
		},
		want: want{
			status: http.StatusAccepted,
			body:   `{"status":"ok","message":"Обработка встречи поставлена в очередь на повторную обработку"}`,
		},
	},
	{
		name: "Conflict - meeting already completed",
		when: whenRetry{
			method: http.MethodPost,
			body:   `{"user_id":"user1","meeting_id":"meeting-1"}`,
			setupMock: func(mock pgxmock.PgxPoolIface) {
				now := time.Now()
				meetingID := "meeting-1"
				userID := "user1"
				name := "Weekly Standup"

				mock.ExpectQuery("SELECT \\* FROM meetings").
					WithArgs(meetingID).
					WillReturnRows(pgxmock.NewRows([]string{
						"id", "user_id", "meeting_name", "file_path",
						"original_file_name", "created_at", "updated_at",
					}).AddRow(meetingID, userID, &name, nil, nil, now, now))

				mock.ExpectQuery("SELECT \\* FROM tasks").
					WithArgs(meetingID).
					WillReturnRows(pgxmock.NewRows([]string{
						"id", "meeting_id", "status", "error_message", "created_at", "updated_at",
					}).AddRow("task-1", meetingID, model.TaskStatusCompleted, nil, now, now))
			},
		},
		want: want{
			status: http.StatusConflict,
			body:   `{"status":"error","message":"Обработка встречи уже завершена"}`,
		},
	},
}

func TestMeetingsHandler_HandleRetry(t *testing.T) {
	for _, tt := range handleRetryTests {
		t.Run(tt.name, func(t *testing.T) {
			mockDB, err := pgxmock.NewPool()
			if err != nil {
				t.Fatalf("failed to create mock pool: %v", err)
			}
			defer mockDB.Close()

			if tt.when.setupMock != nil {
				tt.when.setupMock(mockDB)
			}

			uploadDir, err := os.MkdirTemp("", "meeting-retry-test-*")
			if err != nil {
				t.Fatalf("failed to create temp upload dir: %v", err)
			}
			defer os.RemoveAll(uploadDir)

			handler := newTestMeetingsHandler(t, mockDB, uploadDir, newMeetingProcessorForTest(10))

			req := httptest.NewRequest(tt.when.method, "/api/meetings/retry", strings.NewReader(tt.when.body))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			handler.HandleRetry(rr, req)

			if rr.Code != tt.want.status {
				t.Errorf("expected status %d, got %d", tt.want.status, rr.Code)
			}

			if tt.want.body != "" {
				responseBody := strings.TrimSpace(rr.Body.String())
				if !strings.Contains(responseBody, tt.want.body) {
					t.Errorf("expected body to contain %q, got %q", tt.want.body, responseBody)
				}
			}

			if err := mockDB.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled mock expectations: %v", err)
			}
		})
	}
}

func TestMeetingsHandler_HandleFind(t *testing.T) {
	for _, tt := range handleFindTests {
		t.Run(tt.name, func(t *testing.T) {
			// setup
			mockDB, err := pgxmock.NewPool()
			if err != nil {
				t.Fatalf("failed to create mock pool: %v", err)
			}
			defer mockDB.Close()

			if tt.when.setupMock != nil {
				tt.when.setupMock(mockDB)
			}

			uploadDir, err := os.MkdirTemp("", "meeting-find-test-*")
			if err != nil {
				t.Fatalf("failed to create temp upload dir: %v", err)
			}
			defer os.RemoveAll(uploadDir)

			handler := newTestMeetingsHandler(t, mockDB, uploadDir, newMeetingProcessorForTest(10))

			// when
			req := httptest.NewRequest(tt.when.method, "/api/meetings/find", strings.NewReader(tt.when.body))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			handler.HandleFind(rr, req)

			// проверка ожидаемого результата
			if rr.Code != tt.want.status {
				t.Errorf("expected status %d, got %d", tt.want.status, rr.Code)
			}

			if tt.want.body != "" {
				responseBody := strings.TrimSpace(rr.Body.String())
				if !strings.Contains(responseBody, tt.want.body) {
					t.Errorf("expected body to contain %q, got %q", tt.want.body, responseBody)
				}
			}

			if err := mockDB.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled mock expectations: %v", err)
			}
		})
	}
}
