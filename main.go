package main

import (
	"os"
)

func main() {
	baseURL := os.Getenv("MOODLE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	cli := &CLI{
		client:    New(baseURL),
		workspace: NewWorkspaceManager(),
	}
	cli.Run(os.Args[1:])
}
