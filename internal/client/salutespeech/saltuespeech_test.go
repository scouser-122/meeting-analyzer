package salutespeech

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/scouser-122/meeting-analyzer/internal/config"
	"github.com/scouser-122/meeting-analyzer/internal/domain/model"
	"github.com/scouser-122/meeting-analyzer/internal/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testContext(t *testing.T) context.Context {
	t.Helper()
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	return context.WithValue(context.Background(), logger.LoggerKey, log)
}

type tokenRequest struct {
	Bearer string
}

type uploadRequest struct {
	RequestID   string
	Bearer      string
	ContentType string
	Body        []byte
}

type recognizeRequest struct {
	RequestID string
	Bearer    string
	Body      SaluteSpeechRecognizeRequest
}

type statusRequest struct {
	RequestID string
	Bearer    string
	TaskID    string
}

type downloadRequest struct {
	RequestID string
	Bearer    string
	FileID    string
}

type testServer struct {
	*httptest.Server

	tokenRequests     []tokenRequest
	uploadRequests    []uploadRequest
	recognizeRequests []recognizeRequest
	statusRequests    []statusRequest
	downloadRequests  []downloadRequest

	tokenResponse       any
	uploadResponse      any
	recognizeResponse   any
	statusResponse      model.TaskStatus
	downloadResponse    any
	tokenStatusCode     int
	uploadStatusCode    int
	recognizeStatusCode int
	statusStatusCode    int
	downloadStatusCode  int
	statusSequence      []model.TaskStatus
	statusSequenceIndex int
}

func newTestServer(t *testing.T) *testServer {
	t.Helper()

	ts := &testServer{
		tokenResponse: SaluteAuthToken{
			Token:     "test-access-token",
			ExpiresAt: time.Now().Add(time.Hour).Unix(),
		},
		uploadResponse: SaluteSpeechUploadResponse{
			Status: http.StatusOK,
			Result: SaluteSpeechUploadResult{
				RequestFileId: "uploaded-file-id",
			},
		},
		recognizeResponse: SaluteSpeechRecognizeResponse{
			Status: http.StatusOK,
			Result: SaluteSpeechRecognizeResult{
				ID:     "recognize-task-id",
				Status: "RUNNING",
			},
		},
		statusResponse:      "DONE",
		downloadResponse:    SaluteSpeechRecognizedText{Text: "recognized text"},
		tokenStatusCode:     http.StatusOK,
		uploadStatusCode:    http.StatusOK,
		recognizeStatusCode: http.StatusOK,
		statusStatusCode:    http.StatusOK,
		downloadStatusCode:  http.StatusOK,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/oauth", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		ts.tokenRequests = append(ts.tokenRequests, tokenRequest{
			Bearer: r.Header.Get("Authorization"),
		})

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(ts.tokenStatusCode)
		if ts.tokenResponse != nil {
			require.NoError(t, json.NewEncoder(w).Encode(ts.tokenResponse))
		}
	})
	mux.HandleFunc("/rest/v1/data:upload", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		ts.uploadRequests = append(ts.uploadRequests, uploadRequest{
			RequestID:   r.Header.Get("X-Request-ID"),
			Bearer:      r.Header.Get("Authorization"),
			ContentType: r.Header.Get("Content-Type"),
			Body:        body,
		})

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(ts.uploadStatusCode)
		if ts.uploadResponse != nil {
			require.NoError(t, json.NewEncoder(w).Encode(ts.uploadResponse))
		}
	})
	mux.HandleFunc("/rest/v1/speech:async_recognize", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var reqBody SaluteSpeechRecognizeRequest
		_ = json.NewDecoder(r.Body).Decode(&reqBody)

		ts.recognizeRequests = append(ts.recognizeRequests, recognizeRequest{
			RequestID: r.Header.Get("X-Request-ID"),
			Bearer:    r.Header.Get("Authorization"),
			Body:      reqBody,
		})

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(ts.recognizeStatusCode)
		if ts.recognizeResponse != nil {
			require.NoError(t, json.NewEncoder(w).Encode(ts.recognizeResponse))
		}
	})
	mux.HandleFunc("/rest/v1/task:get", func(w http.ResponseWriter, r *http.Request) {
		taskID := r.URL.Query().Get("id")

		ts.statusRequests = append(ts.statusRequests, statusRequest{
			RequestID: r.Header.Get("X-Request-ID"),
			Bearer:    r.Header.Get("Authorization"),
			TaskID:    taskID,
		})

		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(ts.statusStatusCode)

		status := ts.statusResponse
		if len(ts.statusSequence) > 0 {
			if ts.statusSequenceIndex < len(ts.statusSequence) {
				status = ts.statusSequence[ts.statusSequenceIndex]
				ts.statusSequenceIndex++
			} else {
				status = ts.statusSequence[len(ts.statusSequence)-1]
			}
		}
		_, _ = w.Write([]byte(status))
	})
	mux.HandleFunc("/rest/v1/data:download", func(w http.ResponseWriter, r *http.Request) {
		fileID := r.URL.Query().Get("id")

		ts.downloadRequests = append(ts.downloadRequests, downloadRequest{
			RequestID: r.Header.Get("X-Request-ID"),
			Bearer:    r.Header.Get("Authorization"),
			FileID:    fileID,
		})

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(ts.downloadStatusCode)
		if ts.downloadResponse != nil {
			require.NoError(t, json.NewEncoder(w).Encode(ts.downloadResponse))
		}
	})

	ts.Server = httptest.NewServer(mux)
	return ts
}

