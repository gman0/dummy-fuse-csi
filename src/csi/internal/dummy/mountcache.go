package dummy

import (
	"encoding/json"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"path"
)

const (
	mountCacheStagedDir    = "staged"
	mountCachePublishedDir = "published"
)

type mountCacheStaged struct {
	StageTargetPath string
}

type mountCachePublished struct {
	StageTargetPath string
	TargetPath      string
}

func listMountCacheEntries(cachePath string) []fs.FileInfo {
	if err := os.MkdirAll(cachePath, 0755); err != nil {
		log.Printf("couldn't create mount cache at %s: %v", cachePath, err)
		return nil
	}

	entries, err := ioutil.ReadDir(cachePath)
	if err != nil {
		log.Printf("failed to read mount cache at %s: %v", cachePath, err)
	}

	log.Printf("Found %d entries in %s", len(entries), cachePath)
	return entries
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

func remountStaged(mountCacheRoot string) {
	cachePath := mountCacheStagedPath(mountCacheRoot)

	entries := listMountCacheEntries(cachePath)
	if entries == nil || len(entries) == 0 {
		return
	}

	for i := range entries {
		entryPath := path.Join(cachePath, entries[i].Name())

		var s mountCacheStaged
		readJsonFile(entryPath, &s)

		fuseUnmount(s.StageTargetPath)

		if err := fuseMount(s.StageTargetPath); err != nil {
			log.Printf("failed to remount %s: %v", s.StageTargetPath, err)
		} else {
			log.Printf("successfully remounted %s", s.StageTargetPath)
		}
	}
}

func remountPublished(mountCacheRoot string) {
	cachePath := mountCachePublishedPath(mountCacheRoot)

	entries := listMountCacheEntries(cachePath)
	if entries == nil || len(entries) == 0 {
		return
	}

	for i := range entries {
		entryPath := path.Join(cachePath, entries[i].Name())

		var s mountCachePublished
		readJsonFile(entryPath, &s)

		bindUnmount(s.TargetPath)

		if err := bindMount(s.StageTargetPath, s.TargetPath); err != nil {
			log.Printf("failed to bind %s to %s: %v", s.StageTargetPath, s.TargetPath, err)
		} else {
			log.Printf("successfully bound %s to %s", s.StageTargetPath, s.TargetPath)
		}
	}
}

func cacheStageMount(volID, stageTargetPath, mountCacheRoot string) {
	entryPath := path.Join(mountCacheStagedPath(mountCacheRoot), volID)
	err := writeJsonFile(entryPath, mountCacheStaged{
		StageTargetPath: stageTargetPath,
	})

	if err != nil {
		log.Printf("failed to cache staged mount entry to %s: %v", entryPath, err)
	}
}

func forgetStageMount(volID, mountCacheRoot string) {
	entryPath := path.Join(mountCacheStagedPath(mountCacheRoot), volID)
	if err := os.Remove(entryPath); err != nil {
		log.Printf("failed to remove staged mount cache entry %s: %v", entryPath, err)
	}
}

func cachePublishMount(volID, stageTargetPath, targetPath, mountCacheRoot string) {
	entryPath := path.Join(mountCachePublishedPath(mountCacheRoot), volID)
	err := writeJsonFile(entryPath, mountCachePublished{
		StageTargetPath: stageTargetPath,
		TargetPath:      targetPath,
	})

	if err != nil {
		log.Printf("failed to cache publish mount entry to %s: %v", entryPath, err)
	}
}

func forgetPublishMount(volID, mountCacheRoot string) {
	entryPath := path.Join(mountCachePublishedPath(mountCacheRoot), volID)
	if err := os.Remove(entryPath); err != nil {
		log.Printf("failed to remove publish mount cache entry %s: %v", entryPath, err)
	}
}
