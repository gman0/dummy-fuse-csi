package dummy

func fuseMount(mountpoint string) error {
	return execCommandErr("dummy-fuse", mountpoint)
}

func fuseUnmount(mountpoint string) error {
	return execCommandErr("fusermount3", "-u", mountpoint)
}

func bindMount(from, to string) error {
	return execCommandErr("mount", "--bind", from, to)
}

func bindUnmount(mountpoint string) error {
	return execCommandErr("umount", mountpoint)
}
