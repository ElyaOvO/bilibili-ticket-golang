package terminal

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
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

type terminalColorMode uint8

const (
	colorNone terminalColorMode = iota
	colorANSI16
	colorANSI256
	colorTrue
)

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
	colorMode := detectTerminalColorMode(ansiEnabled)

	message := options.Prompt
	scanner := bufio.NewScanner(input)
	for {
		if message != "" {
			displayMessage := renderStyledTextWithColorMode(message, styledColorMode(options.StyledText, colorMode))
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
			displayMessage := renderStyledTextWithColorMode(options.SuccessMessage, styledColorMode(options.StyledText, colorMode))
			if _, err := fmt.Fprintln(output, displayMessage); err != nil {
				return true, fmt.Errorf("write confirmation success message: %w", err)
			}
		}
		return true, nil
	}
}

func styledColorMode(styled bool, mode terminalColorMode) terminalColorMode {
	if !styled {
		return colorNone
	}
	return mode
}

func renderStyledTextWithColorMode(message string, mode terminalColorMode) string {
	ansiEnabled := mode != colorNone
	bold := ""
	if ansiEnabled {
		bold = ansiBold
	}
	message = strings.ReplaceAll(message, "{{bold}}", bold)
	message = colorTagPattern.ReplaceAllStringFunc(message, func(tag string) string {
		sequence, valid := ansiFromColorTag(tag, mode)
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

func ansiFromColorTag(tag string, mode terminalColorMode) (string, bool) {
	parts := strings.Split(strings.TrimSuffix(strings.TrimPrefix(tag, "{{"), "}}"), "|")
	foreground, ok := parseHexColor(parts[0])
	if !ok {
		return "", false
	}

	codes := ansiColorCodes(false, foreground, mode)
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
			codes = append(codes, ansiColorCodes(true, background, mode)...)
			backgroundSet = true
		default:
			return "", false
		}
	}
	return "\x1b[" + strings.Join(codes, ";") + "m", true
}

func ansiColorCodes(background bool, rgb [3]string, mode terminalColorMode) []string {
	prefix := "38"
	if background {
		prefix = "48"
	}
	switch mode {
	case colorTrue:
		return []string{prefix, "2", rgb[0], rgb[1], rgb[2]}
	case colorANSI256:
		return []string{prefix, "5", strconv.Itoa(xtermColorIndex(rgb))}
	case colorANSI16:
		return []string{strconv.Itoa(ansi16ColorCode(rgb, background))}
	default:
		return nil
	}
}

func xtermColorIndex(rgb [3]string) int {
	red, _ := strconv.Atoi(rgb[0])
	green, _ := strconv.Atoi(rgb[1])
	blue, _ := strconv.Atoi(rgb[2])
	red = (red*5 + 127) / 255
	green = (green*5 + 127) / 255
	blue = (blue*5 + 127) / 255
	return 16 + 36*red + 6*green + blue
}

func ansi16ColorCode(rgb [3]string, background bool) int {
	palette := [][3]int{
		{0, 0, 0}, {128, 0, 0}, {0, 128, 0}, {128, 128, 0},
		{0, 0, 128}, {128, 0, 128}, {0, 128, 128}, {192, 192, 192},
		{128, 128, 128}, {255, 0, 0}, {0, 255, 0}, {255, 255, 0},
		{0, 0, 255}, {255, 0, 255}, {0, 255, 255}, {255, 255, 255},
	}
	red, _ := strconv.Atoi(rgb[0])
	green, _ := strconv.Atoi(rgb[1])
	blue, _ := strconv.Atoi(rgb[2])
	bestIndex := 0
	bestDistance := int(^uint(0) >> 1)
	for index, color := range palette {
		distance := square(red-color[0]) + square(green-color[1]) + square(blue-color[2])
		if distance < bestDistance {
			bestIndex = index
			bestDistance = distance
		}
	}
	base := 30
	if background {
		base = 40
	}
	if bestIndex >= 8 {
		base += 60
		bestIndex -= 8
	}
	return base + bestIndex
}

func square(value int) int {
	return value * value
}

func detectTerminalColorMode(ansiEnabled bool) terminalColorMode {
	if !ansiEnabled {
		return colorNone
	}
	if runtime.GOOS == "windows" {
		return colorTrue
	}

	termProgram := strings.ToLower(os.Getenv("TERM_PROGRAM"))
	if termProgram == "apple_terminal" {
		if mode := terminfoColorMode(); mode != colorNone {
			return mode
		}
		return colorANSI256
	}
	if termProgram == "iterm.app" || termProgram == "wezterm" ||
		termProgram == "vscode" || termProgram == "ghostty" {
		return colorTrue
	}
	colorTerm := strings.ToLower(os.Getenv("COLORTERM"))
	if colorTerm == "truecolor" || colorTerm == "24bit" {
		return colorTrue
	}
	if mode := terminfoColorMode(); mode != colorNone {
		return mode
	}
	term := strings.ToLower(os.Getenv("TERM"))
	if term == "dumb" {
		return colorNone
	}
	if strings.Contains(term, "truecolor") || strings.Contains(term, "24bit") ||
		strings.Contains(term, "direct") || strings.Contains(term, "kitty") ||
		strings.Contains(term, "alacritty") {
		return colorTrue
	}
	if strings.Contains(term, "256color") {
		return colorANSI256
	}
	return colorANSI16
}

func terminfoColorMode() terminalColorMode {
	if runtime.GOOS == "windows" || os.Getenv("TERM") == "" {
		return colorNone
	}
	output, err := exec.Command("tput", "colors").Output()
	if err != nil {
		return colorNone
	}
	count, err := strconv.Atoi(strings.TrimSpace(string(output)))
	if err != nil {
		return colorNone
	}
	return colorModeFromCount(count)
}

func colorModeFromCount(count int) terminalColorMode {
	switch {
	case count >= 1<<24:
		return colorTrue
	case count >= 256:
		return colorANSI256
	case count > 0:
		return colorANSI16
	default:
		return colorNone
	}
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
