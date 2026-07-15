//go:build darwin

package terminal

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func relaunch() (bool, error) {
	executable, err := os.Executable()
	if err != nil {
		return false, fmt.Errorf("resolve executable: %w", err)
	}

	workingDirectory, err := os.Getwd()
	if err != nil {
		return false, fmt.Errorf("resolve working directory: %w", err)
	}

	parts := []string{
		"cd " + shellQuote(workingDirectory),
		relaunchedEnv + "=1 " + shellQuote(executable),
	}
	for _, arg := range os.Args[1:] {
		parts[1] += " " + shellQuote(arg)
	}
	shellCommand := strings.Join(parts, " && ")
	appleScript := "tell application \"Terminal\" to do script " + appleScriptQuote(shellCommand)

	cmd := exec.Command("osascript",
		"-e", appleScript,
		"-e", "tell application \"Terminal\" to activate",
	)
	if output, runErr := cmd.CombinedOutput(); runErr != nil {
		return false, fmt.Errorf("open Terminal: %w: %s", runErr, strings.TrimSpace(string(output)))
	}
	return true, nil
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func appleScriptQuote(value string) string {
	replacer := strings.NewReplacer(
		"\\", "\\\\",
		"\"", "\\\"",
		"\r", "\\r",
		"\n", "\\n",
	)
	return "\"" + replacer.Replace(value) + "\""
}