func (ts *testServer) client(t *testing.T) *SaluteSpeechClient {
	return &SaluteSpeechClient{
		config: &config.SaluteSpeechConfig{
			ServerAddress:            ts.Server.URL,
			StatusPollingInterval:    0,
			StatusPollingMaxAttempts: 3,
			RecognizedFileDir:        t.TempDir(),
		},
	}
}

func (ts *testServer) clientWithTempDir(tempDir string) *SaluteSpeechClient {
	return &SaluteSpeechClient{
		config: &config.SaluteSpeechConfig{
			ServerAddress:            ts.Server.URL,
			StatusPollingInterval:    0,
			StatusPollingMaxAttempts: 3,
			RecognizedFileDir:        tempDir,
		},
	}
}

func TestSaluteSpeechClient_getToken(t *testing.T) {
	type when struct {
		existingToken   SaluteAuthToken
		tokenResponse   any
		tokenStatusCode int
	}

	type want struct {
		token         string
		wantErr       bool
		errContains   string
		tokenRequests int
	}

	var testsList = []struct {
		name string
		when when
		want want
	}{
		{
			name: "valid cached token returned without request",
			when: when{
				existingToken: SaluteAuthToken{
					Token:     "cached-token",
					ExpiresAt: time.Now().Add(time.Hour).Unix(),
				},
			},
			want: want{
				token:         "cached-token",
				tokenRequests: 0,
			},
		},
		{
			name: "expired cached token triggers new token request",
			when: when{
				existingToken: SaluteAuthToken{
					Token:     "expired-token",
					ExpiresAt: time.Now().Add(-time.Hour).Unix(),
				},
			},
			want: want{
				token:         "test-access-token",
				tokenRequests: 1,
			},
		},
		{
			name: "empty cached token triggers new token request",
			when: when{
				existingToken: SaluteAuthToken{},
			},
			want: want{
				token:         "test-access-token",
				tokenRequests: 1,
			},
		},
		{
			name: "oauth returns non-200 status",
			when: when{
				existingToken:   SaluteAuthToken{},
				tokenStatusCode: http.StatusUnauthorized,
				tokenResponse:   map[string]string{"error": "unauthorized"},
			},
			want: want{
				wantErr:       true,
				errContains:   "failed to get token",
				tokenRequests: 1,
			},
		},
	}

	for _, test := range testsList {
		t.Run(test.name, func(t *testing.T) {
			// setup
			server := newTestServer(t)
			defer server.Close()

			if test.when.tokenResponse != nil {
				server.tokenResponse = test.when.tokenResponse
			}
			if test.when.tokenStatusCode != 0 {
				server.tokenStatusCode = test.when.tokenStatusCode
			}

			client := server.client(t)
			client.token = test.when.existingToken

			// when
			token, err := client.getToken(testContext(t))

			// then
			if test.want.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.want.errContains)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.want.token, token)
			assert.Len(t, server.tokenRequests, test.want.tokenRequests)

			if test.want.tokenRequests > 0 {
				assert.Empty(t, server.tokenRequests[0].Bearer)
			}
		})
	}
}

