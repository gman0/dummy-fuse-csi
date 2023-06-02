package node

import (
	"fmt"
	goexec "os/exec"

	"github.com/gman0/dummy-fuse-csi/csi/internal/exec"
	"github.com/gman0/dummy-fuse-csi/csi/internal/mountutils"
)

func bindMount(from, to string) error {
	_, err := exec.CombinedOutput(goexec.Command("mount", "--bind", from, to))
	return err
}

func slaveRecursiveBind(from, to string) error {
	_, err := exec.CombinedOutput(goexec.Command(
		"mount",
		from,
		to,

		// We bindmount recursively in order to retain any
		// existing CVMFS mounts inside of the autofs root.
		"--rbind",

		// We expect the autofs root in /cvmfs to be already marked
		// as shared, making it possible to send and receive mount
		// and unmount events between bindmounts. We need to make event
		// propagation one-way only (from autofs root to bindmounts)
		// however, because, when unmounting, we do so recursively, and
		// this would then mean attempting to unmount autofs-CVMFS mounts
		// in the rest of the bindmounts (used by other Pods on the node
		// that also use CVMFS), which is not desirable of course.
		"--make-slave",
	))

	return err
}

func recursiveUnmount(mountpoint string) error {
	// We need recursive unmount because there are live mounts inside the bindmount.
	// Unmounting only the upper autofs mount would result in EBUSY.
	return mountutils.Unmount(mountpoint, "--recursive")
}

func mountDummyFuse(mountpoint string) error {
	return exec.Run(goexec.Command("dummy-fuse", mountpoint))
}

// Mount function signature used by reconcileMount().
type mountFunc func(mountpoint string) error

// Reconciles the mountpoint. If it's corrupted (e.g. ENOTCONN -- its mount provider exited)
// it unmounts it first. If it's unmounted, it calls the mountF function to restore the volume.
// If it is already mounted, it does nothing.
func reconcileMount(mountpoint string, mountF mountFunc) error {
	mntState, err := mountutils.GetState(mountpoint)
	if err != nil {
		return fmt.Errorf("failed to probe mountpoint %s: %v", mountpoint, err)
	}

	switch mntState {
	case mountutils.StCorrupted:
		// Detected mount corruption. Try to remount.
		if err := mountutils.Unmount(mountpoint); err != nil {
			return fmt.Errorf("failed to unmount %s during mount recovery: %v", mountpoint, err)
		}
		fallthrough
	case mountutils.StNotMounted:
		if err := mountF(mountpoint); err != nil {
			return fmt.Errorf("failed mount into %s: %v", mountpoint, err)
		}
		fallthrough
	case mountutils.StMounted:
		return nil
	default:
		return fmt.Errorf("unexpected mountpoint state in %s: expected %s or %s, got %s",
			mountpoint, mountutils.StNotMounted, mountutils.StMounted, mntState)
	}
}
