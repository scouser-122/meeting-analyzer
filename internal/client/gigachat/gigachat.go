package gigachat

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/google/uuid"
	"github.com/scouser-122/meeting-analyzer/internal/config"
	"github.com/scouser-122/meeting-analyzer/internal/domain/model"
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

func (c *GigaChatClient) SummarizeTranscription(meeting *model.Meeting, text string) (string, error) {
	client := resty.New()

	token, err := c.getToken()
	if err != nil {
		return "", err
	}

	request := GigaChatCompletionsRequest{
		Model: "GigaChat",
		Messages: []GigaChatCompletionsMessage{
			{
				Role:    "user",
				Content: fmt.Sprintf("Напиши краткую выжимку по следующей транскрипции встречи:\n%s", text),
			},
		},
	}

	var response GigaChatCompletionsResponse
	requestID := uuid.New().String()
	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("X-Request-ID", requestID).
		SetHeader("Authorization", fmt.Sprintf("Bearer %s", token)).
		SetBody(request).
		SetResult(&response).
		Post(fmt.Sprintf("%s/v1/chat/completions", c.config.ServerAddress))

	if err != nil {
		return "", fmt.Errorf("gigachat client request failed, err: %s, meetingID: %s", err, meeting.ID)
	}
	if resp.StatusCode() != http.StatusOK {
		return "", fmt.Errorf("gigachat client request failed, http status: %s, meetingID: %s", resp.StatusCode(), meeting.ID)
	}

	var answer string
	for _, c := range response.Choises {
		if c.Message.Role == "assistant" {
			answer = c.Message.Content
		}
	}
	if answer == "" {
		return "", fmt.Errorf("gigachat client response doesn't contain answer, meetingID: %s", meeting.ID)
	}

	return answer, nil
}

func (c *GigaChatClient) getToken() (string, error) {
	if c.token.Token != "" && time.Now().Before(time.Unix(c.token.ExpiresAt, 0)) {
		return c.token.Token, nil
	}

	client := resty.New()
	url := fmt.Sprintf("%s/api/v2/oauth", c.config.GetTokenAddress)

	requestID := uuid.New().String()
	resp, err := client.R().
		SetHeader("RqUID", requestID).
		SetBody("scope=GIGACHAT_API_PERS").
		SetResult(&c.token).
		Get(url)
	if err != nil {
		return "", err
	}
	if resp.StatusCode() != http.StatusOK {
		return "", fmt.Errorf("gigachat client failed to get token, status: %s", resp.StatusCode())
	}
	return c.token.Token, nil
}
