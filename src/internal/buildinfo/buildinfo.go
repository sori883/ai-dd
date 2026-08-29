// Package buildinfo exposes build metadata injected by the Go linker.
package buildinfo

// Version is the application version. Release builds replace it with -ldflags -X.
var Version = "dev"

// Commit is the source revision. Release builds replace it with -ldflags -X.
var Commit = "unknown"

// Info is an immutable snapshot of the application build metadata.
type Info struct {
	Version string
	Commit  string
}

// Current returns the build metadata configured for the current binary.
func Current() Info {
	return Info{
		Version: Version,
		Commit:  Commit,
	}
}
