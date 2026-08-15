package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/scouser-122/meeting-analyzer/internal/tui"
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

	cfg, err := tui.LoadConfig(*configFilePath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	client := tui.NewClient(cfg.BaseURL)

	session, err := tui.LoadSession()
	if err != nil {
		log.Fatalf("Failed to load session: %v", err)
	}

	var userID string

	if session != nil {
		userID = session.UserID
	} else {
		userID = tui.NewUserID()

		resp, err := client.Start(userID)
		if err != nil {
			log.Fatalf("failed to register user on server: %v\n", err)
		} else {
			fmt.Printf("New user registered: %s\n", resp.Message)
		}

		if err := tui.SaveSession(userID); err != nil {
			log.Fatalf("Failed to save session: %v", err)
		}
	}

	m := tui.NewModel(client, userID)

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Fatalf("Error running TUI: %v\n", err)
	}
}
