package buildversion

import "fmt"

// NOTE: these variables are injected at build time

var (
	version, gitSHA, buildTime string
)

func Version() string {
	if version == "" {
		return "v0.0.0"
	}
	return version
}

func ShortSHA() string {
	if gitSHA == "" {
		return "local"
	}
	return gitSHA[:7]
}

func ImageTag() string {
	return fmt.Sprintf("%s", Version())
}
