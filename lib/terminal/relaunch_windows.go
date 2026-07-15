//go:build windows

package terminal

// Windows console-subsystem executables are given a console by the operating
// system. The build must not use "-H windowsgui".
func relaunch() (bool, error) {
	return false, nil
}
