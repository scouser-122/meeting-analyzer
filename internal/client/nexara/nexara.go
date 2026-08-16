package nexara

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/scouser-122/meeting-analyzer/internal/config"
	"github.com/scouser-122/meeting-analyzer/internal/domain/model"
	"github.com/scouser-122/meeting-analyzer/internal/logger"
)

// NexaraClient is an audio processor implementation backed by the Nexara API.
type NexaraClient struct {
	config *config.NexaraConfig
	client *resty.Client
}

// NewNexaraClient creates a new Nexara API client from server configuration.
func NewNexaraClient(
	serverConfig *config.ServerConfig,
) *NexaraClient {
	return &NexaraClient{
		config: serverConfig.Nexara,
		client: createRestyClient(),
	}
}

// TranscribeAudio uploads the meeting audio to SaluteSpeech and returns the recognized text.
func (n *NexaraClient) TranscribeAudio(ctx context.Context, meeting *model.Meeting) (string, error) {
	logger := logger.GetSlogLoggerFromContext(ctx)
	logger.Info("nexara client: start transribing audio")

	file, err := os.Open(*meeting.FilePath)
	if err != nil {
		return "", fmt.Errorf("nexara client failed to open audio file, err: %s, path: %s", err, *meeting.FilePath)
	}
	defer file.Close()

	var uploadResponse NexaraRecognizedText
	resp, err := n.client.R().
		SetContext(ctx).
		SetHeader("Authorization", fmt.Sprintf("Bearer %s", n.config.ApiToken)).
		SetFile("file", *meeting.FilePath).
		SetFormData(map[string]string{
			"model": "whisper-1",
		}).
		SetResult(&uploadResponse).
		Post(fmt.Sprintf("%s/v1/audio/transcriptions", n.config.ServerAddress))

	if err != nil {
		return "", fmt.Errorf("nexara client failed to upload file, err: %s", err)
	}
	if resp.StatusCode() != http.StatusOK {
		return "", fmt.Errorf("nexara client failed to upload file, status: %d", resp.StatusCode())
	}

	logger.Info("nexara client: transribing audio finished succesfully")

	return uploadResponse.Text, nil
}

func createRestyClient() *resty.Client {
	client := resty.New()
	client.SetRetryCount(3).
		SetRetryWaitTime(1 * time.Second).
		SetRetryMaxWaitTime(2 * time.Second)
	client.AddRetryCondition(
		func(r *resty.Response, err error) bool {
			return err != nil ||
				r.StatusCode() == http.StatusRequestTimeout ||
				r.StatusCode() == http.StatusTooManyRequests ||
				r.StatusCode() == http.StatusInternalServerError
		},
	)
	return client
}
