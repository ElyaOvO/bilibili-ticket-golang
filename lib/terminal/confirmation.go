package terminal

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ConfirmationOptions configures a one-time exact-text confirmation in the
// terminal. Input and Output default to os.Stdin and os.Stdout.
type ConfirmationOptions struct {
	MarkerPath     string
	RequiredText   string
	Prompt         string
	RetryMessage   string
	SuccessMessage string
	Input          io.Reader
	Output         io.Writer
}

// ConfirmOnce skips input when MarkerPath already exists. Otherwise it keeps
// prompting until RequiredText is entered, then persists the marker. Prompted
// reports whether terminal input was required during this invocation.
func ConfirmOnce(options ConfirmationOptions) (prompted bool, err error) {
	if options.MarkerPath == "" {
		return false, errors.New("confirmation marker path is empty")
	}
	if options.RequiredText == "" {
		return false, errors.New("required confirmation text is empty")
	}
	if _, err := os.Stat(options.MarkerPath); err == nil {
		return false, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("inspect confirmation marker: %w", err)
	}

	input := options.Input
	if input == nil {
		input = os.Stdin
	}
	output := options.Output
	if output == nil {
		output = os.Stdout
	}

	scanner := bufio.NewScanner(input)
	for {
		if options.Prompt != "" {
			if _, err := fmt.Fprint(output, options.Prompt); err != nil {
				return true, fmt.Errorf("write confirmation prompt: %w", err)
			}
		}
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return true, fmt.Errorf("read confirmation: %w", err)
			}
			return true, io.EOF
		}
		if strings.TrimSpace(scanner.Text()) != options.RequiredText {
			if options.RetryMessage != "" {
				if _, err := fmt.Fprintln(output, options.RetryMessage); err != nil {
					return true, fmt.Errorf("write confirmation retry message: %w", err)
				}
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(options.MarkerPath), 0o755); err != nil {
			return true, fmt.Errorf("create confirmation directory: %w", err)
		}
		if err := os.WriteFile(options.MarkerPath, []byte("1"), 0o644); err != nil {
			return true, fmt.Errorf("persist confirmation: %w", err)
		}
		if options.SuccessMessage != "" {
			if _, err := fmt.Fprintln(output, options.SuccessMessage); err != nil {
				return true, fmt.Errorf("write confirmation success message: %w", err)
			}
		}
		return true, nil
	}
}
