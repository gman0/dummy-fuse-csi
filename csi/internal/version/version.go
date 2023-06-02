package version

import (
	"fmt"
	"time"
)

// Values to be injected during build (ldflags).
var (
	buildTime = time.Now()
	version   = "unreleased"
	commit    string
	treestate string
	metadata  string
)

// Version returns the csi-cvmfsplugin version. It is expected this is defined
// as a semantic version number, or 'unreleased' for unreleased code.
func Version() string {
	return version
}

// Commit returns the git commit SHA for the code that the plugin was built from.
func Commit() string {
	return commit
}

// TreeState returns the git tree state. Can be "clean" or "dirty".
func TreeState() string {
	return treestate
}

// Metadata returns metadata passed during build.
func Metadata() string {
	return metadata
}

// BuildTime returns the date the package was built.
func BuildTime() time.Time {
	return buildTime
}

// FullVersion constructs a string with full version information.
func FullVersion() string {
	return fmt.Sprintf("%s (commit: %s (%s); build time: %s; metadata: %s)",
		Version(), Commit(), TreeState(), BuildTime(), Metadata())
}
