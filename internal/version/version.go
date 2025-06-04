// SPDX-License-Identifier:Apache-2.0

package version

import (
	"fmt"
	"runtime"
)

var (
	version   = "0.15.2"
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

// Version returns the version string.
func Version() string { return version }

// CommitHash returns the commit hash at which the binary was built.
func CommitHash() string { return gitCommit }

// Branch returns the branch at which the binary was built.
func Branch() string { return gitBranch }

// GoString returns the compiler, compiler version and architecture of the build.
func GoString() string {
	return fmt.Sprintf("%s / %s / %s", runtime.Compiler, runtime.Version(), runtime.GOARCH)
}
