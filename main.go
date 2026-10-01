package main

import (
	"fmt"
	"os"

	"lectern/core"
)

func main() {
	baseURL := os.Getenv("MOODLE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	cfg, err := core.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load config: %v\n", err)
		cfg = &core.GlobalConfig{}
	}

	if cfg.BaseURL != "" {
		baseURL = cfg.BaseURL
	}

	client := core.New(baseURL)
	if cfg.Token != "" {
		client.RestoreSession(cfg.Token, &core.User{
			ID:       cfg.UserID,
			Username: cfg.Username,
			Email:    cfg.Email,
		})
	}

	cli := &CLI{svc: core.NewService(client)}
	cli.Run(os.Args[1:])
}
