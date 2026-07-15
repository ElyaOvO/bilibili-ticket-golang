//go:build !windows && !linux && !darwin

package terminal

func relaunch() (bool, error) {
	return false, nil
}
