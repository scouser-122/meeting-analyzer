package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/alchemy/rotoslog"
	"github.com/scouser-122/meeting-analyzer/internal/client"
	"github.com/scouser-122/meeting-analyzer/internal/client/gigachat"
	"github.com/scouser-122/meeting-analyzer/internal/client/nexara"
	"github.com/scouser-122/meeting-analyzer/internal/client/salutespeech"
	"github.com/scouser-122/meeting-analyzer/internal/config"
	"github.com/scouser-122/meeting-analyzer/internal/logger"
	"github.com/scouser-122/meeting-analyzer/internal/repository/postgres"
	"github.com/scouser-122/meeting-analyzer/internal/server"
	"github.com/scouser-122/meeting-analyzer/internal/service"
	"github.com/scouser-122/meeting-analyzer/internal/storage"
	"github.com/scouser-122/meeting-analyzer/internal/storage/filesystem"
	miniostorage "github.com/scouser-122/meeting-analyzer/internal/storage/minio"
	"github.com/scouser-122/meeting-analyzer/internal/worker"
)

var meetingProcessor *worker.MeetingProcessor

func main() {
	serverConfig := config.DefaultServerConfig()
	serverConfig.Parse()

	var fileHandler *rotoslog.Handler
	if *serverConfig.Environment == "prod" {
		var err error
		fileHandler, err = rotoslog.NewHandler(
			rotoslog.FilePrefix("metting-analyzer-"),
			rotoslog.MaxFileSize(32*1024*1024),
			rotoslog.MaxRotatedFiles(3),
		)
		if err != nil {
			panic(err)
		}
		defer fileHandler.Close()
	}
	logger.Initialize(*serverConfig.LogLevel, *serverConfig.Environment, fileHandler)

	database := postgres.NewPostgresDB(serverConfig)
	if err := database.Open(); err != nil {
		slog.Error("cannot connect to database", "err", err)
		panic(err)
	}
	if err := database.Ping(context.Background()); err != nil {
		slog.Error("cannot ping database", "err", err)
		panic(err)
	}
	defer database.Close()

	handlers, err := initServicesAndGetHandlers(database, &serverConfig)
	if err != nil {
		slog.Error("cannot init handlers", "err", err)
		panic(err)
	}

	server := server.NewServer(&serverConfig)
	if err := server.Init(handlers); err != nil {
		slog.Error("cannot init server", "err", err)
		panic(err)
	}
	go func() {
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
			panic(err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)
	<-quit

	server.Shutdown()
	meetingProcessor.Shutdown()
}

func initServicesAndGetHandlers(database postgres.PostgresDatabase, serverConfig *config.ServerConfig) ([]server.Handler, error) {
	repositoryUtils := postgres.NewPostgresRepositoryUtils(&database)

	usersRepo := postgres.NewPostgresUserRepository(&database)
	usersService := service.NewUsersService(usersRepo)

	tasksRepo := postgres.NewPostgresTaskRepository(&database)
	tasksService := service.NewTasksService(tasksRepo, repositoryUtils)

	transcriptionsRepo := postgres.NewPostgresTranscriptionRepository(&database)
	transcriptionsService := service.NewTranscriptionService(transcriptionsRepo, repositoryUtils)

	summaryRepo := postgres.NewPostgresSummaryRepository(&database)
	summaryService := service.NewSummaryService(summaryRepo, repositoryUtils)

	fileStorage, err := initFileStorage(serverConfig)
	if err != nil {
		return nil, err
	}

	meetingsRepo := postgres.NewPostgresMeetingRepository(&database)
	meetingsService := service.NewMeetingsService(meetingsRepo, repositoryUtils, usersService, tasksService, transcriptionsService, summaryService, serverConfig, fileStorage)

	var audioProcessor client.AudioProcessor
	if *serverConfig.RecognizeService == "salute_speech" {
		audioProcessor = salutespeech.NewSaluteSpeechClient(serverConfig, fileStorage)
	} else {
		audioProcessor = nexara.NewNexaraClient(serverConfig, fileStorage)
	}

	llmClient := gigachat.NewGigaChatClient(serverConfig)

	meetingProcessor = worker.NewMeetingProcessor(
		meetingsService,
		tasksService,
		transcriptionsService,
		summaryService,
		repositoryUtils,
		audioProcessor,
		llmClient,
		serverConfig,
	)
	meetingProcessor.Run()

	return server.InitializeHandlers(
		serverConfig,
		usersService,
		meetingsService,
		tasksService,
		transcriptionsService,
		summaryService,
		meetingProcessor,
		llmClient,
	), nil
}

func initFileStorage(serverConfig *config.ServerConfig) (storage.FileStorage, error) {
	switch *serverConfig.FileStorageType {
	case "minio":
		minioStorage, err := miniostorage.NewStorage(serverConfig.Minio)
		if err != nil {
			slog.Error("cannot create minio storage", "err", err)
			return nil, err
		}
		return minioStorage, nil
	default:
		return filesystem.NewStorage(*serverConfig.UploadFileDir), nil
	}
}
