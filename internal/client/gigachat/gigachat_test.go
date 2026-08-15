package gigachat

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/scouser-122/meeting-analyzer/internal/config"
	"github.com/scouser-122/meeting-analyzer/internal/domain/model"
	"github.com/scouser-122/meeting-analyzer/internal/logger"
	"github.com/scouser-122/meeting-analyzer/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testContext(t *testing.T) context.Context {
	t.Helper()
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	return context.WithValue(context.Background(), logger.LoggerKey, log)
}

type tokenRequest struct {
	RqUID string
	Basic string
}

type completionsRequest struct {
	RequestID string
	Bearer    string
	Body      GigaChatCompletionsRequest
}

type testServer struct {
	*httptest.Server

	tokenRequests       []tokenRequest
	completionsRequests []completionsRequest

	tokenResponse         any
	completionsResponse   any
	tokenStatusCode       int
	completionsStatusCode int
}

func newTestServer(t *testing.T) *testServer {
	t.Helper()

	ts := &testServer{
		tokenResponse: GigaAccessToken{
			Token:     "test-access-token",
			ExpiresAt: time.Now().Add(time.Hour).Unix(),
		},
		completionsResponse: GigaChatCompletionsResponse{
			Choices: []GigaChatCompletionsResponseChoice{
				{
					Message: GigaChatCompletionsMessage{
						Role:    "assistant",
						Content: "test answer",
					},
				},
			},
		},
		tokenStatusCode:       http.StatusOK,
		completionsStatusCode: http.StatusOK,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/oauth", func(w http.ResponseWriter, r *http.Request) {
		body := r.Body
		defer body.Close()

		ts.tokenRequests = append(ts.tokenRequests, tokenRequest{
			RqUID: r.Header.Get("RqUID"),
			Basic: r.Header.Get("Authorization"),
		})

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(ts.tokenStatusCode)
		if ts.tokenResponse != nil {
			require.NoError(t, json.NewEncoder(w).Encode(ts.tokenResponse))
		}
	})
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		body := r.Body
		defer body.Close()

		var reqBody GigaChatCompletionsRequest
		_ = json.NewDecoder(body).Decode(&reqBody)

		ts.completionsRequests = append(ts.completionsRequests, completionsRequest{
			RequestID: r.Header.Get("X-Request-ID"),
			Bearer:    r.Header.Get("Authorization"),
			Body:      reqBody,
		})

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(ts.completionsStatusCode)
		if ts.completionsResponse != nil {
			require.NoError(t, json.NewEncoder(w).Encode(ts.completionsResponse))
		}
	})

	ts.Server = httptest.NewServer(mux)
	return ts
}

func (ts *testServer) client() *GigaChatClient {
	return &GigaChatClient{
		config: &config.GigaChatConfig{
			GetTokenAddress: ts.Server.URL,
			ServerAddress:   ts.Server.URL,
			AuthKey:         "test-auth-key",
			Model:           "test-model",
		},
	}
}

func TestGigaChatClient_getToken(t *testing.T) {
	type when struct {
		existingToken   GigaAccessToken
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
				existingToken: GigaAccessToken{
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
				existingToken: GigaAccessToken{
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
				existingToken: GigaAccessToken{},
			},
			want: want{
				token:         "test-access-token",
				tokenRequests: 1,
			},
		},
		{
			name: "oauth returns non-200 status",
			when: when{
				existingToken:   GigaAccessToken{},
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

			client := server.client()
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
				assert.Equal(t, "Basic test-auth-key", server.tokenRequests[0].Basic)
				assert.NotEmpty(t, server.tokenRequests[0].RqUID)
			}
		})
	}
}

