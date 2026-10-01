package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"lectern/core"
	"lectern/tui"

	"golang.org/x/term"
)

type CLI struct {
	svc *core.Service
}

func (c *CLI) Run(args []string) {
	if len(args) == 0 {
		// A bare `lectern` opens the menu in a real terminal; when piped or
		// scripted there's no one to drive it, so show help instead.
		if term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd())) {
			args = []string{"ez"}
		} else {
			args = []string{"help"}
		}
	}

	os.Exit(c.dispatch(args))
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
		if err := tui.Run(c.svc); err != nil {
			fmt.Fprintln(os.Stderr, "lectern:", err)
			return 1
		}
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

	if err := c.svc.Connect(username, password); err != nil {
		fmt.Println("ERROR:", err)
		return
	}
	fmt.Println("Connected.")
}

func (c *CLI) requireConnected() bool {
	if !c.svc.IsConnected() {
		fmt.Println("ERROR:", core.ErrNotConnected)
		return false
	}
	return true
}

func (c *CLI) handleClone(args []string) {
	path := ""
	if len(args) > 0 {
		path = args[0]
	}

	targetDir, err := core.ResolveCloneTarget(path)
	if err != nil {
		fmt.Printf("Error %v\n", err)
		return
	}

	fmt.Println("Pulling courses from the moodle...")
	courses, err := c.svc.FetchCourses()
	if err != nil {
		fmt.Printf("Error %v\n", err)
		return
	}

	err = c.svc.CloneCourses(targetDir, courses, func(e core.CloneEvent) {
		switch e.Kind {
		case core.CloneCourseCreated:
			fmt.Printf("course created %s\n", e.Course)
		case core.CloneSectionCreated:
			fmt.Printf("└── %s\n", e.Section)
		case core.CloneFileDownloaded:
			fmt.Printf("    - %s\n", e.File)
		}
	})
	if err != nil {
		fmt.Printf("Error %v\n", err)
		return
	}

	fmt.Println("Cloning finished successfully!")
}

func (c *CLI) handleStatus() {
	currentDir, err := os.Getwd()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	report, err := c.svc.LoadStatus(currentDir)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	state := report.State
	fmt.Printf("%-12s%s\n", "Workspace", report.Root)
	fmt.Printf("%-12s%s (%s)\n", "Last sync", state.LastSync.Format("2006-01-02 15:04"), humanAgo(state.LastSync))
	fmt.Printf("%-12s%d across %d courses\n", "Files", report.TotalFiles, len(report.Courses))

	for _, course := range report.Courses {
		fmt.Println()

		title := course.DisplayName
		if course.ShortName != "" {
			title = fmt.Sprintf("%s — %s", course.ShortName, course.DisplayName)
		}
		fmt.Println(title)

		fmt.Printf("  %-11s%s\n", "location", course.Path)
		fmt.Printf("  %-11s%d\n", "sections", course.Sections)
		fmt.Printf("  %-11s%d\n", "files", len(course.Files))
		printFileColumns(course.Files, 2)
	}
}

func humanAgo(t time.Time) string {
	d := time.Since(t)

	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

func printFileColumns(names []string, cols int) {
	if len(names) == 0 {
		return
	}

	maxLen := 0
	for _, n := range names {
		if len(n) > maxLen {
			maxLen = len(n)
		}
	}
	colWidth := maxLen + 3

	for i := 0; i < len(names); i += cols {
		fmt.Print("    ")
		for j := i; j < i+cols && j < len(names); j++ {
			if j == len(names)-1 || j == i+cols-1 {
				fmt.Print(names[j])
			} else {
				fmt.Printf("%-*s", colWidth, names[j])
			}
		}
		fmt.Println()
	}
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
	status, err := core.CheckConfig()
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
	if err := c.svc.ClearConfig(); err != nil {
		fmt.Printf("Error %v\n", err)
		return
	}
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

func (c *CLI) printHelp() {
	fmt.Println("usage: lectern <command> [<args>]")
	fmt.Println()
	fmt.Println("Available commands:")
	fmt.Println("  connect      - Connect user to CLI")
	fmt.Println("  clone        - Clone all user's courses")
	fmt.Println("  status       - check status of the local workspace")
	fmt.Println("  config check - print what's saved in the local config file")
	fmt.Println("  config clear - clear the local config file")
	fmt.Println("  ez           - Open the interactive menu (also what plain 'lectern' does)")
	fmt.Println("  help         - Show this help")
	fmt.Println("  update       - Install/Update lectern")
}
