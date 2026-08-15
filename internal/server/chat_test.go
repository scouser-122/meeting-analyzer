package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
	"unsafe"

	"github.com/pashagolub/pgxmock/v5"
	"github.com/scouser-122/meeting-analyzer/internal/config"
	"github.com/scouser-122/meeting-analyzer/internal/domain/model"
	"github.com/scouser-122/meeting-analyzer/internal/models"
	"github.com/scouser-122/meeting-analyzer/internal/repository/postgres"
	"github.com/scouser-122/meeting-analyzer/internal/service"
)

// ─────────────────────────────────────────────────────────────────────────────
// Заглушка LLM-клиента (внешний REST API)
// ─────────────────────────────────────────────────────────────────────────────

// mockLLMClient — заглушка для client.LLMClient, имитирующая ответы внешнего REST API GigaChat.
type mockLLMClient struct {
	extractIntentFn func(ctx context.Context, text string) (*models.QueryIntent, error)
}

func (m *mockLLMClient) SummarizeTranscription(_ context.Context, _ *model.Meeting, _ string) (string, error) {
	return "", nil
}

func (m *mockLLMClient) ExtractIntent(ctx context.Context, text string) (*models.QueryIntent, error) {
	return m.extractIntentFn(ctx, text)
}

// ─────────────────────────────────────────────────────────────────────────────
// Вспомогательная функция сборки ChatHandler
// ─────────────────────────────────────────────────────────────────────────────

// newTestChatHandler собирает ChatHandler с реальными сервисами и репозиториями
// поверх pgxmock-пула и переданной заглушкой LLM-клиента.
//
// Репозитории transcription и summary используют PostgresDatabase напрямую,
// поэтому неэкспортированное поле pool устанавливается через unsafe.Pointer.
func newTestChatHandler(
	t *testing.T,
	mockDB pgxmock.PgxPoolIface,
	llm *mockLLMClient,
) *ChatHandler {
	t.Helper()

	// Устанавливаем pool в PostgresDatabase через unsafe, чтобы репозитории
	// transcription и summary работали через тот же pgxmock.
	db := &postgres.PostgresDatabase{}
	dbValue := reflect.ValueOf(db).Elem()
	poolField := dbValue.FieldByName("pool")
	if poolField.IsValid() && poolField.Kind() == reflect.Interface {
		reflect.NewAt(poolField.Type(), unsafe.Pointer(poolField.UnsafeAddr())).
			Elem().Set(reflect.ValueOf(mockDB))
	}
	repoUtils := postgres.NewPostgresRepositoryUtils(db)

	// Репозитории meeting и user используют FromPool-конструкторы (принимают QueryExecutor).
	meetingRepo := postgres.NewPostgresMeetingRepositoryFromPool(mockDB)
	userRepo := postgres.NewPostgresUserRepositoryFromPool(mockDB)
	usersService := service.NewUsersService(userRepo)
	taskRepo := postgres.NewPostgresTaskRepositoryFromPool(mockDB)
	tasksService := service.NewTasksService(taskRepo, repoUtils)

	serverConfig := &config.ServerConfig{}
	serverConfig.UploadFileDir = new(string)
	*serverConfig.UploadFileDir = t.TempDir()
	serverConfig.MaxUploadSize = new(int64)
	*serverConfig.MaxUploadSize = 10 << 20
	serverConfig.ProcessorLimit = new(int)
	*serverConfig.ProcessorLimit = 5

	meetingsService := service.NewMeetingsService(
		meetingRepo,
		repoUtils,
		usersService,
		tasksService,
		serverConfig,
	)

	// Репозитории summary и transcription используют PostgresDatabase (pool через unsafe).
	summaryRepo := postgres.NewPostgresSummaryRepository(db)
	summaryService := service.NewSummaryService(summaryRepo, repoUtils)

	transcriptionRepo := postgres.NewPostgresTranscriptionRepository(db)
	transcriptionService := service.NewTranscriptionService(transcriptionRepo, repoUtils)

	return NewChatHandler(llm, meetingsService, tasksService, transcriptionService, summaryService)
}

func transcriptionRows(rows ...[]any) *pgxmock.Rows {
	cols := pgxmock.NewRows([]string{
		"id", "meeting_id", "user_id", "text", "created_at", "rank",
	})
	for _, r := range rows {
		cols.AddRow(r...)
	}
	return cols
}