func TestGigaChatClient_SummarizeTranscription(t *testing.T) {
	type when struct {
		existingToken         GigaAccessToken
		tokenStatusCode       int
		completionsResponse   any
		completionsStatusCode int
	}

	type want struct {
		answer              string
		wantErr             bool
		errContains         string
		tokenRequests       int
		completionsRequests int
	}

	var testsList = []struct {
		name string
		when when
		want want
	}{
		{
			name: "successful summarization",
			when: when{
				existingToken: GigaAccessToken{
					Token:     "cached-token",
					ExpiresAt: time.Now().Add(time.Hour).Unix(),
				},
				completionsResponse: GigaChatCompletionsResponse{
					Choices: []GigaChatCompletionsResponseChoice{
						{
							Message: GigaChatCompletionsMessage{
								Role:    "assistant",
								Content: "Краткая выжимка встречи.",
							},
						},
					},
				},
			},
			want: want{
				answer:              "Краткая выжимка встречи.",
				tokenRequests:       0,
				completionsRequests: 1,
			},
		},
		{
			name: "token request fails",
			when: when{
				existingToken:   GigaAccessToken{},
				tokenStatusCode: http.StatusInternalServerError,
			},
			want: want{
				wantErr:       true,
				errContains:   "failed to get token",
				tokenRequests: 1,
			},
		},
		{
			name: "completions returns non-200 status",
			when: when{
				existingToken: GigaAccessToken{
					Token:     "cached-token",
					ExpiresAt: time.Now().Add(time.Hour).Unix(),
				},
				completionsStatusCode: http.StatusInternalServerError,
				completionsResponse:   map[string]string{"error": "internal"},
			},
			want: want{
				wantErr:             true,
				errContains:         "request failed",
				tokenRequests:       0,
				completionsRequests: 1,
			},
		},
		{
			name: "response does not contain assistant answer",
			when: when{
				existingToken: GigaAccessToken{
					Token:     "cached-token",
					ExpiresAt: time.Now().Add(time.Hour).Unix(),
				},
				completionsResponse: GigaChatCompletionsResponse{
					Choices: []GigaChatCompletionsResponseChoice{
						{
							Message: GigaChatCompletionsMessage{
								Role:    "user",
								Content: "some content",
							},
						},
					},
				},
			},
			want: want{
				wantErr:             true,
				errContains:         "response doesn't contain answer",
				tokenRequests:       0,
				completionsRequests: 1,
			},
		},
	}

	for _, test := range testsList {
		t.Run(test.name, func(t *testing.T) {
			// setup
			server := newTestServer(t)
			defer server.Close()

			if test.when.tokenStatusCode != 0 {
				server.tokenStatusCode = test.when.tokenStatusCode
			}
			if test.when.completionsResponse != nil {
				server.completionsResponse = test.when.completionsResponse
			} else {
				server.completionsResponse = nil
			}
			if test.when.completionsStatusCode != 0 {
				server.completionsStatusCode = test.when.completionsStatusCode
			}

			client := server.client()
			client.token = test.when.existingToken

			meeting := &model.Meeting{ID: uuid.New().String()}

			// when
			answer, err := client.SummarizeTranscription(testContext(t), meeting, "транскрипция встречи")

			// then
			if test.want.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.want.errContains)
				assert.Len(t, server.tokenRequests, test.want.tokenRequests)
				assert.Len(t, server.completionsRequests, test.want.completionsRequests)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.want.answer, answer)
			assert.Len(t, server.tokenRequests, test.want.tokenRequests)
			assert.Len(t, server.completionsRequests, test.want.completionsRequests)

			if test.want.completionsRequests > 0 {
				assert.Equal(t, "Bearer cached-token", server.completionsRequests[0].Bearer)
				assert.NotEmpty(t, server.completionsRequests[0].RequestID)
				assert.Equal(t, "test-model", server.completionsRequests[0].Body.Model)
				assert.Len(t, server.completionsRequests[0].Body.Messages, 2)
				assert.Contains(t, server.completionsRequests[0].Body.Messages[1].Content, "транскрипция встречи")
			}
		})
	}
}

