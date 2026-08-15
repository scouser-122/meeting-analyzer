package gigachat

// GigaAccessToken represents an OAuth access token returned by GigaChat.
type GigaAccessToken struct {
	Token     string `json:"access_token"`
	ExpiresAt int64  `json:"expires_at"`
}

// GigaChatCompletionsMessage represents a single message in a GigaChat completions request.
type GigaChatCompletionsMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// GigaChatCompletionsRequest is the request body for GigaChat chat completions.
type GigaChatCompletionsRequest struct {
	Model    string                       `json:"model"`
	Messages []GigaChatCompletionsMessage `json:"messages"`
}

// GigaChatCompletionsResponseChoice represents one completion choice returned by GigaChat.
type GigaChatCompletionsResponseChoice struct {
	Message GigaChatCompletionsMessage `json:"message"`
}

// GigaChatCompletionsResponse is the response body from GigaChat chat completions.
type GigaChatCompletionsResponse struct {
	Model   string                              `json:"model"`
	Choices []GigaChatCompletionsResponseChoice `json:"choices"`
}