func TestSaluteSpeechClient_sendFile(t *testing.T) {
	type when struct {
		existingToken    SaluteAuthToken
		filePath         string
		tokenStatusCode  int
		uploadStatusCode int
		uploadResponse   any
	}

	type want struct {
		fileID         string
		wantErr        bool
		errContains    string
		uploadRequests int
	}

	tempDir := t.TempDir()
	validFilePath := filepath.Join(tempDir, "test.mp3")
	require.NoError(t, os.WriteFile(validFilePath, []byte("audio content"), 0o644))

	var testsList = []struct {
		name string
		when when
		want want
	}{
		{
			name: "successful file upload",
			when: when{
				existingToken: SaluteAuthToken{
					Token:     "cached-token",
					ExpiresAt: time.Now().Add(time.Hour).Unix(),
				},
				filePath: validFilePath,
			},
			want: want{
				fileID:         "uploaded-file-id",
				uploadRequests: 1,
			},
		},
		{
			name: "file open error",
			when: when{
				existingToken: SaluteAuthToken{
					Token:     "cached-token",
					ExpiresAt: time.Now().Add(time.Hour).Unix(),
				},
				filePath: filepath.Join(tempDir, "nonexistent.mp3"),
			},
			want: want{
				wantErr:        true,
				errContains:    "failed to open audio file",
				uploadRequests: 0,
			},
		},
		{
			name: "token request returns non-200 status",
			when: when{
				existingToken:   SaluteAuthToken{},
				filePath:        validFilePath,
				tokenStatusCode: http.StatusUnauthorized,
			},
			want: want{
				wantErr:        true,
				errContains:    "failed to get token",
				uploadRequests: 0,
			},
		},
		{
			name: "upload returns non-200 status",
			when: when{
				existingToken: SaluteAuthToken{
					Token:     "cached-token",
					ExpiresAt: time.Now().Add(time.Hour).Unix(),
				},
				filePath:         validFilePath,
				uploadStatusCode: http.StatusInternalServerError,
				uploadResponse:   map[string]string{"error": "internal"},
			},
			want: want{
				wantErr:        true,
				errContains:    "failed to upload file",
				uploadRequests: 1,
			},
		},
	}

	for _, test := range testsList {
		t.Run(test.name, func(t *testing.T) {
			// setup
			server := newTestServer(t)
			defer server.Close()

			if test.when.uploadResponse != nil {
				server.uploadResponse = test.when.uploadResponse
			}
			if test.when.uploadStatusCode != 0 {
				server.uploadStatusCode = test.when.uploadStatusCode
			}
			if test.when.tokenStatusCode != 0 {
				server.tokenStatusCode = test.when.tokenStatusCode
			}

			client := server.client(t)
			client.token = test.when.existingToken

			// when
			fileID, err := client.sendFile(testContext(t), test.when.filePath)

			// then
			if test.want.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.want.errContains)
				assert.Len(t, server.uploadRequests, test.want.uploadRequests)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.want.fileID, fileID)
			assert.Len(t, server.uploadRequests, test.want.uploadRequests)

			if test.want.uploadRequests > 0 {
				assert.Equal(t, "Bearer cached-token", server.uploadRequests[0].Bearer)
				assert.NotEmpty(t, server.uploadRequests[0].RequestID)
				assert.Equal(t, "audio/mpeg", server.uploadRequests[0].ContentType)
				assert.Equal(t, []byte("audio content"), server.uploadRequests[0].Body)
			}
		})
	}
}

