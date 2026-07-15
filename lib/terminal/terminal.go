// Package terminal detects whether the process has an interactive terminal and
// can relaunch the process in one when supported by the current platform.
package terminal

import (
	"os"

	"golang.org/x/term"
)

const relaunchedEnv = "BTG_TERMINAL_RELAUNCHED"

// Attached reports whether at least one of the standard streams is connected
// to an interactive terminal.
func Attached() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) ||
		term.IsTerminal(int(os.Stdout.Fd())) ||
		term.IsTerminal(int(os.Stderr.Fd()))
}

// Ensure relaunches the current process inside a terminal when it was started
// without one and the current platform supports doing so. The returned bool is
// true when the caller must exit because a replacement process was started.
func Ensure() (bool, error) {
	if Attached() || os.Getenv(relaunchedEnv) == "1" {
		return false, nil
	}

	return relaunch()
}

func relaunchEnvironment() []string {
	return append(os.Environ(), relaunchedEnv+"=1")
}
