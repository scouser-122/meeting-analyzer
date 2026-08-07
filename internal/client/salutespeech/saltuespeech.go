package salutespeech

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/google/uuid"
	"github.com/scouser-122/meeting-analyzer/internal/config"
	"github.com/scouser-122/meeting-analyzer/internal/domain/model"
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

func (s *SaluteSpeechClient) TranscribeAudio(meeting *model.Meeting) (string, error) {
	slog.Info("salute speech client: start transribing audio", "name", *meeting.MeetingName, "meetingID", meeting.ID)

	fileID, err := s.sendFile(*meeting.FilePath)
	if err != nil {
		return "", err
	}

	taskID, err := s.createRecognizeTask(fileID)
	if err != nil {
		return "", err
	}

	for i := 0; i < s.config.StatusPollingMaxAttempts; i++ {
		time.Sleep(time.Duration(s.config.StatusPollingInterval) * time.Second)
		status, err := s.getRecognizeStatus(taskID)
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

	resultFilePath, err := s.getFileWithResult(fileID)
	if err != nil {
		return "", err
	}

	transcription, err := s.getTranscriptionFromFile(resultFilePath)
	if err != nil {
		return "", err
	}

	slog.Info("salute speech client: transribing audio finished succesfully", "name", *meeting.MeetingName, "meetingID", meeting.ID)

	return transcription, nil
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

func (s *SaluteSpeechClient) getToken() (string, error) {
	if s.token.Token != "" && time.Now().Before(time.Unix(s.token.ExpiresAt, 0)) {
		return s.token.Token, nil
	}

	client := s.createRestyClient()
	url := fmt.Sprintf("%s/api/v2/oauth", s.config.ServerAddress)

	resp, err := client.R().
		SetResult(&s.token).
		Get(url)
	if err != nil {
		return "", err
	}
	if resp.StatusCode() != http.StatusOK {
		return "", fmt.Errorf("salute speech client failed to get token, status: %s", resp.StatusCode())
	}
	return s.token.Token, nil
}

func (s *SaluteSpeechClient) sendFile(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("salute speech client failed to open audio file, err: %s, path: %s", err, filePath)
	}
	defer file.Close()

	client := s.createRestyClient()

	token, err := s.getToken()
	if err != nil {
		return "", err
	}

	var uploadResponse SaluteSpeechUploadResponse
	requestID := uuid.New().String()
	resp, err := client.R().
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
		return "", fmt.Errorf("salute speech client failed to upload file, status: %s", resp.StatusCode())
	}
	slog.Info("salute speech client successfully sent file", "path", filePath, "fileID", uploadResponse.Result.RequestFileId)
	return uploadResponse.Result.RequestFileId, nil
}

func (s *SaluteSpeechClient) createRecognizeTask(fileID string) (string, error) {
	client := s.createRestyClient()

	token, err := s.getToken()
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
		return "", fmt.Errorf("salute speech client failed to start recognize, http status: %s, fileID: %s", resp.StatusCode(), fileID)
	}
	if response.Result.Status == "ERROR" {
		return "", fmt.Errorf("salute speech client failed to start recognize, status: %s, fileID: %s", response.Result.Status, fileID)
	}

	slog.Info("salute speech client successfully create recognize task", "fileID", fileID)
	return response.Result.ID, nil
}

func (s *SaluteSpeechClient) getRecognizeStatus(taskID string) (model.TaskStatus, error) {
	client := s.createRestyClient()

	token, err := s.getToken()
	if err != nil {
		return "", err
	}

	requestID := uuid.New().String()
	resp, err := client.R().
		SetHeader("X-Request-ID", requestID).
		SetHeader("Authorization", fmt.Sprintf("Bearer %s", token)).
		Get(fmt.Sprintf("%s/rest/v1/task:get?id=%s", s.config.ServerAddress, taskID))

	if err != nil {
		return "", fmt.Errorf("salute speech client failed to get recognize status, err: %s, taskID: %s", err, taskID)
	}
	if resp.StatusCode() != http.StatusOK {
		return "", fmt.Errorf("salute speech client failed to get recognize status, http status: %s, taskID: %s", resp.StatusCode(), taskID)
	}
	var status model.TaskStatus
	status = model.TaskStatus(resp.Body())
	return status, nil
}

func (s *SaluteSpeechClient) getFileWithResult(fileID string) (string, error) {
	client := s.createRestyClient()

	token, err := s.getToken()
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(s.config.RecognizedFileDir, 0o755); err != nil {
		return "", fmt.Errorf("salute speech client mkdir failed, err: %s, fileID: %s", err, fileID)
	}

	savePath := filepath.Join(s.config.RecognizedFileDir, fileID+".json")

	requestID := uuid.New().String()
	resp, err := client.R().
		SetHeader("X-Request-ID", requestID).
		SetHeader("Authorization", fmt.Sprintf("Bearer %s", token)).
		SetOutput(savePath).
		Get(fmt.Sprintf("%s/rest/v1/data:download?id=%s", s.config.ServerAddress, fileID))

	if err != nil {
		return "", fmt.Errorf("salute speech client failed to start recognize, err: %s, fileID: %s", err, fileID)
	}
	if resp.StatusCode() != http.StatusOK {
		return "", fmt.Errorf("salute speech client failed to start recognize, http status: %s, fileID: %s", resp.StatusCode(), fileID)
	}
	return savePath, nil
}

func (s *SaluteSpeechClient) getTranscriptionFromFile(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("salute speech client failed to open recognize file, err: %s, filePath: %s", err, filePath)
	}

	var textData SaluteSpeechRecognizedText
	err = json.Unmarshal(data, &textData)
	if err != nil {
		return "", fmt.Errorf("salute speech client failed unmarshalling JSON, err: %s, filePath: %s", err, filePath)
	}

	return textData.Text, nil
}
