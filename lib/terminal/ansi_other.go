//go:build !windows

package terminal

import (
	"io"

	"golang.org/x/term"
)

func enableANSI(output io.Writer) (bool, error) {
	stream, ok := output.(fileDescriptor)
	return ok && term.IsTerminal(int(stream.Fd())), nil
}
