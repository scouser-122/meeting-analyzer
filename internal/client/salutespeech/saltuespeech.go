package salutespeech

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/google/uuid"
	"github.com/scouser-122/meeting-analyzer/internal/config"
	"github.com/scouser-122/meeting-analyzer/internal/domain/model"
	"github.com/scouser-122/meeting-analyzer/internal/logger"
)

type SaluteSpeechClient struct {
	config *config.SaluteSpeechConfig
	token  SaluteAuthToken
}

func NewSaluteSpeechClient(
	serverConfig *config.ServerConfig,
) *SaluteSpeechClient {
	return &SaluteSpeechClient{
		config: serverConfig.SaluteSpeech,
	}
}

func (s *SaluteSpeechClient) TranscribeAudio(ctx context.Context, meeting *model.Meeting) (string, error) {
	logger := logger.GetSlogLoggerFromContext(ctx)
	logger.Info("salute speech client: start transribing audio")

	fileID, err := s.sendFile(ctx, *meeting.FilePath)
	if err != nil {
		return "", err
	}

	taskID, err := s.createRecognizeTask(ctx, fileID)
	if err != nil {
		return "", err
	}

	for i := 0; i < s.config.StatusPollingMaxAttempts; i++ {
		time.Sleep(time.Duration(s.config.StatusPollingInterval) * time.Second)
		var status model.TaskStatus
		status, err = s.getRecognizeStatus(ctx, taskID)
		if err != nil {
			return "", err
		}
		if status == "ERROR" {
			return "", fmt.Errorf("salute speech client: recognize status error. taskID: %s", taskID)
		}
		if status == "CANCELED" {
			return "", fmt.Errorf("salute speech client: recognize status cancelled. taskID: %s", taskID)
		}
		if status == "DONE" {
			break
		}
	}

	resultText, err := s.getResultFromFile(ctx, fileID)
	if err != nil {
		return "", err
	}

	logger.Info("salute speech client: transribing audio finished succesfully")

	return resultText, nil
}

func (s *SaluteSpeechClient) createRestyClient() *resty.Client {
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

func (s *SaluteSpeechClient) getToken(ctx context.Context) (string, error) {
	if s.token.Token != "" && time.Now().Before(time.Unix(s.token.ExpiresAt, 0)) {
		return s.token.Token, nil
	}

	client := s.createRestyClient()
	url := fmt.Sprintf("%s/api/v2/oauth", s.config.ServerAddress)

	resp, err := client.R().
		SetContext(ctx).
		SetResult(&s.token).
		Get(url)
	if err != nil {
		return "", err
	}
	if resp.StatusCode() != http.StatusOK {
		return "", fmt.Errorf("salute speech client failed to get token, status: %d", resp.StatusCode())
	}
	return s.token.Token, nil
}

func (s *SaluteSpeechClient) sendFile(ctx context.Context, filePath string) (string, error) {
	logger := logger.GetSlogLoggerFromContext(ctx)
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("salute speech client failed to open audio file, err: %s, path: %s", err, filePath)
	}
	defer file.Close()

	client := s.createRestyClient()

	token, err := s.getToken(ctx)
	if err != nil {
		return "", err
	}

	var uploadResponse SaluteSpeechUploadResponse
	requestID := uuid.New().String()
	resp, err := client.R().
		SetContext(ctx).
		SetHeader("Content-Type", "audio/mpeg").
		SetHeader("X-Request-ID", requestID).
		SetHeader("Authorization", fmt.Sprintf("Bearer %s", token)).
		SetBody(file).
		SetResult(&uploadResponse).
		Post(fmt.Sprintf("%s/rest/v1/data:upload", s.config.ServerAddress))

	if err != nil {
		return "", fmt.Errorf("salute speech client failed to upload file, err: %s", err)
	}
	if resp.StatusCode() != http.StatusOK {
		return "", fmt.Errorf("salute speech client failed to upload file, status: %d", resp.StatusCode())
	}
	logger.Info("salute speech client successfully sent file", "path", filePath, "fileID", uploadResponse.Result.RequestFileId)
	return uploadResponse.Result.RequestFileId, nil
}

