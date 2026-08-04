package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/scouser-122/meeting-analyzer/internal/client/gigachat"
	"github.com/scouser-122/meeting-analyzer/internal/service"
)

var jwtService *service.JwtService

func handleGetToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	log.Println("received request via /api/v2/oauth")
	jwt, err := jwtService.GenerateJWT("salute")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	if err := enc.Encode(gigachat.GigaAccessToken{
		Token:     jwt,
		ExpiresAt: time.Now().Add(3 * time.Hour).Unix(),
	}); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(buf.Bytes())
}

func handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	log.Println("received request via /v1/chat/completions")
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	if err := enc.Encode(gigachat.GigaChatCompletionsResponse{
		Model: "GigaChat",
		Choises: []gigachat.GigaChatCompletionsResponseChoise{
			{
				Message: gigachat.GigaChatCompletionsMessage{
					Role:    "user",
					Content: "User request",
				},
			},
			{
				Message: gigachat.GigaChatCompletionsMessage{
					Role:    "assistant",
					Content: "На встрече обсуждали добавление нового параметра priority в метод создания заказа. Договорились реализовать и протестировать в этот же день.",
				},
			},
		},
	}); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(buf.Bytes())
}

func main() {
	jwtService = service.NewJwtService(3, "defaultSecretKey")

	http.HandleFunc("/api/v2/oauth", handleGetToken)
	http.HandleFunc("/v1/chat/completions", handleChatCompletions)

	log.Println("Server starting on :45059")
	if err := http.ListenAndServe(":45059", nil); err != nil {
		log.Fatal(err)
	}
}
