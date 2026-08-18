package salutespeech

// SaluteAuthToken represents an OAuth access token returned by SaluteSpeech.
type SaluteAuthToken struct {
	Token     string `json:"Токен доступа"`
	ExpiresAt int64  `json:"expires_at"`
}

// SaluteSpeechUploadResult holds the file identifier returned after uploading audio.
type SaluteSpeechUploadResult struct {
	RequestFileId string `json:"request_file_id"`
}

// SaluteSpeechUploadResponse is the response body from the SaluteSpeech upload endpoint.
type SaluteSpeechUploadResponse struct {
	Status int                      `json:"status"`
	Result SaluteSpeechUploadResult `json:"result"`
}

// SaluteSpeechRecognizeRequestOptions contains options for an asynchronous recognition request.
type SaluteSpeechRecognizeRequestOptions struct {
	AudioEncoding string `json:"audio_encoding"`
}

// SaluteSpeechRecognizeRequest is the request body to start asynchronous speech recognition.
type SaluteSpeechRecognizeRequest struct {
	Options SaluteSpeechRecognizeRequestOptions `json:"options"`
}

// SaluteSpeechRecognizeResult holds the task identifier and status for recognition.
type SaluteSpeechRecognizeResult struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// SaluteSpeechRecognizeResponse is the response body from the recognition start endpoint.
type SaluteSpeechRecognizeResponse struct {
	Status int                         `json:"status"`
	Result SaluteSpeechRecognizeResult `json:"result"`
}

// SaluteSpeechRecognizedText is the final recognized text returned by SaluteSpeech.
type SaluteSpeechRecognizedText struct {
	Text string `json:"text"`
}
