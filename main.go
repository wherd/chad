package main

import (
	"os"

	"github.com/charmbracelet/log"
	"wherd.dev/chad/internal/bot"
	"wherd.dev/chad/internal/config"
)

func main() {
	// Get config file path from args
	configPath := ".chad"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	config, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatal(err)
	}

	// Create bor instance
	b := bot.New(config)
	if err := b.Run(); err != nil {
		log.Fatal(err)
	}
}
