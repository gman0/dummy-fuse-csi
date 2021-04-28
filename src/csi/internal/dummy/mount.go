package dummy

import (
	"bytes"
	"fmt"
)

// Checks if unmount error `errBs` is critical.
// Some errors are absorbed to preserve idempotency.
func isCriticalUnmountErr(errBs []byte) bool {
	absorbErrs := [][]byte{
		[]byte("Invalid argument"),          // Not mounted or Not a directory
		[]byte("No such file or directory"), // ENOENT
	}

	for i := range absorbErrs {
		if bytes.Equal(errBs, absorbErrs[i]) {
			return false
		}
	}

	return true
}

type mounterUnmounter interface {
	mount(dev, mountpoint string) error
	unmount(mountpoint string) error
}

type fuseMounter struct{}

func (fuseMounter) mount(_, mountpoint string) error {
	return execCommandErr("dummy-fuse", mountpoint)
}

func (fuseMounter) unmount(mountpoint string) error {
	_, stderr, err := execCommand("fusermount3", "-u", mountpoint)
	if err != nil {
		prefix := []byte(fmt.Sprintf("fusermount3: failed to unmount %s: ", mountpoint))

		if !bytes.HasPrefix(stderr, prefix) {
			return err
		}

		if isCriticalUnmountErr(stderr[len(prefix):]) {
			return err
		}
	}

	return nil
}

type bindMounter struct{}

func (bindMounter) mount(from, to string) error {
	return execCommandErr("mount", "--bind", from, to)
}

func (bindMounter) unmount(mountpoint string) error {
	_, stderr, err := execCommand("umount", mountpoint)
	if err != nil {
		prefix := []byte(fmt.Sprintf("umount: can't unmount %s: ", mountpoint))

		if !bytes.HasPrefix(stderr, prefix) {
			return err
		}

		if isCriticalUnmountErr(stderr[len(prefix):]) {
			return err
		}
	}

	return nil
}