// ─────────────────────────────────────────────────────────────────────────────
// Типы when/want для тестов HandleChat
// ─────────────────────────────────────────────────────────────────────────────

type whenChat struct {
	requestBody string
	llmClient   *mockLLMClient
	setupMock   func(mock pgxmock.PgxPoolIface)
}

type wantChat struct {
	status      int
	bodyContain string // подстрока, которую ожидаем в теле ответа
}

// ─────────────────────────────────────────────────────────────────────────────
// Таблица тестов
// ─────────────────────────────────────────────────────────────────────────────

var handleChatTests = []struct {
	name string
	when whenChat
	want wantChat
}{
	// ── 1. Невалидный JSON в теле запроса ────────────────────────────────────
	{
		name: "Bad Request - invalid JSON body",
		when: whenChat{
			requestBody: `{"user_id":}`,
			llmClient: &mockLLMClient{
				extractIntentFn: func(_ context.Context, _ string) (*models.QueryIntent, error) {
					return nil, nil // не должен вызываться
				},
			},
			setupMock: func(_ pgxmock.PgxPoolIface) {
				// Никаких обращений к БД не ожидается
			},
		},
		want: wantChat{
			status:      http.StatusBadRequest,
			bodyContain: `"status":"error"`,
		},
	},
	// ── 2. Пустое тело запроса ───────────────────────────────────────────────
	{
		name: "Bad Request - empty body",
		when: whenChat{
			requestBody: ``,
			llmClient: &mockLLMClient{
				extractIntentFn: func(_ context.Context, _ string) (*models.QueryIntent, error) {
					return nil, nil
				},
			},
			setupMock: func(_ pgxmock.PgxPoolIface) {},
		},
		want: wantChat{
			status:      http.StatusBadRequest,
			bodyContain: `"status":"error"`,
		},
	},
	// ── 3. LLM-клиент (внешний REST API) вернул ошибку ───────────────────────
	{
		name: "Internal Server Error - LLM ExtractIntent fails",
		when: whenChat{
			requestBody: `{"user_id":"user1","question":"что обсуждали на встрече?"}`,
			llmClient: &mockLLMClient{
				extractIntentFn: func(_ context.Context, _ string) (*models.QueryIntent, error) {
					return nil, fmt.Errorf("gigachat unavailable")
				},
			},
			setupMock: func(_ pgxmock.PgxPoolIface) {
				// Никаких обращений к БД не ожидается
			},
		},
		want: wantChat{
			status:      http.StatusInternalServerError,
			bodyContain: `"status":"error"`,
		},
	},
	// ── 4. Вопрос не про встречу (IsMeetingQuery = false) ────────────────────
	{
		name: "Success - question is not about a meeting",
		when: whenChat{
			requestBody: `{"user_id":"user1","question":"какая погода сегодня?"}`,
			llmClient: &mockLLMClient{
				extractIntentFn: func(_ context.Context, _ string) (*models.QueryIntent, error) {
					return &models.QueryIntent{
						IsMeetingQuery: false,
						Keywords:       nil,
						Topic:          "",
					}, nil
				},
			},
			setupMock: func(_ pgxmock.PgxPoolIface) {
				// Никаких обращений к БД не ожидается
			},
		},
		want: wantChat{
			status:      http.StatusOK,
			bodyContain: "Похоже, вопрос не про встречу",
		},
	},
	// ── 5. Вопрос про встречу, транскрипции не найдены (пустой результат) ────
	{
		name: "Success - meeting query but no transcriptions found",
		when: whenChat{
			requestBody: `{"user_id":"user2","question":"найди встречу про бюджет"}`,
			llmClient: &mockLLMClient{
				extractIntentFn: func(_ context.Context, _ string) (*models.QueryIntent, error) {
					return &models.QueryIntent{
						IsMeetingQuery: true,
						Keywords:       []string{"бюджет"},
						Topic:          "финансы",
					}, nil
				},
			},
			setupMock: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery("SELECT id, meeting_id, user_id, text, created_at").
					WithArgs("user2", pgxmock.AnyArg(), pgxmock.AnyArg()).
					WillReturnRows(transcriptionRows())
			},
		},
		want: wantChat{
			status:      http.StatusOK,
			bodyContain: "Не удалось найти встречу по указанной теме",
		},
	},
	// ── 6. Транскрипция найдена, саммари отсутствует → ответ "не найдено" ────
	{
		name: "Success - transcription found but summary is nil",
		when: whenChat{
			requestBody: `{"user_id":"user3","question":"найди встречу про планирование"}`,
			llmClient: &mockLLMClient{
				extractIntentFn: func(_ context.Context, _ string) (*models.QueryIntent, error) {
					return &models.QueryIntent{
						IsMeetingQuery: true,
						Keywords:       []string{"планирование"},
						Topic:          "план",
					}, nil
				},
			},
			setupMock: func(mock pgxmock.PgxPoolIface) {
				meetingID := "meeting-id-3"
				now := time.Now()

				// FindByKeyWords: 2 строки → строка 2 сканируется (строка 1 поглощается внешним Next).
				mock.ExpectQuery("SELECT id, meeting_id, user_id, text, created_at").
					WithArgs("user3", pgxmock.AnyArg(), pgxmock.AnyArg()).
					WillReturnRows(transcriptionRows(
						[]any{"tr-id-3", meetingID, "user3", "текст транскрипции", now, 0.5},
					))

				// GetSummary → GetByParameter → pgx.ErrNoRows → "summary not found" → nil, nil
				mock.ExpectQuery("SELECT \\* FROM summary WHERE meeting_id").
					WithArgs(meetingID).
					WillReturnRows(pgxmock.NewRows([]string{
						"id", "meeting_id", "user_id", "text", "created_at", "search_vector",
					}))
			},
		},
		want: wantChat{
			status:      http.StatusOK,
			bodyContain: "Не удалось найти встречу по указанной теме",
		},
	},
	// ── 7. Транскрипция и саммари найдены, встреча получена → полный ответ ───
	//
	// Двойная итерация обходится двумя строками в FindByKeyWords.
	// GetSummary возвращает текст саммари, GetByID возвращает встречу.
	{
		name: "Success - full answer with meeting name, date and summary",
		when: whenChat{
			requestBody: `{"user_id":"user4","question":"найди встречу про релиз"}`,
			llmClient: &mockLLMClient{
				extractIntentFn: func(_ context.Context, _ string) (*models.QueryIntent, error) {
					return &models.QueryIntent{
						IsMeetingQuery: true,
						Keywords:       []string{"релиз"},
						Topic:          "выпуск",
					}, nil
				},
			},
			setupMock: func(mock pgxmock.PgxPoolIface) {
				meetingID := "meeting-id-4"
				now := time.Now()
				meetingName := "Release Planning"
				summaryText := "Обсудили план релиза на следующий квартал"

				// FindByKeyWords: 2 строки → строка 2 сканируется.
				mock.ExpectQuery("SELECT id, meeting_id, user_id, text, created_at").
					WithArgs("user4", pgxmock.AnyArg(), pgxmock.AnyArg()).
					WillReturnRows(transcriptionRows(
						[]any{"tr-id-4", meetingID, "user4", "текст про релиз", now, 0.9},
					))

				// GetSummary → GetByParameter → возвращает саммари
				mock.ExpectQuery("SELECT \\* FROM summary WHERE meeting_id").
					WithArgs(meetingID).
					WillReturnRows(pgxmock.NewRows([]string{
						"id", "meeting_id", "user_id", "text", "created_at", "search_vector",
					}).AddRow("sum-id-4", meetingID, "user4", summaryText, now, nil))

				// GetByID для встречи → SELECT * FROM meetings WHERE id = $1
				mock.ExpectQuery("SELECT \\* FROM meetings WHERE id").
					WithArgs(meetingID).
					WillReturnRows(pgxmock.NewRows([]string{
						"id", "user_id", "meeting_name", "file_path",
						"original_file_name", "created_at", "updated_at",
					}).AddRow(meetingID, "user4", &meetingName, nil, nil, now, now))
			},
		},
		want: wantChat{
			status:      http.StatusOK,
			bodyContain: "Release Planning",
		},
	},
	// ── 8. Ошибка БД при получении саммари → 500 ─────────────────────────────
	//
	// FindByKeyWords возвращает транскрипцию (двойная итерация обходится 2 строками).
	// GetSummary возвращает неожиданную ошибку БД (не "summary not found") → 500.
	{
		name: "Internal Server Error - DB error on GetSummary",
		when: whenChat{
			requestBody: `{"user_id":"user6","question":"найди встречу про архитектуру"}`,
			llmClient: &mockLLMClient{
				extractIntentFn: func(_ context.Context, _ string) (*models.QueryIntent, error) {
					return &models.QueryIntent{
						IsMeetingQuery: true,
						Keywords:       []string{"архитектура"},
						Topic:          "система",
					}, nil
				},
			},
			setupMock: func(mock pgxmock.PgxPoolIface) {
				meetingID := "meeting-id-6"
				now := time.Now()

				// FindByKeyWords: 2 строки → строка 2 сканируется.
				mock.ExpectQuery("SELECT id, meeting_id, user_id, text, created_at").
					WithArgs("user6", pgxmock.AnyArg(), pgxmock.AnyArg()).
					WillReturnRows(transcriptionRows(
						[]any{"tr-id-6", meetingID, "user6", "текст про архитектуру", now, 0.7},
					))

				// GetSummary → неожиданная ошибка БД (не "summary not found") → 500
				mock.ExpectQuery("SELECT \\* FROM summary WHERE meeting_id").
					WithArgs(meetingID).
					WillReturnError(fmt.Errorf("unexpected db error"))
			},
		},
		want: wantChat{
			status:      http.StatusInternalServerError,
			bodyContain: `"status":"error"`,
		},
	},
	// ── 9. Ошибка БД при получении встречи по ID → 500 ───────────────────────
	//
	// FindByKeyWords и GetSummary успешны, но GetByID для встречи возвращает ошибку.
	{
		name: "Internal Server Error - DB error on GetMeetingByID",
		when: whenChat{
			requestBody: `{"user_id":"user7","question":"найди встречу про тестирование"}`,
			llmClient: &mockLLMClient{
				extractIntentFn: func(_ context.Context, _ string) (*models.QueryIntent, error) {
					return &models.QueryIntent{
						IsMeetingQuery: true,
						Keywords:       []string{"тестирование"},
						Topic:          "QA",
					}, nil
				},
			},
			setupMock: func(mock pgxmock.PgxPoolIface) {
				meetingID := "meeting-id-7"
				now := time.Now()
				summaryText := "Краткое содержание встречи по тестированию"

				// FindByKeyWords: 2 строки → строка 2 сканируется.
				mock.ExpectQuery("SELECT id, meeting_id, user_id, text, created_at").
					WithArgs("user7", pgxmock.AnyArg(), pgxmock.AnyArg()).
					WillReturnRows(transcriptionRows(
						[]any{"tr-id-7", meetingID, "user7", "текст про тестирование", now, 0.8},
					))

				// GetSummary → возвращает саммари
				mock.ExpectQuery("SELECT \\* FROM summary WHERE meeting_id").
					WithArgs(meetingID).
					WillReturnRows(pgxmock.NewRows([]string{
						"id", "meeting_id", "user_id", "text", "created_at", "search_vector",
					}).AddRow("sum-id-7", meetingID, "user7", summaryText, now, nil))

				// GetByID для встречи → ошибка БД
				mock.ExpectQuery("SELECT \\* FROM meetings WHERE id").
					WithArgs(meetingID).
					WillReturnError(fmt.Errorf("db timeout"))
			},
		},
		want: wantChat{
			status:      http.StatusInternalServerError,
			bodyContain: `"status":"error"`,
		},
	},
}

// ─────────────────────────────────────────────────────────────────────────────
// Тест
// ─────────────────────────────────────────────────────────────────────────────

func TestChatHandler_HandleChat(t *testing.T) {
	for _, tt := range handleChatTests {
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

			handler := newTestChatHandler(t, mockDB, tt.when.llmClient)

			// when
			req := httptest.NewRequest(
				http.MethodPost,
				"/api/chat",
				strings.NewReader(tt.when.requestBody),
			)
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler.HandleChat(rr, req)

			// проверка ожидаемого результата
			if rr.Code != tt.want.status {
				t.Errorf("expected status %d, got %d; body: %s",
					tt.want.status, rr.Code, rr.Body.String())
			}

			if tt.want.bodyContain != "" {
				responseBody := strings.TrimSpace(rr.Body.String())
				if !strings.Contains(responseBody, tt.want.bodyContain) {
					t.Errorf("expected body to contain %q, got %q",
						tt.want.bodyContain, responseBody)
				}
			}

			if err := mockDB.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled mock expectations: %v", err)
			}
		})
	}
}
