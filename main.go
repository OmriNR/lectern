package main

import (
	"fmt"
	"os"
)

func main() {
	baseURL := os.Getenv("MOODLE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	cfg, err := LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load config: %v\n", err)
		cfg = &GlobalConfig{}
	}

	if cfg.BaseURL != "" {
		baseURL = cfg.BaseURL
	}

	client := New(baseURL)
	if cfg.Token != "" {
		client.RestoreSession(cfg.Token, &User{
			ID:       cfg.UserID,
			Username: cfg.Username,
			Email:    cfg.Email,
		})
	}

	cli := &CLI{
		client:    client,
		workspace: NewWorkspaceManager(),
	}
	cli.Run(os.Args[1:])
}
