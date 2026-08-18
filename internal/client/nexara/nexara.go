package nexara

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/scouser-122/meeting-analyzer/internal/config"
	"github.com/scouser-122/meeting-analyzer/internal/domain/model"
	"github.com/scouser-122/meeting-analyzer/internal/logger"
	"github.com/scouser-122/meeting-analyzer/internal/storage"
)

// NexaraClient is an audio processor implementation backed by the Nexara API.
type NexaraClient struct {
	config      *config.NexaraConfig
	client      *resty.Client
	fileStorage storage.FileStorage
}

// NewNexaraClient creates a new Nexara API client from server configuration.
func NewNexaraClient(
	serverConfig *config.ServerConfig,
	fileStorage storage.FileStorage,
) *NexaraClient {
	return &NexaraClient{
		config:      serverConfig.Nexara,
		client:      createRestyClient(),
		fileStorage: fileStorage,
	}
}

// TranscribeAudio uploads the meeting audio to Nexara and returns the recognized text.
func (n *NexaraClient) TranscribeAudio(ctx context.Context, meeting *model.Meeting) (string, error) {
	logger := logger.GetSlogLoggerFromContext(ctx)
	logger.Info("nexara client: start transribing audio")

	file, err := n.fileStorage.Open(ctx, *meeting.FilePath)
	if err != nil {
		return "", fmt.Errorf("nexara client failed to open audio file, err: %s, path: %s", err, *meeting.FilePath)
	}
	defer file.Close()

	filename := "audio.mp3"
	if meeting.OriginalFilename != nil && *meeting.OriginalFilename != "" {
		filename = *meeting.OriginalFilename
	}

	var uploadResponse NexaraRecognizedText
	resp, err := n.client.R().
		SetContext(ctx).
		SetHeader("Authorization", fmt.Sprintf("Bearer %s", n.config.ApiToken)).
		SetFileReader("file", filename, file).
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
