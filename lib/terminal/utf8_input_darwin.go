//go:build darwin

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
	original, err := unix.IoctlGetTermios(fd, unix.TIOCGETA)
	if err != nil {
		return noInputRestore, nil
	}
	if original.Iflag&unix.IUTF8 != 0 {
		return noInputRestore, nil
	}
	updated := *original
	updated.Iflag |= unix.IUTF8
	if err := unix.IoctlSetTermios(fd, unix.TIOCSETA, &updated); err != nil {
		return noInputRestore, err
	}
	return func() error {
		return unix.IoctlSetTermios(fd, unix.TIOCSETA, original)
	}, nil
}

func noInputRestore() error {
	return nil
}

func enableInteractiveInput(fd int) (func() error, error) {
	original, err := unix.IoctlGetTermios(fd, unix.TIOCGETA)
	if err != nil {
		return noInputRestore, err
	}
	updated := *original
	updated.Lflag &^= unix.ICANON | unix.ECHO | unix.ISIG | unix.IEXTEN
	updated.Cc[unix.VMIN] = 1
	updated.Cc[unix.VTIME] = 0
	if err := unix.IoctlSetTermios(fd, unix.TIOCSETA, &updated); err != nil {
		return noInputRestore, err
	}
	return func() error {
		return unix.IoctlSetTermios(fd, unix.TIOCSETA, original)
	}, nil
}
