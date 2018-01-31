package version

import (
	"bytes"
	"fmt"
)

var (
	// GitCommit is the git commit that was compiled. This will be filled in by the compiler.
	GitCommit string

	// Version is the main version number that is being run at the moment.
	Version = "0.0.3"

	// VersionPrerelease is a pre-release marker for the version. If this is "" (empty string)
	// then it means that it is a final release. Otherwise, this is a pre-release
	// such as "dev" (in development), "beta", "rc1", etc.
	VersionPrerelease = "dev"
)

// Get returns the version number
func Get(rev bool) string {
	var v bytes.Buffer
	fmt.Fprintf(&v, "%s", Version)
	if VersionPrerelease != "" {
		fmt.Fprintf(&v, "-%s", VersionPrerelease)
	}

	if rev && GitCommit != "" {
		fmt.Fprintf(&v, " (%s)", GitCommit)
	}

	return v.String()
}