func TestSaluteSpeechClient_createRecognizeTask(t *testing.T) {
	type when struct {
		existingToken       SaluteAuthToken
		tokenStatusCode     int
		recognizeStatusCode int
		recognizeResponse   any
	}

	type want struct {
		taskID            string
		wantErr           bool
		errContains       string
		recognizeRequests int
	}

	var testsList = []struct {
		name string
		when when
		want want
	}{
		{
			name: "successful recognize task creation",
			when: when{
				existingToken: SaluteAuthToken{
					Token:     "cached-token",
					ExpiresAt: time.Now().Add(time.Hour).Unix(),
				},
			},
			want: want{
				taskID:            "recognize-task-id",
				recognizeRequests: 1,
			},
		},
		{
			name: "token request returns non-200 status",
			when: when{
				existingToken:   SaluteAuthToken{},
				tokenStatusCode: http.StatusUnauthorized,
			},
			want: want{
				wantErr:           true,
				errContains:       "failed to get token",
				recognizeRequests: 0,
			},
		},
		{
			name: "recognize returns non-200 status",
			when: when{
				existingToken: SaluteAuthToken{
					Token:     "cached-token",
					ExpiresAt: time.Now().Add(time.Hour).Unix(),
				},
				recognizeStatusCode: http.StatusBadRequest,
				recognizeResponse:   map[string]string{"error": "bad request"},
			},
			want: want{
				wantErr:           true,
				errContains:       "failed to start recognize",
				recognizeRequests: 1,
			},
		},
		{
			name: "recognize response status is ERROR",
			when: when{
				existingToken: SaluteAuthToken{
					Token:     "cached-token",
					ExpiresAt: time.Now().Add(time.Hour).Unix(),
				},
				recognizeResponse: SaluteSpeechRecognizeResponse{
					Status: http.StatusOK,
					Result: SaluteSpeechRecognizeResult{
						ID:     "error-task-id",
						Status: "ERROR",
					},
				},
			},
			want: want{
				wantErr:           true,
				errContains:       "failed to start recognize",
				recognizeRequests: 1,
			},
		},
	}

	for _, test := range testsList {
		t.Run(test.name, func(t *testing.T) {
			// setup
			server := newTestServer(t)
			defer server.Close()

			if test.when.recognizeResponse != nil {
				server.recognizeResponse = test.when.recognizeResponse
			}
			if test.when.recognizeStatusCode != 0 {
				server.recognizeStatusCode = test.when.recognizeStatusCode
			}
			if test.when.tokenStatusCode != 0 {
				server.tokenStatusCode = test.when.tokenStatusCode
			}

			client := server.client(t)
			client.token = test.when.existingToken

			// when
			taskID, err := client.createRecognizeTask(testContext(t), "uploaded-file-id")

			// then
			if test.want.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.want.errContains)
				assert.Len(t, server.recognizeRequests, test.want.recognizeRequests)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.want.taskID, taskID)
			assert.Len(t, server.recognizeRequests, test.want.recognizeRequests)

			if test.want.recognizeRequests > 0 {
				assert.Equal(t, "Bearer cached-token", server.recognizeRequests[0].Bearer)
				assert.NotEmpty(t, server.recognizeRequests[0].RequestID)
				assert.Equal(t, "MP3", server.recognizeRequests[0].Body.Options.AudioEncoding)
			}
		})
	}
}

