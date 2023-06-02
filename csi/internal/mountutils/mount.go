package mountutils

import (
	"bytes"
	goexec "os/exec"

	"github.com/gman0/dummy-fuse-csi/csi/internal/exec"
)

func Unmount(mountpoint string, extraArgs ...string) error {
	out, err := exec.CombinedOutput(goexec.Command("umount", append(extraArgs, mountpoint)...))
	if err != nil {
		// There are no well-defined exit codes for cases of "not mounted"
		// and "doesn't exist". We need to check the output.
		if bytes.HasSuffix(out, []byte(": not mounted")) ||
			bytes.Contains(out, []byte("No such file or directory")) {
			return nil
		}
	}

	return err
}
