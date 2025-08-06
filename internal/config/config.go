package config

import (
	"bytes"
	"encoding/json"
	"os"
)

type Config struct {
	DiscordToken     string     `json:"discord_token"`
	SearchApiKey     string     `json:"search_api"`
	Prefix           string     `json:"prefix"`
	AutoSaveInterval int        `json:"auto_save_interval"`
	OpenRouter       OpenRouter `json:"open_router"`
	RateLimit        RateLimit  `json:"rate_limit"`
}

type RateLimit struct {
	MaxRequests int   `json:"max_requests"`
	Window      int64 `json:"window"`
	MuteTime    int64 `json:"mute_time"`
}

type OpenRouter struct {
	Key                  string  `json:"key"`
	SystemPrompt         string  `json:"system_prompt"`
	Temperature          float64 `json:"temperature"`
	MaxTokens            int     `json:"max_tokens"`
	MaxMessagesInContext int     `json:"max_messages_in_context"`
	Model                string  `json:"model"`
}

func LoadConfig(name string) (*Config, error) {
	b, err := os.ReadFile(name)
	if err != nil {
		return nil, err
	}

	config := &Config{
		Prefix:           "!",
		AutoSaveInterval: 60,
		RateLimit: RateLimit{
			MaxRequests: 10,
			Window:      60,
			MuteTime:    60,
		},
	}

	json.NewDecoder(bytes.NewBuffer(b)).Decode(config)
	return config, nil
}