func TestSaluteSpeechClient_getRecognizeStatus(t *testing.T) {
	type when struct {
		existingToken    SaluteAuthToken
		tokenStatusCode  int
		statusStatusCode int
		statusResponse   model.TaskStatus
	}

	type want struct {
		status         model.TaskStatus
		wantErr        bool
		errContains    string
		statusRequests int
	}

	var testsList = []struct {
		name string
		when when
		want want
	}{
		{
			name: "successful status retrieval",
			when: when{
				existingToken: SaluteAuthToken{
					Token:     "cached-token",
					ExpiresAt: time.Now().Add(time.Hour).Unix(),
				},
				statusResponse: "DONE",
			},
			want: want{
				status:         "DONE",
				statusRequests: 1,
			},
		},
		{
			name: "token request returns non-200 status",
			when: when{
				existingToken:   SaluteAuthToken{},
				tokenStatusCode: http.StatusUnauthorized,
			},
			want: want{
				wantErr:        true,
				errContains:    "failed to get token",
				statusRequests: 0,
			},
		},
		{
			name: "status returns non-200 status",
			when: when{
				existingToken: SaluteAuthToken{
					Token:     "cached-token",
					ExpiresAt: time.Now().Add(time.Hour).Unix(),
				},
				statusStatusCode: http.StatusNotFound,
				statusResponse:   "NOT_FOUND",
			},
			want: want{
				wantErr:        true,
				errContains:    "failed to get recognize status",
				statusRequests: 1,
			},
		},
	}

	for _, test := range testsList {
		t.Run(test.name, func(t *testing.T) {
			// setup
			server := newTestServer(t)
			defer server.Close()

			if test.when.statusResponse != "" {
				server.statusResponse = test.when.statusResponse
			}
			if test.when.statusStatusCode != 0 {
				server.statusStatusCode = test.when.statusStatusCode
			}
			if test.when.tokenStatusCode != 0 {
				server.tokenStatusCode = test.when.tokenStatusCode
			}

			client := server.client(t)
			client.token = test.when.existingToken

			// when
			status, err := client.getRecognizeStatus(testContext(t), "recognize-task-id")

			// then
			if test.want.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.want.errContains)
				assert.Len(t, server.statusRequests, test.want.statusRequests)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.want.status, status)
			assert.Len(t, server.statusRequests, test.want.statusRequests)

			if test.want.statusRequests > 0 {
				assert.Equal(t, "Bearer cached-token", server.statusRequests[0].Bearer)
				assert.NotEmpty(t, server.statusRequests[0].RequestID)
				assert.Equal(t, "recognize-task-id", server.statusRequests[0].TaskID)
			}
		})
	}
}

func TestSaluteSpeechClient_getResultFromFile(t *testing.T) {
	type when struct {
		existingToken      SaluteAuthToken
		tokenStatusCode    int
		downloadStatusCode int
		downloadResponse   any
	}

	type want struct {
		text             string
		wantErr          bool
		errContains      string
		downloadRequests int
	}

	var testsList = []struct {
		name string
		when when
		want want
	}{
		{
			name: "successful result download and parse",
			when: when{
				existingToken: SaluteAuthToken{
					Token:     "cached-token",
					ExpiresAt: time.Now().Add(time.Hour).Unix(),
				},
				downloadResponse: SaluteSpeechRecognizedText{Text: "recognized text"},
			},
			want: want{
				text:             "recognized text",
				downloadRequests: 1,
			},
		},
		{
			name: "token request returns non-200 status",
			when: when{
				existingToken:   SaluteAuthToken{},
				tokenStatusCode: http.StatusUnauthorized,
			},
			want: want{
				wantErr:          true,
				errContains:      "failed to get token",
				downloadRequests: 0,
			},
		},
		{
			name: "download returns non-200 status",
			when: when{
				existingToken: SaluteAuthToken{
					Token:     "cached-token",
					ExpiresAt: time.Now().Add(time.Hour).Unix(),
				},
				downloadStatusCode: http.StatusBadRequest,
			},
			want: want{
				wantErr:          true,
				errContains:      "failed to start recognize",
				downloadRequests: 1,
			},
		},
		{
			name: "download response is invalid json",
			when: when{
				existingToken: SaluteAuthToken{
					Token:     "cached-token",
					ExpiresAt: time.Now().Add(time.Hour).Unix(),
				},
				downloadResponse: "not a json",
			},
			want: want{
				wantErr:          true,
				errContains:      "failed unmarshalling JSON",
				downloadRequests: 1,
			},
		},
	}

	for _, test := range testsList {
		t.Run(test.name, func(t *testing.T) {
			// setup
			server := newTestServer(t)
			defer server.Close()

			if test.when.downloadResponse != nil {
				server.downloadResponse = test.when.downloadResponse
			}
			if test.when.downloadStatusCode != 0 {
				server.downloadStatusCode = test.when.downloadStatusCode
			}
			if test.when.tokenStatusCode != 0 {
				server.tokenStatusCode = test.when.tokenStatusCode
			}

			client := server.client(t)
			client.token = test.when.existingToken

			// when
			text, err := client.getResultFromFile(testContext(t), "uploaded-file-id")

			// then
			if test.want.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.want.errContains)
				assert.Len(t, server.downloadRequests, test.want.downloadRequests)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.want.text, text)
			assert.Len(t, server.downloadRequests, test.want.downloadRequests)

			if test.want.downloadRequests > 0 {
				assert.Equal(t, "Bearer cached-token", server.downloadRequests[0].Bearer)
				assert.NotEmpty(t, server.downloadRequests[0].RequestID)
				assert.Equal(t, "uploaded-file-id", server.downloadRequests[0].FileID)
			}
		})
	}
}

