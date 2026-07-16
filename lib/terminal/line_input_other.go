//go:build !darwin && !linux

package terminal

import "io"

func newConfirmationLineInput(input io.Reader, _ io.Writer) (confirmationLineInput, func() error, error) {
	return newScannerLineInput(input), noLineInputRestore, nil
}
