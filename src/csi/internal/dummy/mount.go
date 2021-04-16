package dummy

type mounterUnmounter interface {
	mount(dev, mountpoint string) error
	unmount(mountpoint string) error
}

type fuseMounter struct{}

func (fuseMounter) mount(_, mountpoint string) error {
	return execCommandErr("dummy-fuse", mountpoint)
}

func (fuseMounter) unmount(mountpoint string) error {
	return execCommandErr("fusermount3", "-u", mountpoint)
}

type bindMounter struct{}

func (bindMounter) mount(from, to string) error {
	return execCommandErr("mount", "--bind", from, to)
}

func (bindMounter) unmount(mountpoint string) error {
	return execCommandErr("umount", mountpoint)
}
