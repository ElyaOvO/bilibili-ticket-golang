//go:build windows

package terminal

import (
	"io"

	"golang.org/x/sys/windows"
)

const enableVirtualTerminalProcessing = 0x0004
const utf8CodePage = 65001

func enableANSI(output io.Writer) (bool, error) {
	stream, ok := output.(fileDescriptor)
	if !ok {
		return false, nil
	}

	handle := windows.Handle(stream.Fd())
	var mode uint32
	if err := windows.GetConsoleMode(handle, &mode); err != nil {
		// Redirected files and pipes do not support console control sequences.
		return false, nil
	}
	if mode&enableVirtualTerminalProcessing == 0 {
		if err := windows.SetConsoleMode(handle, mode|enableVirtualTerminalProcessing); err != nil {
			return false, err
		}
	}
	if err := windows.SetConsoleOutputCP(utf8CodePage); err != nil {
		return false, err
	}
	if err := windows.SetConsoleCP(utf8CodePage); err != nil {
		return false, err
	}
	return true, nil
}
