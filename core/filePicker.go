package core

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

var ErrPickCanceled = errors.New("no file chosen")

const pickTitle = "Choose a file to upload"

func PickFile(startDir string) (string, error) {
	if startDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		startDir = home
	}

	cmd, err := pickerCommand(startDir)
	if err != nil {
		return "", err
	}

	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return "", ErrPickCanceled
		}
		return "", fmt.Errorf("file chooser: %w", err)
	}

	path := strings.TrimSpace(string(out))
	if path == "" {
		return "", ErrPickCanceled
	}

	return path, nil
}

func pickerCommand(startDir string) (*exec.Cmd, error) {
	switch runtime.GOOS {
	case "darwin":
		script := fmt.Sprintf(
			"POSIX path of (choose file with prompt %s default location POSIX file %s)",
			appleScriptString(pickTitle), appleScriptString(startDir),
		)
		return exec.Command("osascript", "-e", script), nil

	case "windows":
		script := "[Console]::OutputEncoding = [Text.Encoding]::UTF8\n" +
			"Add-Type -AssemblyName System.Windows.Forms\n" +
			"$d = New-Object System.Windows.Forms.OpenFileDialog\n" +
			"$d.Title = " + powerShellString(pickTitle) + "\n" +
			"$d.InitialDirectory = " + powerShellString(startDir) + "\n" +
			"if ($d.ShowDialog() -eq 'OK') { $d.FileName }"
		return exec.Command("powershell", "-NoProfile", "-STA", "-Command", script), nil

	default:
		if zenity, err := exec.LookPath("zenity"); err == nil {
			return exec.Command(zenity, "--file-selection",
				"--title="+pickTitle,
				"--filename="+strings.TrimRight(startDir, "/")+"/",
			), nil
		}
		if kdialog, err := exec.LookPath("kdialog"); err == nil {
			return exec.Command(kdialog, "--title", pickTitle, "--getopenfilename", startDir), nil
		}
		return nil, errors.New("no file chooser found: install zenity or kdialog")
	}
}

func appleScriptString(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}

func powerShellString(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