func TestSaluteSpeechClient_TranscribeAudio(t *testing.T) {
	type when struct {
		existingToken       SaluteAuthToken
		tokenStatusCode     int
		statusSequence      []model.TaskStatus
		statusStatusCode    int
		downloadStatusCode  int
		downloadResponse    any
		uploadStatusCode    int
		recognizeStatusCode int
	}

	type want struct {
		text              string
		wantErr           bool
		errContains       string
		uploadRequests    int
		recognizeRequests int
		statusRequests    int
		downloadRequests  int
	}

	tempDir := t.TempDir()
	validFilePath := filepath.Join(tempDir, "test.mp3")
	require.NoError(t, os.WriteFile(validFilePath, []byte("audio content"), 0o644))

	var testsList = []struct {
		name string
		when when
		want want
	}{
		{
			name: "successful transcription",
			when: when{
				existingToken: SaluteAuthToken{
					Token:     "cached-token",
					ExpiresAt: time.Now().Add(time.Hour).Unix(),
				},
				statusSequence:   []model.TaskStatus{"RUNNING", "DONE"},
				downloadResponse: SaluteSpeechRecognizedText{Text: "full transcription text"},
			},
			want: want{
				text:              "full transcription text",
				uploadRequests:    1,
				recognizeRequests: 1,
				statusRequests:    2,
				downloadRequests:  1,
			},
		},
		{
			name: "recognize status ERROR",
			when: when{
				existingToken: SaluteAuthToken{
					Token:     "cached-token",
					ExpiresAt: time.Now().Add(time.Hour).Unix(),
				},
				statusSequence: []model.TaskStatus{"ERROR"},
			},
			want: want{
				wantErr:           true,
				errContains:       "recognize status error",
				uploadRequests:    1,
				recognizeRequests: 1,
				statusRequests:    1,
				downloadRequests:  0,
			},
		},
		{
			name: "recognize status CANCELED",
			when: when{
				existingToken: SaluteAuthToken{
					Token:     "cached-token",
					ExpiresAt: time.Now().Add(time.Hour).Unix(),
				},
				statusSequence: []model.TaskStatus{"CANCELED"},
			},
			want: want{
				wantErr:           true,
				errContains:       "recognize status cancelled",
				uploadRequests:    1,
				recognizeRequests: 1,
				statusRequests:    1,
				downloadRequests:  0,
			},
		},
		{
			name: "max polling attempts exceeded without DONE then downloads result",
			when: when{
				existingToken: SaluteAuthToken{
					Token:     "cached-token",
					ExpiresAt: time.Now().Add(time.Hour).Unix(),
				},
				statusSequence: []model.TaskStatus{"RUNNING", "RUNNING", "RUNNING"},
			},
			want: want{
				text:              "recognized text",
				uploadRequests:    1,
				recognizeRequests: 1,
				statusRequests:    3,
				downloadRequests:  1,
			},
		},
		{
			name: "upload fails",
			when: when{
				existingToken: SaluteAuthToken{
					Token:     "cached-token",
					ExpiresAt: time.Now().Add(time.Hour).Unix(),
				},
				uploadStatusCode: http.StatusBadRequest,
			},
			want: want{
				wantErr:           true,
				errContains:       "failed to upload file",
				uploadRequests:    1,
				recognizeRequests: 0,
				statusRequests:    0,
				downloadRequests:  0,
			},
		},
		{
			name: "recognize task creation fails",
			when: when{
				existingToken: SaluteAuthToken{
					Token:     "cached-token",
					ExpiresAt: time.Now().Add(time.Hour).Unix(),
				},
				recognizeStatusCode: http.StatusBadRequest,
			},
			want: want{
				wantErr:           true,
				errContains:       "failed to start recognize",
				uploadRequests:    1,
				recognizeRequests: 1,
				statusRequests:    0,
				downloadRequests:  0,
			},
		},
		{
			name: "download fails",
			when: when{
				existingToken: SaluteAuthToken{
					Token:     "cached-token",
					ExpiresAt: time.Now().Add(time.Hour).Unix(),
				},
				statusSequence:     []model.TaskStatus{"DONE"},
				downloadStatusCode: http.StatusBadRequest,
			},
			want: want{
				wantErr:           true,
				errContains:       "failed to start recognize",
				uploadRequests:    1,
				recognizeRequests: 1,
				statusRequests:    1,
				downloadRequests:  1,
			},
		},
	}

	for _, test := range testsList {
		t.Run(test.name, func(t *testing.T) {
			// setup
			server := newTestServer(t)
			defer server.Close()

			if test.when.statusSequence != nil {
				server.statusSequence = test.when.statusSequence
			}
			if test.when.statusStatusCode != 0 {
				server.statusStatusCode = test.when.statusStatusCode
			}
			if test.when.downloadResponse != nil {
				server.downloadResponse = test.when.downloadResponse
			}
			if test.when.downloadStatusCode != 0 {
				server.downloadStatusCode = test.when.downloadStatusCode
			}
			if test.when.uploadStatusCode != 0 {
				server.uploadStatusCode = test.when.uploadStatusCode
			}
			if test.when.recognizeStatusCode != 0 {
				server.recognizeStatusCode = test.when.recognizeStatusCode
			}
			if test.when.tokenStatusCode != 0 {
				server.tokenStatusCode = test.when.tokenStatusCode
			}

			client := server.clientWithTempDir(tempDir)
			client.token = test.when.existingToken

			meeting := &model.Meeting{
				ID:       uuid.New().String(),
				FilePath: &validFilePath,
			}

			// when
			text, err := client.TranscribeAudio(testContext(t), meeting)

			// then
			if test.want.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.want.errContains)
				assert.Len(t, server.uploadRequests, test.want.uploadRequests)
				assert.Len(t, server.recognizeRequests, test.want.recognizeRequests)
				assert.Len(t, server.statusRequests, test.want.statusRequests)
				assert.Len(t, server.downloadRequests, test.want.downloadRequests)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.want.text, text)
			assert.Len(t, server.uploadRequests, test.want.uploadRequests)
			assert.Len(t, server.recognizeRequests, test.want.recognizeRequests)
			assert.Len(t, server.statusRequests, test.want.statusRequests)
			assert.Len(t, server.downloadRequests, test.want.downloadRequests)
		})
	}
}
