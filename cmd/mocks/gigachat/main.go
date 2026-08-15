package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"math/rand/v2"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/scouser-122/meeting-analyzer/internal/client/gigachat"
	"github.com/scouser-122/meeting-analyzer/internal/models"
	"github.com/scouser-122/meeting-analyzer/internal/service"
)

var jwtService *service.JwtService

func handleGetToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
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

	bodyBuf, err := io.ReadAll(r.Body)
	if err != nil {
		log.Println("cannot read request body", "err", err)
		w.WriteHeader(http.StatusBadRequest)
		w.Write(models.NewErrorResponseBuffer(models.UnexpectedErrorMessage))
		return
	}

	var request gigachat.GigaChatCompletionsRequest
	if err := json.Unmarshal(bodyBuf, &request); err != nil {
		log.Println("cannot decode request json body", "err", err)
		w.WriteHeader(http.StatusBadRequest)
		w.Write(models.NewErrorResponseBuffer(models.UnexpectedErrorMessage))
		return
	}

	log.Println("received request via /v1/chat/completions")

	time.Sleep(time.Duration(1000+rand.IntN(500)) * time.Millisecond)

	var question string
	var choises []gigachat.GigaChatCompletionsResponseChoice
	for _, m := range request.Messages {
		if m.Role == "user" {
			if strings.Index(m.Content, "Напиши краткую выжимку") == 0 {
				question = "summary"
			} else if strings.Index(m.Content, "Определи, спрашивает ли пользователь про какую-то встречу") == 0 {
				question = "intent_extract"
			}
		}
		choises = append(choises, gigachat.GigaChatCompletionsResponseChoice{
			Message: gigachat.GigaChatCompletionsMessage{
				Role:    m.Role,
				Content: m.Content,
			},
		})
	}

	if question == "summary" {
		choises = append(choises, gigachat.GigaChatCompletionsResponseChoice{
			Message: gigachat.GigaChatCompletionsMessage{
				Role:    "assistant",
				Content: "На встрече обсуждали добавление нового параметра priority в метод создания заказа. Договорились реализовать и протестировать функционал в этот же день.",
			},
		})
	} else if question == "intent_extract" {
		choises = append(choises, gigachat.GigaChatCompletionsResponseChoice{
			Message: gigachat.GigaChatCompletionsMessage{
				Role:    "assistant",
				Content: `{"is_meeting_query": true, "keywords": ["добавить", "новый", "параметр", "API", "срочно"], "topic": "Добавление нового параметра в API"}`,
			},
		})
	}

	slices.Reverse(choises)

	log.Println("choises len: ", len(choises))

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	if err := enc.Encode(gigachat.GigaChatCompletionsResponse{
		Model:   "GigaChat",
		Choices: choises,
	}); err != nil {
		log.Println("error process request /v1/chat/completions: ", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(buf.Bytes())

	log.Println("success process request /v1/chat/completions")
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