func TestGigaChatClient_ExtractIntent(t *testing.T) {
	type when struct {
		existingToken         GigaAccessToken
		tokenStatusCode       int
		completionsResponse   any
		completionsStatusCode int
	}

	type want struct {
		intent              *models.QueryIntent
		wantErr             bool
		errContains         string
		tokenRequests       int
		completionsRequests int
	}

	validIntentJSON, err := json.Marshal(models.QueryIntent{
		IsMeetingQuery: true,
		Keywords:       []string{"встреча", "продажи"},
		Topic:          "итоги встречи по продажам",
	})
	require.NoError(t, err)

	var testsList = []struct {
		name string
		when when
		want want
	}{
		{
			name: "successful intent extraction",
			when: when{
				existingToken: GigaAccessToken{
					Token:     "cached-token",
					ExpiresAt: time.Now().Add(time.Hour).Unix(),
				},
				completionsResponse: GigaChatCompletionsResponse{
					Choices: []GigaChatCompletionsResponseChoice{
						{
							Message: GigaChatCompletionsMessage{
								Role:    "assistant",
								Content: string(validIntentJSON),
							},
						},
					},
				},
			},
			want: want{
				intent: &models.QueryIntent{
					IsMeetingQuery: true,
					Keywords:       []string{"встреча", "продажи"},
					Topic:          "итоги встречи по продажам",
				},
				tokenRequests:       0,
				completionsRequests: 1,
			},
		},
		{
			name: "token request fails",
			when: when{
				existingToken:   GigaAccessToken{},
				tokenStatusCode: http.StatusInternalServerError,
			},
			want: want{
				wantErr:       true,
				errContains:   "failed to get token",
				tokenRequests: 1,
			},
		},
		{
			name: "completions returns non-200 status",
			when: when{
				existingToken: GigaAccessToken{
					Token:     "cached-token",
					ExpiresAt: time.Now().Add(time.Hour).Unix(),
				},
				completionsStatusCode: http.StatusBadRequest,
				completionsResponse:   map[string]string{"error": "bad request"},
			},
			want: want{
				wantErr:             true,
				errContains:         "request failed",
				tokenRequests:       0,
				completionsRequests: 1,
			},
		},
		{
			name: "response does not contain assistant answer",
			when: when{
				existingToken: GigaAccessToken{
					Token:     "cached-token",
					ExpiresAt: time.Now().Add(time.Hour).Unix(),
				},
				completionsResponse: GigaChatCompletionsResponse{
					Choices: []GigaChatCompletionsResponseChoice{},
				},
			},
			want: want{
				wantErr:             true,
				errContains:         "response doesn't contain answer",
				tokenRequests:       0,
				completionsRequests: 1,
			},
		},
		{
			name: "assistant answer is not valid json",
			when: when{
				existingToken: GigaAccessToken{
					Token:     "cached-token",
					ExpiresAt: time.Now().Add(time.Hour).Unix(),
				},
				completionsResponse: GigaChatCompletionsResponse{
					Choices: []GigaChatCompletionsResponseChoice{
						{
							Message: GigaChatCompletionsMessage{
								Role:    "assistant",
								Content: "not a json",
							},
						},
					},
				},
			},
			want: want{
				wantErr:             true,
				errContains:         "parse intent json",
				tokenRequests:       0,
				completionsRequests: 1,
			},
		},
	}

	for _, test := range testsList {
		t.Run(test.name, func(t *testing.T) {
			// setup
			server := newTestServer(t)
			defer server.Close()

			if test.when.tokenStatusCode != 0 {
				server.tokenStatusCode = test.when.tokenStatusCode
			}
			if test.when.completionsResponse != nil {
				server.completionsResponse = test.when.completionsResponse
			} else {
				server.completionsResponse = nil
			}
			if test.when.completionsStatusCode != 0 {
				server.completionsStatusCode = test.when.completionsStatusCode
			}

			client := server.client()
			client.token = test.when.existingToken

			// when
			intent, err := client.ExtractIntent(testContext(t), "Что обсуждали на встрече по продажам?")

			// then
			if test.want.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.want.errContains)
				assert.Len(t, server.tokenRequests, test.want.tokenRequests)
				assert.Len(t, server.completionsRequests, test.want.completionsRequests)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.want.intent, intent)
			assert.Len(t, server.tokenRequests, test.want.tokenRequests)
			assert.Len(t, server.completionsRequests, test.want.completionsRequests)

			if test.want.completionsRequests > 0 {
				assert.Equal(t, "Bearer cached-token", server.completionsRequests[0].Bearer)
				assert.NotEmpty(t, server.completionsRequests[0].RequestID)
				assert.Equal(t, "test-model", server.completionsRequests[0].Body.Model)
				assert.Contains(t, server.completionsRequests[0].Body.Messages[1].Content, "Что обсуждали на встрече по продажам?")
			}
		})
	}
}
