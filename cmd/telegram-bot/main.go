package main

import (
	"flag"
	"log"
	"os"

	"github.com/scouser-122/meeting-analyzer/internal/telegram"
)

func main() {
	configFilePath := new(string)
	flag.StringVar(configFilePath, "config", "config.yaml", "config file path")
	flag.StringVar(configFilePath, "c", "config.yaml", "config file path")
	flag.Parse()
	envConfigFile := os.Getenv("CONFIG")
	if envConfigFile != "" {
		*configFilePath = envConfigFile
	}

	cfg, err := telegram.LoadConfig(*configFilePath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	bot := telegram.NewTelegramBot(cfg)
	if err := bot.Init(); err != nil {
		log.Fatalf("failed init telegram bot: %w", err)
	}
	bot.Run()
}
