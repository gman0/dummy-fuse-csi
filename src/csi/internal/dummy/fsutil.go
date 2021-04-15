package dummy

import (
	"fmt"
	"os"
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

	return exitCode == 0, nil
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
