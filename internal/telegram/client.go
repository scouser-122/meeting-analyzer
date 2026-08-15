package telegram

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/scouser-122/meeting-analyzer/internal/models"
)

// TelegramClient is an HTTP client for the backend API used by the Telegram bot.
type TelegramClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewClient creates a TelegramClient with the given backend base URL.
func NewClient(baseURL string) *TelegramClient {
	return &TelegramClient{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// Start registers a new user via the backend API.
func (c *TelegramClient) Start(userID string) (*models.CommonResponse, error) {
	body := map[string]string{"id": userID}
	resp, err := c.doJSON("POST", "/api/users/start", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusConflict {
		var errResp models.CommonResponse
		json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, fmt.Errorf("%s", errResp.Message)
	}

	var result models.CommonResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}

// UploadMeeting sends an audio file and metadata to the backend for processing.
func (c *TelegramClient) UploadMeeting(userID, name, filename string, file io.Reader) (*models.LoadResponse, error) {
	metadata, err := json.Marshal(map[string]string{
		"user_id": userID,
		"name":    name,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal metadata: %w", err)
	}

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	filePart, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, fmt.Errorf("create file part: %w", err)
	}
	if _, err = io.Copy(filePart, file); err != nil {
		return nil, fmt.Errorf("write file part: %w", err)
	}

	if err = writer.WriteField("metadata", string(metadata)); err != nil {
		return nil, fmt.Errorf("write metadata field: %w", err)
	}

	if err = writer.Close(); err != nil {
		return nil, fmt.Errorf("close multipart writer: %w", err)
	}

	contentType := writer.FormDataContentType()

	req, err := http.NewRequest(http.MethodPost, c.BaseURL+"/api/meetings/load", &buf)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		var errResp models.CommonResponse
		json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, fmt.Errorf("%s", errResp.Message)
	}

	var result models.LoadResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}

// List returns the list of meetings for the specified user.
func (c *TelegramClient) List(userID string) ([]models.MeetingResponseData, error) {
	url := fmt.Sprintf("%s/api/meetings/list?user_id=%s", c.BaseURL, userID)
	resp, err := c.HTTPClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp models.CommonResponse
		json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, fmt.Errorf("%s", errResp.Message)
	}

	var meetings []models.MeetingResponseData
	if err := json.NewDecoder(resp.Body).Decode(&meetings); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return meetings, nil
}

// Status returns the processing status of the specified meeting.
func (c *TelegramClient) Status(userID, meetingID string) (*models.MeetingResponseData, error) {
	url := fmt.Sprintf("%s/api/meetings/status?user_id=%s&meeting_id=%s", c.BaseURL, userID, meetingID)
	resp, err := c.HTTPClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp models.CommonResponse
		json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, fmt.Errorf("%s", errResp.Message)
	}

	var meeting models.MeetingResponseData
	if err := json.NewDecoder(resp.Body).Decode(&meeting); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &meeting, nil
}

// Transcription returns the transcription text for the specified meeting.
func (c *TelegramClient) Transcription(userID, meetingID string) (*models.TranscriptionResponseData, error) {
	url := fmt.Sprintf("%s/api/meetings/transcription?user_id=%s&meeting_id=%s", c.BaseURL, userID, meetingID)
	resp, err := c.HTTPClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp models.CommonResponse
		json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, fmt.Errorf("%s", errResp.Message)
	}

	var result models.TranscriptionResponseData
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}

// Find searches meetings by keywords through the backend API.
func (c *TelegramClient) Find(userID, keywords string) ([]models.MeetingResponseData, error) {
	body := map[string]string{
		"user_id":   userID,
		"key_words": keywords,
	}
	resp, err := c.doJSON("POST", "/api/meetings/find", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp models.CommonResponse
		json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, fmt.Errorf("%s", errResp.Message)
	}

	var meetings []models.MeetingResponseData
	if err := json.NewDecoder(resp.Body).Decode(&meetings); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return meetings, nil
}

// Chat sends a question to the backend chat assistant and returns the answer.
func (c *TelegramClient) Chat(userID, question string) (*models.ChatResponse, error) {
	body := map[string]string{
		"user_id":  userID,
		"question": question,
	}
	resp, err := c.doJSON("POST", "/api/chat", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp models.CommonResponse
		json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, fmt.Errorf("%s", errResp.Message)
	}

	var result models.ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}

func (c *TelegramClient) doJSON(method, path string, body interface{}) (*http.Response, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal body: %w", err)
	}

	req, err := http.NewRequest(method, c.BaseURL+path, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	return resp, nil
}
