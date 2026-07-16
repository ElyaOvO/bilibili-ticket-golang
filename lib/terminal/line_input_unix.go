//go:build darwin || linux

package terminal

import (
	"bufio"
	"fmt"
	"io"
	"unicode"

	"golang.org/x/term"
)

type rawLineInput struct {
	reader *bufio.Reader
	output io.Writer
}

func newConfirmationLineInput(input io.Reader, output io.Writer) (confirmationLineInput, func() error, error) {
	inputStream, inputOK := input.(fileDescriptor)
	outputStream, outputOK := output.(fileDescriptor)
	if !inputOK || !outputOK || !term.IsTerminal(int(inputStream.Fd())) || !term.IsTerminal(int(outputStream.Fd())) {
		return newScannerLineInput(input), noLineInputRestore, nil
	}

	restore, err := enableInteractiveInput(int(inputStream.Fd()))
	if err != nil {
		return nil, noLineInputRestore, err
	}
	return &rawLineInput{reader: bufio.NewReader(input), output: output}, restore, nil
}

func (input *rawLineInput) ReadLine() (string, error) {
	line := make([]rune, 0, 16)
	for {
		value, _, err := input.reader.ReadRune()
		if err != nil {
			return "", err
		}
		switch value {
		case '\r', '\n':
			if _, err := fmt.Fprint(input.output, "\r\n"); err != nil {
				return "", err
			}
			return string(line), nil
		case '\b', '\x7f':
			if len(line) == 0 {
				continue
			}
			last := line[len(line)-1]
			line = line[:len(line)-1]
			if err := eraseRune(input.output, last); err != nil {
				return "", err
			}
		case '\x03':
			_, _ = fmt.Fprint(input.output, "^C\r\n")
			return "", io.EOF
		case '\x04':
			if len(line) == 0 {
				return "", io.EOF
			}
		default:
			if unicode.IsControl(value) {
				continue
			}
			line = append(line, value)
			if _, err := fmt.Fprint(input.output, string(value)); err != nil {
				return "", err
			}
		}
	}
}
