package dummy

import (
	"fmt"
	"os"
	"syscall"
)

func makeMountpoint(path string) error {
	return os.MkdirAll(path, 0750)
}

func rmMountpoint(path string) error {
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	return nil
}

func isMountpoint(path string) (bool, error) {
	exitCode, err := execCommandExitCode("mountpoint", path)
	if err != nil {
		return false, fmt.Errorf("failed to check for mountpoint %s: %v", path, err)
	}

	//if exitCode != 0 && exitCode != 32 /* See man mountpoint(1) */ {
	//	return false, fmt.Errorf("failed to check for mountpoint %s: exit code %d", path, exitCode)
	//}

	return exitCode == 0, nil
}

func isDanglingMountpoint(path string) (bool, error) {
	var st syscall.Stat_t
	err := syscall.Stat(path, &st)

	if err != nil {
		if err == syscall.ENOTCONN {
			// Caused by exited FUSE
			return true, nil
		}

		if err == syscall.ENOENT {
			// Path doesn't exist
			return false, nil
		}

		return false, err
	}

	return false, nil
}

func pathExists(path string) (bool, error) {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}
