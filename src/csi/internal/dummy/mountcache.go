package dummy

import (
	"encoding/json"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"path"
)

// Mount cache stores a JSON document with mountpoint paths for each
// NodeStageVolume and NodePublish RPC. The documents are meant to be
// stored in a hostPath volume, and should survive dummy-fuse-csi Node
// plugin crash/update/restart. These documents are deleted once the volume
// is successfully unpublished/unstaged.
//
// The idea is to attempt to restore existing FUSE mounts on a node for workloads
// that consume the volumes.
//
// Driver.Run() calls remountStaged and remountPublished functions that do the actual remounts.
//
// This doesn't work because workloads that mount these volumes have stale bind mounts
// and these need to be restored by the orchestrator.

const (
	mountCacheStagedDir    = "staged"
	mountCachePublishedDir = "published"
)

type mountCacheEntry struct {
	StageTargetPath string `json:",omitempty"`
	TargetPath      string `json:",omitempty"`
}

type remountPaths interface {
	mountPaths(e *mountCacheEntry) (string, string)
	unmountPath(e *mountCacheEntry) string
}

type fuseRemountPaths struct{}

func (fuseRemountPaths) mountPaths(e *mountCacheEntry) (string, string) { return "", e.StageTargetPath }
func (fuseRemountPaths) unmountPath(e *mountCacheEntry) string          { return e.StageTargetPath }

type bindRemountPaths struct{}

func (bindRemountPaths) mountPaths(e *mountCacheEntry) (string, string) {
	return e.StageTargetPath, e.TargetPath
}

func (bindRemountPaths) unmountPath(e *mountCacheEntry) string { return e.TargetPath }

func listMountCacheEntries(cachePath string) ([]fs.FileInfo, error) {
	if err := os.MkdirAll(cachePath, 0755); err != nil {
		return nil, err
	}

	entries, err := ioutil.ReadDir(cachePath)
	if err != nil {
		return nil, err
	}

	return entries, nil
}

func readJsonFile(filename string, v interface{}) error {
	contents, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}

	return json.Unmarshal(contents, v)
}

func writeJsonFile(filename string, v interface{}) error {
	contents, err := json.Marshal(v)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(filename, contents, 0644)
}

func mountCacheStagedPath(mountCacheRoot string) string {
	return path.Join(mountCacheRoot, mountCacheStagedDir)
}

func mountCachePublishedPath(mountCacheRoot string) string {
	return path.Join(mountCacheRoot, mountCachePublishedDir)
}

func remountFromCache(cachePath string, m mounterUnmounter, r remountPaths) {
	entries, err := listMountCacheEntries(cachePath)
	if err != nil {
		log.Printf("Failed to list mount cache %s: %v", cachePath, err)
		return
	}

	if entries == nil || len(entries) == 0 {
		log.Printf("No mount cache entries in %s", cachePath)
		return
	}

	var e mountCacheEntry
	for i := range entries {
		entryPath := path.Join(cachePath, entries[i].Name())
		readJsonFile(entryPath, &e)

		m.unmount(r.unmountPath(&e))

		if err := m.mount(r.mountPaths(&e)); err != nil {
			log.Printf("failed to remount %s %s: %e", e.TargetPath, e.StageTargetPath, err)
		} else {
			log.Printf("successfully remounted %s %s", e.TargetPath, e.StageTargetPath)
		}
	}
}

func remountStaged(mountCacheRoot string) {
	remountFromCache(
		mountCacheStagedPath(mountCacheRoot),
		fuseMounter{},
		fuseRemountPaths{},
	)
}

func remountPublished(mountCacheRoot string) {
	remountFromCache(
		mountCachePublishedPath(mountCacheRoot),
		bindMounter{},
		bindRemountPaths{},
	)
}

func cacheStageMount(volID, stageTargetPath, mountCacheRoot string) {
	entryPath := path.Join(mountCacheStagedPath(mountCacheRoot), volID)
	err := writeJsonFile(entryPath, mountCacheEntry{
		StageTargetPath: stageTargetPath,
	})

	if err != nil {
		log.Printf("Failed to cache staged mount entry to %s: %v", entryPath, err)
	} else {
		log.Printf("Saved stage mount entry to %s", entryPath)
	}
}

func forgetStageMount(volID, mountCacheRoot string) {
	entryPath := path.Join(mountCacheStagedPath(mountCacheRoot), volID)
	if err := os.Remove(entryPath); err != nil {
		log.Printf("Failed to remove staged mount cache entry %s: %v", entryPath, err)
	} else {
		log.Printf("Forgot stage mount entry %s", entryPath)
	}
}

func cachePublishMount(volID, stageTargetPath, targetPath, mountCacheRoot string) {
	entryPath := path.Join(mountCachePublishedPath(mountCacheRoot), volID)
	err := writeJsonFile(entryPath, mountCacheEntry{
		StageTargetPath: stageTargetPath,
		TargetPath:      targetPath,
	})

	if err != nil {
		log.Printf("Failed to cache publish mount entry to %s: %v", entryPath, err)
	} else {
		log.Printf("Saved publish mount entry to %s", entryPath)
	}
}

func forgetPublishMount(volID, mountCacheRoot string) {
	entryPath := path.Join(mountCachePublishedPath(mountCacheRoot), volID)
	if err := os.Remove(entryPath); err != nil {
		log.Printf("Failed to remove publish mount cache entry %s: %v", entryPath, err)
	} else {
		log.Printf("Forgot publish mount entry %s", entryPath)
	}
}
