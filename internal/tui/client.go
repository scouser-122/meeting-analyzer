package tui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/scouser-122/meeting-analyzer/internal/models"
)

// TuiClient is an HTTP client for the backend API used by the TUI application.
type TuiClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewClient creates a TuiClient with the given backend base URL.
func NewClient(baseURL string) *TuiClient {
	return &TuiClient{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// Start registers a new user via the backend API.
func (c *TuiClient) Start(userID string) (*models.CommonResponse, error) {
	body := map[string]string{"id": userID}
	resp, err := c.doJSON("POST", "/api/users/start", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
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

// Load uploads an audio file and metadata to the backend for processing.
func (c *TuiClient) Load(userID, meetingName, filePath string) (*models.LoadResponse, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	filePart, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return nil, fmt.Errorf("create form file: %w", err)
	}
	if _, err = io.Copy(filePart, file); err != nil {
		return nil, fmt.Errorf("copy file: %w", err)
	}

	metadata := map[string]string{
		"user_id": userID,
		"name":    meetingName,
	}
	metadataJSON, _ := json.Marshal(metadata)
	writer.WriteField("metadata", string(metadataJSON))

	writer.Close()

	req, err := http.NewRequest("POST", c.BaseURL+"/api/meetings/load", &buf)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

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
func (c *TuiClient) List(userID string) ([]models.MeetingResponseData, error) {
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
func (c *TuiClient) Status(userID, meetingID string) (*models.MeetingResponseData, error) {
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
func (c *TuiClient) Transcription(userID, meetingID string) (*models.TranscriptionResponseData, error) {
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

// Delete removes the specified meeting through the backend API.
func (c *TuiClient) Delete(userID, meetingID string) error {
	url := fmt.Sprintf("%s/api/meetings/delete?user_id=%s&meeting_id=%s", c.BaseURL, userID, meetingID)
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp models.CommonResponse
		json.NewDecoder(resp.Body).Decode(&errResp)
		return fmt.Errorf("%s", errResp.Message)
	}
	return nil
}

// Find searches meetings by keywords through the backend API.
func (c *TuiClient) Find(userID, keywords string) ([]models.MeetingResponseData, error) {
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
func (c *TuiClient) Chat(userID, question string) (*models.ChatResponse, error) {
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

func (c *TuiClient) doJSON(method, path string, body interface{}) (*http.Response, error) {
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
