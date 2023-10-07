package pgupgrade

import (
	"fmt"
	"strings"

	"golang.org/x/mod/semver"
)

func AutoDiscoverPostgresVersionFromImage(image string) (string, error) {
	splitted := strings.SplitAfter(image, ":")

	if len(splitted) < 2 {
		return "", fmt.Errorf("failed to auto discover postgres version due to: image is missing tag")
	}
	tag := splitted[1]

	// Package semver implements comparison of semantic version strings.
	// In this package, semantic version strings must begin with a leading "v", as in "v1.0.0".
	// https://pkg.go.dev/golang.org/x/mod/semver#IsValid
	if !strings.HasPrefix(tag, "v") {
		tag = fmt.Sprintf("v%s", tag)
	}

	if !semver.IsValid(tag) {
		return "", fmt.Errorf("tag %s is not a valid semver", tag)
	}

	res := semver.Major(semver.Canonical(tag))
	if res == "" {
		return "", fmt.Errorf("failed to auto discover postgres version due to: failed to detect major postgres version from image tag: %q", tag)
	}

	// strip the "v" from the result
	return strings.TrimPrefix(res, "v"), nil
}
