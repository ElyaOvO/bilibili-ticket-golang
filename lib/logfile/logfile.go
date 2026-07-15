// Package logfile opens application log files and archives the previous log
// when the application starts.
package logfile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const archiveTimeLayout = "2006-02-01 15-04-05"

// OpenRotating moves an existing log into an "old" directory next to it and
// opens a fresh log file. Archived contains the destination path when a file
// was moved, or is empty when there was no previous log.
func OpenRotating(path string) (file *os.File, archived string, err error) {
	return openRotatingAt(path, time.Now())
}

func openRotatingAt(path string, now time.Time) (*os.File, string, error) {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return nil, "", fmt.Errorf("create log directory: %w", err)
	}

	archived, err := archiveExisting(path, directory, now)
	if err != nil {
		return nil, "", err
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, archived, fmt.Errorf("open log file: %w", err)
	}
	return file, archived, nil
}

func archiveExisting(path, directory string, now time.Time) (string, error) {
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("inspect previous log: %w", err)
	}

	archiveDirectory := filepath.Join(directory, "old")
	if err := os.MkdirAll(archiveDirectory, 0o755); err != nil {
		return "", fmt.Errorf("create log archive directory: %w", err)
	}

	stem := now.Format(archiveTimeLayout)
	destination := filepath.Join(archiveDirectory, stem+".log")
	for sequence := 2; ; sequence++ {
		_, err := os.Stat(destination)
		if errors.Is(err, os.ErrNotExist) {
			break
		}
		if err != nil {
			return "", fmt.Errorf("inspect log archive destination: %w", err)
		}
		destination = filepath.Join(archiveDirectory, fmt.Sprintf("%s-%d.log", stem, sequence))
	}

	if err := os.Rename(path, destination); err != nil {
		return "", fmt.Errorf("archive previous log: %w", err)
	}
	return destination, nil
}