func (s *SaluteSpeechClient) createRecognizeTask(ctx context.Context, fileID string) (string, error) {
	logger := logger.GetSlogLoggerFromContext(ctx)
	client := s.createRestyClient()

	token, err := s.getToken(ctx)
	if err != nil {
		return "", err
	}

	request := SaluteSpeechRecognizeRequest{
		Options: SaluteSpeechRecognizeRequestOptions{
			AudioEncoding: "MP3",
		},
	}

	var response SaluteSpeechRecognizeResponse
	requestID := uuid.New().String()
	resp, err := client.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetHeader("X-Request-ID", requestID).
		SetHeader("Authorization", fmt.Sprintf("Bearer %s", token)).
		SetBody(request).
		SetResult(&response).
		Post(fmt.Sprintf("%s/rest/v1/speech:async_recognize", s.config.ServerAddress))

	if err != nil {
		return "", fmt.Errorf("salute speech client failed to start recognize, err: %s, fileID: %s", err, fileID)
	}
	if resp.StatusCode() != http.StatusOK {
		return "", fmt.Errorf("salute speech client failed to start recognize, http status: %d, fileID: %s", resp.StatusCode(), fileID)
	}
	if response.Result.Status == "ERROR" {
		return "", fmt.Errorf("salute speech client failed to start recognize, status: %s, fileID: %s", response.Result.Status, fileID)
	}

	logger.Info("salute speech client successfully create recognize task", "fileID", fileID, "taskID", response.Result.ID, "status", response.Result.Status)
	return response.Result.ID, nil
}

func (s *SaluteSpeechClient) getRecognizeStatus(ctx context.Context, taskID string) (model.TaskStatus, error) {
	logger := logger.GetSlogLoggerFromContext(ctx)
	client := s.createRestyClient()

	token, err := s.getToken(ctx)
	if err != nil {
		return "", err
	}

	requestID := uuid.New().String()
	resp, err := client.R().
		SetContext(ctx).
		SetHeader("X-Request-ID", requestID).
		SetHeader("Authorization", fmt.Sprintf("Bearer %s", token)).
		Get(fmt.Sprintf("%s/rest/v1/task:get?id=%s", s.config.ServerAddress, taskID))

	if err != nil {
		return "", fmt.Errorf("salute speech client failed to get recognize status, err: %s, taskID: %s", err, taskID)
	}
	if resp.StatusCode() != http.StatusOK {
		return "", fmt.Errorf("salute speech client failed to get recognize status, http status: %d, taskID: %s", resp.StatusCode(), taskID)
	}
	var status model.TaskStatus
	status = model.TaskStatus(resp.Body())

	logger.Info("salute speech client get recognize status", "taskID", taskID, "status", status)
	return status, nil
}

func (s *SaluteSpeechClient) getResultFromFile(ctx context.Context, fileID string) (string, error) {
	client := s.createRestyClient()

	token, err := s.getToken(ctx)
	if err != nil {
		return "", err
	}

	if err = os.MkdirAll(s.config.RecognizedFileDir, 0o755); err != nil {
		return "", fmt.Errorf("salute speech client mkdir failed, err: %s, fileID: %s", err, fileID)
	}

	transcriptionJsonPath := filepath.Join(s.config.RecognizedFileDir, fileID+".json")

	requestID := uuid.New().String()
	resp, err := client.R().
		SetContext(ctx).
		SetHeader("X-Request-ID", requestID).
		SetHeader("Authorization", fmt.Sprintf("Bearer %s", token)).
		SetOutput(transcriptionJsonPath).
		Get(fmt.Sprintf("%s/rest/v1/data:download?id=%s", s.config.ServerAddress, fileID))

	if err != nil {
		return "", fmt.Errorf("salute speech client failed to start recognize, err: %s, fileID: %s", err, fileID)
	}
	if resp.StatusCode() != http.StatusOK {
		return "", fmt.Errorf("salute speech client failed to start recognize, http status: %d, fileID: %s", resp.StatusCode(), fileID)
	}

	data, err := os.ReadFile(transcriptionJsonPath)
	if err != nil {
		return "", fmt.Errorf("salute speech client failed to open recognize file, err: %s, filePath: %s", err, transcriptionJsonPath)
	}

	var textData SaluteSpeechRecognizedText
	err = json.Unmarshal(data, &textData)
	if err != nil {
		return "", fmt.Errorf("salute speech client failed unmarshalling JSON, err: %s, filePath: %s", err, transcriptionJsonPath)
	}

	err = os.RemoveAll(transcriptionJsonPath)
	if err != nil {
		return "", fmt.Errorf("salute speech client failed to delete recognize file, err: %s, filePath: %s", err, transcriptionJsonPath)
	}

	return textData.Text, nil
}
