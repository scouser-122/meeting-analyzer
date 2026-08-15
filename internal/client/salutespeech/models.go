package salutespeech

type SaluteAuthToken struct {
	Token     string `json:"Токен доступа"`
	ExpiresAt int64  `json:"expires_at"`
}

type SaluteSpeechUploadResult struct {
	RequestFileId string `json:"request_file_id"`
}

type SaluteSpeechUploadResponse struct {
	Status int                      `json:"status"`
	Result SaluteSpeechUploadResult `json:"result"`
}

type SaluteSpeechRecognizeRequestOptions struct {
	AudioEncoding string `json:"audio_encoding"`
}

type SaluteSpeechRecognizeRequest struct {
	Options SaluteSpeechRecognizeRequestOptions `json:"options"`
}

type SaluteSpeechRecognizeResult struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

type SaluteSpeechRecognizeResponse struct {
	Status int                         `json:"status"`
	Result SaluteSpeechRecognizeResult `json:"result"`
}

type SaluteSpeechRecognizedText struct {
	Text string `json:"text"`
}
