package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/alchemy/rotoslog"
	"github.com/scouser-122/meeting-analyzer/internal/client/gigachat"
	"github.com/scouser-122/meeting-analyzer/internal/client/salutespeech"
	"github.com/scouser-122/meeting-analyzer/internal/config"
	"github.com/scouser-122/meeting-analyzer/internal/logger"
	"github.com/scouser-122/meeting-analyzer/internal/repository/postgres"
	"github.com/scouser-122/meeting-analyzer/internal/server"
	"github.com/scouser-122/meeting-analyzer/internal/service"
	"github.com/scouser-122/meeting-analyzer/internal/worker"
)

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
	logger.Initialize(*serverConfig.LogLevel, fileHandler)

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

	handlers := initServicesAndGetHandlers(database, &serverConfig)

	server := server.NewServer(&serverConfig)
	if err := server.Init(handlers); err != nil {
		slog.Error("cannot init server", "err", err)
		panic(err)
	}
	go func() {
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error: %v", err)
			panic(err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)
	<-quit

	server.Shutdown()
	slog.Info("server gracefully stopped")
}

func initServicesAndGetHandlers(database postgres.PostgresDatabase, serverConfig *config.ServerConfig) []server.Handler {
	repositoryUtils := postgres.NewPostgresRepositoryUtils(&database)

	usersRepo := postgres.NewPostgresUserRepository(&database)
	usersService := service.NewUsersService(usersRepo)

	meetingsRepo := postgres.NewPostgresMeetingRepository(&database)
	meetingsService := service.NewMeetingsService(meetingsRepo, usersRepo, repositoryUtils, serverConfig)

	tasksRepo := postgres.NewPostgresTaskRepository(&database)
	tasksService := service.NewTasksService(tasksRepo, repositoryUtils)

	transcriptionsRepo := postgres.NewPostgresTranscriptionRepository(&database)
	transcriptionsService := service.NewTranscriptionService(transcriptionsRepo, repositoryUtils)

	summaryRepo := postgres.NewPostgresSummaryRepository(&database)
	summaryService := service.NewSummaryService(summaryRepo, repositoryUtils)

	audioProcessor := salutespeech.NewSaluteSpeechClient(serverConfig)
	summaryProcessor := gigachat.NewGigaChatClient(serverConfig)

	meetingProcessor := worker.NewMeetingProcessor(
		meetingsService,
		tasksService,
		transcriptionsService,
		summaryService,
		audioProcessor,
		summaryProcessor,
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
	)
}
