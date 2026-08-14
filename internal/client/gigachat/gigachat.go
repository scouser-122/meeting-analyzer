package gigachat

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/scouser-122/meeting-analyzer/internal/config"
	"github.com/scouser-122/meeting-analyzer/internal/domain/model"
	"github.com/scouser-122/meeting-analyzer/internal/logger"
	"github.com/scouser-122/meeting-analyzer/internal/models"
)

type GigaChatClient struct {
	config *config.GigaChatConfig
	token  GigaAccessToken
}

func NewGigaChatClient(
	serverConfig *config.ServerConfig,
) *GigaChatClient {
	return &GigaChatClient{
		config: serverConfig.GigaChat,
	}
}

func (c *GigaChatClient) SummarizeTranscription(ctx context.Context, meeting *model.Meeting, transcriptionText string) (string, error) {
	logger := logger.GetSlogLoggerFromContext(ctx)
	logger.Info("GigaChat client: start summarizing transcription")

	token, err := c.getToken(ctx)
	if err != nil {
		return "", err
	}

	request := GigaChatCompletionsRequest{
		Model: c.config.Model,
		Messages: []GigaChatCompletionsMessage{
			{
				Role:    "system",
				Content: "Ты - помощник по обработке данных",
			},
			{
				Role:    "user",
				Content: fmt.Sprintf("Напиши краткую выжимку по следующей транскрипции встречи (не больше двух предложений):\n%s", transcriptionText),
			},
		},
	}

	var response GigaChatCompletionsResponse
	requestID := uuid.New().String()
	client := resty.New()
	resp, err := client.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetHeader("X-Request-ID", requestID).
		SetHeader("Authorization", fmt.Sprintf("Bearer %s", token)).
		SetBody(request).
		SetResult(&response).
		Post(fmt.Sprintf("%s/v1/chat/completions", c.config.ServerAddress))

	if err != nil {
		return "", fmt.Errorf("GigaChat client: request failed, err: %s, meetingID: %s", err, meeting.ID)
	}
	if resp.StatusCode() != http.StatusOK {
		return "", fmt.Errorf("GigaChat client: request failed, http status: %d, body: %s, meetingID: %s", resp.StatusCode(), string(resp.Body()), meeting.ID)
	}

	var answer string
	for _, c := range response.Choices {
		if c.Message.Role == "assistant" {
			answer = c.Message.Content
		}
	}
	if answer == "" {
		return "", fmt.Errorf("GigaChat client: response doesn't contain answer, meetingID: %s", meeting.ID)
	}
	logger.Info("GigaChat client: transcription summarization finished successfully")

	return answer, nil
}

func (c *GigaChatClient) ExtractIntent(ctx context.Context, text string) (*models.QueryIntent, error) {
	logger := logger.GetSlogLoggerFromContext(ctx)
	logger.Info("GigaChat client: start extract intent")

	client := resty.New()

	token, err := c.getToken(ctx)
	if err != nil {
		return nil, err
	}

	const extractPrompt = `Определи, спрашивает ли пользователь про какую-то встречу/созвон (найти запись, выжимку, что обсуждали и т.п.).

Если НЕТ — верни {"is_meeting_query": false, "keywords": [], "topic": ""}.

Если ДА — извлеки ключевые слова и тему встречи.

Ответь ТОЛЬКО валидным JSON без пояснений и markdown, в формате:
{"is_meeting_query": true|false, "keywords": ["слово1", "слово2"], "topic": "краткая тема"}

Вопрос: %s`

	request := GigaChatCompletionsRequest{
		Model: c.config.Model,
		Messages: []GigaChatCompletionsMessage{
			{
				Role:    "system",
				Content: "Ты - помощник по обработке данных",
			},
			{
				Role:    "user",
				Content: fmt.Sprintf(extractPrompt, text),
			},
		},
	}

	var response GigaChatCompletionsResponse
	requestID := uuid.New().String()
	resp, err := client.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetHeader("X-Request-ID", requestID).
		SetHeader("Authorization", fmt.Sprintf("Bearer %s", token)).
		SetBody(request).
		SetResult(&response).
		Post(fmt.Sprintf("%s/v1/chat/completions", c.config.ServerAddress))

	if err != nil {
		return nil, fmt.Errorf("gigachat client extract intent request failed, err: %s", err)
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("gigachat client extract intent request failed, http status: %d", resp.StatusCode())
	}

	var answer string
	for _, c := range response.Choices {
		if c.Message.Role == "assistant" {
			answer = c.Message.Content
		}
	}
	if answer == "" {
		return nil, fmt.Errorf("gigachat client extract intent response doesn't contain answer")
	}

	var intent models.QueryIntent
	if err := json.Unmarshal([]byte(answer), &intent); err != nil {
		return nil, fmt.Errorf("parse intent json: %w", err)
	}
	logger.Info("GigaChat client: intent extracted", "intent", intent)
	return &intent, nil
}

func (c *GigaChatClient) getToken(ctx context.Context) (string, error) {
	if c.token.Token != "" && time.Now().Before(time.Unix(c.token.ExpiresAt, 0)) {
		return c.token.Token, nil
	}

	client := resty.New()
	url := fmt.Sprintf("%s/api/v2/oauth", c.config.GetTokenAddress)

	requestID := uuid.New().String()
	resp, err := client.R().
		SetContext(ctx).
		SetHeader("RqUID", requestID).
		SetHeader("Authorization", fmt.Sprintf("Basic %s", c.config.AuthKey)).
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetBody("scope=GIGACHAT_API_PERS").
		SetResult(&c.token).
		Post(url)
	if err != nil {
		return "", errors.WithStack(err)
	}
	if resp.StatusCode() != http.StatusOK {
		return "", fmt.Errorf("gigachat client failed to get token, status: %d, body: %s", resp.StatusCode(), string(resp.Body()))
	}
	return c.token.Token, nil
}
