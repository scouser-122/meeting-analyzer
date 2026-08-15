package gigachat

type GigaAccessToken struct {
	Token     string `json:"access_token"`
	ExpiresAt int64  `json:"expires_at"`
}

type GigaChatCompletionsMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type GigaChatCompletionsRequest struct {
	Model    string                       `json:"model"`
	Messages []GigaChatCompletionsMessage `json:"messages"`
}

type GigaChatCompletionsResponseChoice struct {
	Message GigaChatCompletionsMessage `json:"message"`
}

type GigaChatCompletionsResponse struct {
	Model   string                              `json:"model"`
	Choices []GigaChatCompletionsResponseChoice `json:"choices"`
}
