package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pashagolub/pgxmock/v5"
	"github.com/scouser-122/meeting-analyzer/internal/repository/postgres"
	"github.com/scouser-122/meeting-analyzer/internal/service"
)

// ─────────────────────────────────────────────────────────────────────────────
// Тесты для HandleStart
// ─────────────────────────────────────────────────────────────────────────────

var usersHandlerTests = []struct {
	name           string
	userID         string
	requestBody    string
	setupMock      func(mock pgxmock.PgxPoolIface)
	expectedStatus int
	expectedBody   string
}{
	{
		name:        "Success - user created successfully",
		userID:      "user123",
		requestBody: `{"id":"user123"}`,
		setupMock: func(mock pgxmock.PgxPoolIface) {
			// Expect INSERT
			mock.ExpectExec(
				"INSERT INTO users",
			).WithArgs("user123", pgxmock.AnyArg()).
				WillReturnResult(pgxmock.NewResult("INSERT", 1))
			// Expect SELECT (GetByID called after Create to return the created user)
			mock.ExpectQuery(
				"SELECT \\* FROM users",
			).WithArgs("user123").
				WillReturnRows(
					pgxmock.NewRows([]string{"id", "created_at"}).
						AddRow("user123", time.Now()),
				)
		},
		expectedStatus: http.StatusOK,
		expectedBody:   `{"status":"ok","message":"user successfully registered"}`,
	},
	{
		name:        "Bad Request - empty user ID in service validation",
		userID:      "",
		requestBody: `{"id":""}`,
		setupMock: func(mock pgxmock.PgxPoolIface) {
			// No DB interaction expected due to validation
		},
		expectedStatus: http.StatusBadRequest,
		expectedBody:   `{"status":"error","message":"user id absent"}`,
	},
	{
		name:        "Bad Request - missing id field in JSON (empty after unmarshal)",
		userID:      "",
		requestBody: `{"name":"test"}`,
		setupMock: func(mock pgxmock.PgxPoolIface) {
			// No DB interaction expected - unmarshal creates empty user.ID
		},
		expectedStatus: http.StatusBadRequest,
		expectedBody:   `{"status":"error","message":"user id absent"}`,
	},
	{
		name:        "Conflict - user ID already exists",
		userID:      "existinguser",
		requestBody: `{"id":"existinguser"}`,
		setupMock: func(mock pgxmock.PgxPoolIface) {
			// Simulate unique violation error
			pgErr := &pgconn.PgError{
				Code:           "23505",
				ConstraintName: "users_pkey",
				Message:        "duplicate key value violates unique constraint \"users_pkey\"",
			}
			mock.ExpectExec(
				"INSERT INTO users",
			).WithArgs("existinguser", pgxmock.AnyArg()).
				WillReturnError(pgErr)
		},
		expectedStatus: http.StatusConflict,
		expectedBody:   `{"status":"error","message":"user id busy"}`,
	},
	{
		name:        "Internal Server Error - general database error",
		userID:      "user456",
		requestBody: `{"id":"user456"}`,
		setupMock: func(mock pgxmock.PgxPoolIface) {
			// Simulate general database error
			mock.ExpectExec(
				"INSERT INTO users",
			).WithArgs("user456", pgxmock.AnyArg()).
				WillReturnError(fmt.Errorf("database connection lost"))
		},
		expectedStatus: http.StatusInternalServerError,
		expectedBody:   `{"status":"error","message":"unexpected error happen"}`,
	},
}

func TestUsersHandler_HandleStart(t *testing.T) {
	for _, tt := range usersHandlerTests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock database
			mockDB, err := pgxmock.NewPool()
			if err != nil {
				t.Fatalf("failed to create mock pool: %v", err)
			}
			defer mockDB.Close()

			// Setup mock expectations
			if tt.setupMock != nil {
				tt.setupMock(mockDB)
			}

			// Create repository from mock
			userRepo := postgres.NewPostgresUserRepositoryFromPool(mockDB)
			usersService := service.NewUsersService(userRepo)

			// Create handler
			handler := NewUsersHandler(usersService)

			// Create request
			req := httptest.NewRequest(
				http.MethodPost,
				"/api/users/start",
				strings.NewReader(tt.requestBody),
			)
			req.Header.Set("Content-Type", "application/json")

			// Create response recorder
			rr := httptest.NewRecorder()

			// Call handler
			handler.HandleStart(rr, req)

			// Verify status code
			if rr.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rr.Code)
			}

			// Verify response body
			responseBody := strings.TrimSpace(rr.Body.String())
			if responseBody != tt.expectedBody {
				t.Errorf("expected body %q, got %q", tt.expectedBody, responseBody)
			}

			// Verify all expectations were met
			if err := mockDB.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

// Additional test for invalid JSON
func TestUsersHandler_HandleStart_InvalidJSON(t *testing.T) {
	t.Run("Invalid JSON format", func(t *testing.T) {
		// Create mock database
		mockDB, err := pgxmock.NewPool()
		if err != nil {
			t.Fatalf("failed to create mock pool: %v", err)
		}
		defer mockDB.Close()

		// No DB expectations - should fail before hitting DB
		usersService := service.NewUsersService(
			postgres.NewPostgresUserRepositoryFromPool(mockDB),
		)
		handler := NewUsersHandler(usersService)

		req := httptest.NewRequest(
			http.MethodPost,
			"/api/users/start",
			strings.NewReader(`{"id":}`), // Invalid JSON
		)
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		handler.HandleStart(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
	})
}

// Additional test for empty request body
func TestUsersHandler_HandleStart_EmptyBody(t *testing.T) {
	t.Run("Empty request body", func(t *testing.T) {
		mockDB, err := pgxmock.NewPool()
		if err != nil {
			t.Fatalf("failed to create mock pool: %v", err)
		}
		defer mockDB.Close()

		usersService := service.NewUsersService(
			postgres.NewPostgresUserRepositoryFromPool(mockDB),
		)
		handler := NewUsersHandler(usersService)

		req := httptest.NewRequest(
			http.MethodPost,
			"/api/users/start",
			strings.NewReader(""), // Empty body
		)
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		handler.HandleStart(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
		}
	})
}
