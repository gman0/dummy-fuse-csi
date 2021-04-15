package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"dummy-fuse-csi/internal/dummy"
	"dummy-fuse-csi/internal/dummy/version"
)

var (
	endpoint       string
	name           string
	nodeID         string
	mountCachePath string

	showVersion bool
)

func main() {
	flag.StringVar(&endpoint, "endpoint", "", "CSI endpoint (path to Unix socket file)")
	flag.StringVar(&name, "name", "dummy.csi", "driver name")
	flag.StringVar(&nodeID, "nodeid", "", "node ID")
	flag.BoolVar(&showVersion, "version", false, "show version and exit")
	flag.StringVar(&mountCachePath, "mount-cache-path", "", "hostPath volume for storing mount cache entries")

	flag.Parse()

	if showVersion {
		fmt.Println(version.Version)
		os.Exit(0)
	}

	d, err := dummy.NewDriver(&dummy.DriverOpts{
		Endpoint:       endpoint,
		Name:           name,
		NodeID:         nodeID,
		MountCachePath: mountCachePath,
	})

	if err != nil {
		log.Fatalln("Driver is misconfigured:", err)
	}

	if err = d.Run(); err != nil {
		log.Fatalln("Driver failed to run:", err)
	}
}
