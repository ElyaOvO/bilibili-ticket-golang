//go:build linux

package terminal

import (
	"fmt"
	"os"
	"os/exec"
)

type emulator struct {
	name string
	args []string
}

func relaunch() (bool, error) {
	executable, err := os.Executable()
	if err != nil {
		return false, fmt.Errorf("resolve executable: %w", err)
	}

	emulators := []emulator{
		{name: "x-terminal-emulator", args: []string{"-e"}},
		{name: "gnome-terminal", args: []string{"--"}},
		{name: "konsole", args: []string{"-e"}},
		{name: "xfce4-terminal", args: []string{"-x"}},
		{name: "xterm", args: []string{"-e"}},
	}

	for _, candidate := range emulators {
		path, lookupErr := exec.LookPath(candidate.name)
		if lookupErr != nil {
			continue
		}

		args := append([]string{}, candidate.args...)
		args = append(args, executable)
		args = append(args, os.Args[1:]...)
		cmd := exec.Command(path, args...)
		cmd.Env = relaunchEnvironment()
		cmd.Dir, _ = os.Getwd()
		if err = cmd.Start(); err != nil {
			continue
		}
		_ = cmd.Process.Release()
		return true, nil
	}

	return false, fmt.Errorf("no supported terminal emulator found")
}
