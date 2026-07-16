package terminal

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// ConfirmationOptions configures a one-time exact-text confirmation in the
// terminal. Input and Output default to os.Stdin and os.Stdout.
type ConfirmationOptions struct {
	MarkerPath   string
	RequiredText string
	Prompt       string
	RetryMessage string
	// RewriteOnRetry replaces the previous terminal line with RetryMessage.
	RewriteOnRetry bool
	// StyledText renders supported inline style tags as ANSI sequences.
	StyledText     bool
	SuccessMessage string
	Input          io.Reader
	Output         io.Writer
}

const (
	ansiRewritePreviousLine = "\x1b[1A\x1b[2K\r"
	ansiBold                = "\x1b[1;7m"
	ansiReset               = "\x1b[0m"
)

var colorTagPattern = regexp.MustCompile(`\{\{#[^{}]+\}\}`)

type fileDescriptor interface {
	Fd() uintptr
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
	ansiEnabled := false
	if options.RewriteOnRetry || options.StyledText {
		var err error
		ansiEnabled, err = enableANSI(output)
		if err != nil {
			return true, fmt.Errorf("enable ANSI confirmation output: %w", err)
		}
	}
	rewriteOnRetry := options.RewriteOnRetry && ansiEnabled

	message := options.Prompt
	scanner := bufio.NewScanner(input)
	for {
		if message != "" {
			displayMessage := renderStyledText(message, options.StyledText && ansiEnabled)
			if _, err := fmt.Fprint(output, displayMessage); err != nil {
				return true, fmt.Errorf("write confirmation message: %w", err)
			}
		}
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return true, fmt.Errorf("read confirmation: %w", err)
			}
			return true, io.EOF
		}
		if strings.TrimSpace(scanner.Text()) != options.RequiredText {
			message = options.RetryMessage
			if rewriteOnRetry && message != "" {
				if _, err := fmt.Fprint(output, ansiRewritePreviousLine); err != nil {
					return true, fmt.Errorf("rewrite confirmation line: %w", err)
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
			displayMessage := renderStyledText(options.SuccessMessage, options.StyledText && ansiEnabled)
			if _, err := fmt.Fprintln(output, displayMessage); err != nil {
				return true, fmt.Errorf("write confirmation success message: %w", err)
			}
		}
		return true, nil
	}
}

func renderStyledText(message string, ansiEnabled bool) string {
	bold := ""
	if ansiEnabled {
		bold = ansiBold
	}
	message = strings.ReplaceAll(message, "{{bold}}", bold)
	message = colorTagPattern.ReplaceAllStringFunc(message, func(tag string) string {
		sequence, valid := ansiFromColorTag(tag)
		if !valid {
			return tag
		}
		if ansiEnabled {
			return sequence
		}
		return ""
	})
	reset := ""
	if ansiEnabled {
		reset = ansiReset
	}
	return strings.ReplaceAll(message, "{{/}}", reset)
}

func ansiFromColorTag(tag string) (string, bool) {
	parts := strings.Split(strings.TrimSuffix(strings.TrimPrefix(tag, "{{"), "}}"), "|")
	foreground, ok := parseHexColor(parts[0])
	if !ok {
		return "", false
	}

	codes := []string{"38", "2", foreground[0], foreground[1], foreground[2]}
	backgroundSet := false
	boldSet := false
	for _, part := range parts[1:] {
		switch {
		case part == "bold" && !boldSet:
			codes = append([]string{"1"}, codes...)
			boldSet = true
		case strings.HasPrefix(part, "#") && !backgroundSet:
			background, valid := parseHexColor(part)
			if !valid {
				return "", false
			}
			codes = append(codes, "48", "2", background[0], background[1], background[2])
			backgroundSet = true
		default:
			return "", false
		}
	}
	return "\x1b[" + strings.Join(codes, ";") + "m", true
}

func parseHexColor(value string) ([3]string, bool) {
	var rgb [3]string
	if len(value) != 7 || value[0] != '#' {
		return rgb, false
	}
	for index := range rgb {
		component, err := strconv.ParseUint(value[1+index*2:3+index*2], 16, 8)
		if err != nil {
			return rgb, false
		}
		rgb[index] = strconv.FormatUint(component, 10)
	}
	return rgb, true
}
