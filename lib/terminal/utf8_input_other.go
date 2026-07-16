//go:build !darwin && !linux

package terminal

import "io"

func enableUTF8Input(io.Reader) (func() error, error) {
	return func() error { return nil }, nil
}
