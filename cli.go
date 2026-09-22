package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/term"
)

type CLI struct {
	client    *MoodleClient
	workspace *WorkspaceManager
}

func (c *CLI) Run(args []string) {
	if len(args) == 0 {
		c.RunInteractive()
		return
	}

	os.Exit(c.dispatch(args))
}

func (c *CLI) RunInteractive() {
	fmt.Println("lectern is running")
	fmt.Println("write help to see commands")

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			return
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		args := strings.Fields(line)
		if args[0] == "exit" || args[0] == "quit" {
			return
		}

		c.dispatch(args)
	}
}

func (c *CLI) dispatch(args []string) int {
	command := args[0]

	switch command {
	case "connect":
		c.handleConnect()
	case "clone":
		if !c.requireConnected() {
			return 1
		}

		c.handleClone(args[1:])
	case "status":
		if !c.requireConnected() {
			return 1
		}

		c.handleStatus()
	case "config":
		c.handleConfig(args[1:])
	case "update":
		c.handleShellUpdate()
	case "ez":
		c.handleEZ()
	case "help", "--help", "-h":
		c.printHelp()
	default:
		fmt.Fprintf(os.Stderr, "lectern: '%s' is not a lectern command. See 'lectern help'.\n", command)
		return 1
	}

	return 0
}

func (c *CLI) handleConnect() {
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Print("Username: ")
	if !scanner.Scan() {
		fmt.Println("ERROR: failed to read username")
		return
	}
	username := strings.TrimSpace(scanner.Text())

	fmt.Print("Password: ")
	passwordBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		fmt.Println("ERROR: failed to read password")
		return
	}
	password := strings.TrimSpace(string(passwordBytes))

	if err := c.client.Login(username, password); err != nil {
		fmt.Println("ERROR: failed to connect:", err)
		return
	}

	cfg := &GlobalConfig{
		BaseURL:  c.client.Host(),
		Email:    c.client.user.Email,
		Username: c.client.user.Username,
		UserID:   c.client.user.ID,
		Token:    c.client.token,
	}

	if err := SaveConfig(cfg); err != nil {
		fmt.Println("ERROR: failed to save config:", err)
		return
	}
	fmt.Println("Connected.")
}

func (c *CLI) requireConnected() bool {
	if c.client == nil || c.client.token == "" {
		fmt.Println("ERROR: not connected. Run 'connect' first.")
		return false
	}
	return true
}

func (c *CLI) handleClone(args []string) {
	currentDIr, err := os.Getwd()

	if err != nil {
		fmt.Printf("Error recognizing current folder: %v\n", err)
		return
	}

	targetDir := currentDIr

	if len(args) > 0 {
		customPath := args[0]
		if filepath.IsAbs(customPath) {
			targetDir = customPath
		} else {
			targetDir = filepath.Join(currentDIr, customPath)
		}
	}

	fmt.Println("Pulling courses from the moodle...")
	courses, err := c.client.GetUserCourses()
	if err != nil {
		fmt.Printf("Error fetching courses: %v\n", err)
		return
	}

	if err := c.workspace.InitWorkspace(targetDir, courses, c.client); err != nil {
		fmt.Printf("Error setting up workspace: %v\n", err)
		return
	}

	fmt.Println("CLoning finished successfully!!!")
}

func (c *CLI) handleStatus() {
	currentDir, err := os.Getwd()

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	root, err := c.workspace.FindRoot(currentDir)

	if err != nil {
		fmt.Printf("Couldn't find root: %v\n", err)
		return
	}

	state, err := c.workspace.LoadState(root)
	if err != nil {
		fmt.Printf("Error loading sync file: %v\n", err)
		return
	}

	fmt.Printf("Workspace location: %s\n", root)
	fmt.Printf("Last sync: %s\n", state.LastSync.Format("2006-01-02 15:04:05"))
	fmt.Printf("Number of files: %+v\n", len(state.Files))
}

func (c *CLI) handleConfig(args []string) {
	if len(args) == 0 {
		fmt.Println("usage: lectern config <check|clear>")
		return
	}

	switch args[0] {
	case "check":
		c.handleConfigCheck()
	case "clear":
		c.handleConfigClear()
	default:
		fmt.Fprintf(os.Stderr, "lectern: 'config %s' is not a lectern command. See 'lectern help'.\n", args[0])
	}
}

func (c *CLI) handleConfigCheck() {
	status, err := CheckConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		return
	}

	fmt.Printf("Config file: %s\n", status.Path)

	if !status.Connected {
		fmt.Println("Not connected.")
		return
	}

	fmt.Printf("Base URL: %s\n", status.BaseURL)
	fmt.Printf("Username: %s\n", status.Username)
	fmt.Printf("Email:    %s\n", status.Email)
	fmt.Printf("User ID:  %d\n", status.UserID)
	fmt.Println("Token:    ********")
}

func (c *CLI) handleConfigClear() {
	if err := ClearConfig(); err != nil {
		fmt.Printf("Error clearing config: %v\n", err)
		return
	}

	c.client.RestoreSession("", nil)
	fmt.Println("Config cleared.")
}

func (c *CLI) handleShellUpdate() {
	cmd := exec.Command("bash", "-c", "curl -fsSL https://raw.githubusercontent.com/OmriNR/lectern/main/install.sh | bash")

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("Failed updating: %+v\n", err)
	}
}

func (c *CLI) handleEZ() {
	if err := runEZ(c); err != nil {
		fmt.Printf("Error running easy menu: %v\n", err)
	}
}

func (c *CLI) printHelp() {
	fmt.Println("usage: lectern <command> [<args>]")
	fmt.Println()
	fmt.Println("Available commands:")
	fmt.Println("  connect      - Connect user to CLI")
	fmt.Println("  clone        - Clone all user's courses")
	fmt.Println("  status       - check status of the local workspace")
	fmt.Println("  config check - print what's saved in the local config file")
	fmt.Println("  config clear - clear the local config file")
	fmt.Println("  ez           - opens the easy menu (for users that are not used to terminal commands)")
	fmt.Println("  help         - Show this help")
	fmt.Println("  update       - Install/Update lectern")
	fmt.Println("  exit         - Exit lectern (interactive mode only)")
}
