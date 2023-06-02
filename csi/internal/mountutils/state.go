package mountutils

import (
	mount "k8s.io/mount-utils"
)

type (
	State int
)

const (
	StUnknown State = iota
	StNotMounted
	StMounted
	StCorrupted
)

var (
	dummyMounter = mount.New("")
)

func (s State) String() string {
	return [...]string{
		"UNKNOWN",
		"NOT_MOUNTED",
		"MOUNTED",
		"CORRUPTED",
	}[int(s)]
}

func GetState(p string) (State, error) {
	isNotMnt, err := mount.IsNotMountPoint(dummyMounter, p)
	if err != nil {
		if mount.IsCorruptedMnt(err) {
			return StCorrupted, nil
		}

		return StUnknown, err
	}

	if !isNotMnt {
		return StMounted, nil
	}

	return StNotMounted, nil
}
