//go:build linux

package terminal

import (
	"io"

	"golang.org/x/sys/unix"
)

func enableUTF8Input(input io.Reader) (func() error, error) {
	stream, ok := input.(fileDescriptor)
	if !ok {
		return noInputRestore, nil
	}
	fd := int(stream.Fd())
	original, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		return noInputRestore, nil
	}
	if original.Iflag&unix.IUTF8 != 0 {
		return noInputRestore, nil
	}
	updated := *original
	updated.Iflag |= unix.IUTF8
	if err := unix.IoctlSetTermios(fd, unix.TCSETS, &updated); err != nil {
		return noInputRestore, err
	}
	return func() error {
		return unix.IoctlSetTermios(fd, unix.TCSETS, original)
	}, nil
}

func noInputRestore() error {
	return nil
}
