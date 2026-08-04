package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/scouser-122/meeting-analyzer/internal/client/salutespeech"
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
	if err := enc.Encode(salutespeech.SaluteAuthToken{
		Token:     jwt,
		ExpiresAt: time.Now().Add(3 * time.Hour).Unix(),
	}); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(buf.Bytes())
}

func handleUploadFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	log.Println("received request via /rest/v1/data:upload")
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	if err := enc.Encode(salutespeech.SaluteSpeechUploadResponse{
		Status: 200,
		Result: salutespeech.SaluteSpeechUploadResult{
			RequestFileId: uuid.New().String(),
		},
	}); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(buf.Bytes())
}

func handleCreateRecognizeTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	log.Println("received request via /rest/v1/speech:async_recognize")
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	if err := enc.Encode(salutespeech.SaluteSpeechRecognizeResponse{
		Status: 200,
		Result: salutespeech.SaluteSpeechRecognizeResult{
			ID:     uuid.New().String(),
			Status: "NEW",
		},
	}); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(buf.Bytes())
}

var statusRandCounter int

func handleGetRecognizeStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	log.Println("received request via /rest/v1/task:get")

	var status string
	statusRandCounter++
	if statusRandCounter == 1 {
		status = "PROCESSING"
	} else {
		status = "DONE"
		statusRandCounter = 0
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	log.Println("status %", status)
	w.Write([]byte(status))
}

func handleGetFileWithResult(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	log.Println("received request via /rest/v1/task:get")

	content, err := os.ReadFile("./transcription.txt")
	if err != nil {
		log.Fatalf("Failed to read file: %s", err)
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	if err := enc.Encode(salutespeech.SaluteSpeechRecognizedText{
		Text: string(content),
	}); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(buf.Bytes())
}

func main() {
	jwtService = service.NewJwtService(3, "defaultSecretKey")
	statusRandCounter = 0

	http.HandleFunc("/api/v2/oauth", handleGetToken)
	http.HandleFunc("/rest/v1/data:upload", handleUploadFile)
	http.HandleFunc("/rest/v1/speech:async_recognize", handleCreateRecognizeTask)
	http.HandleFunc("/rest/v1/task:get", handleGetRecognizeStatus)
	http.HandleFunc("/rest/v1/data:download", handleGetFileWithResult)

	log.Println("Server starting on :45058")
	if err := http.ListenAndServe(":45058", nil); err != nil {
		log.Fatal(err)
	}
}
