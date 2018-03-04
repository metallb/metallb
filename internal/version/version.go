package version

import "fmt"

var (
	version   = "0.4.2"
	gitCommit string // Provided by ldflags during build
	gitBranch string // Provided by ldflags during build
)

// String returns a human-readable version string.
func String() string {
	hasVersion := version != ""
	hasBuildInfo := gitCommit != ""

	switch {
	case hasVersion && hasBuildInfo:
		return fmt.Sprintf("version %s (commit %s, branch %s)", version, gitCommit, gitBranch)
	case !hasVersion && hasBuildInfo:
		return fmt.Sprintf("(commit %s, branch %s)", gitCommit, gitBranch)
	case hasVersion && !hasBuildInfo:
		return fmt.Sprintf("version %s (no build information)", version)
	default:
		return "(no version or build info)"
	}
}
