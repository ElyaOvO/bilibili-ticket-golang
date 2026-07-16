package terminal

import (
	"bufio"
	"io"
)

type confirmationLineInput interface {
	ReadLine() (string, error)
}

type scannerLineInput struct {
	scanner *bufio.Scanner
}

func newScannerLineInput(input io.Reader) confirmationLineInput {
	return &scannerLineInput{scanner: bufio.NewScanner(input)}
}

func (input *scannerLineInput) ReadLine() (string, error) {
	if input.scanner.Scan() {
		return input.scanner.Text(), nil
	}
	if err := input.scanner.Err(); err != nil {
		return "", err
	}
	return "", io.EOF
}

func noLineInputRestore() error {
	return nil
}
