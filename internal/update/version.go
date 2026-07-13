package update

import (
	"fmt"
	"regexp"
	"strings"
)

var stableVersionRE = regexp.MustCompile(`^v?(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:\+([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$`)

type stableVersion struct {
	parts [3]string
}

func parseStableVersion(value string) (stableVersion, error) {
	value = strings.TrimSpace(value)
	match := stableVersionRE.FindStringSubmatch(value)
	if match == nil {
		return stableVersion{}, fmt.Errorf("%q is not a stable semantic version", value)
	}
	return stableVersion{parts: [3]string{match[1], match[2], match[3]}}, nil
}

// CompareVersions compares two stable semantic versions. It returns -1, 0, or
// 1 when current is respectively older than, equal to, or newer than latest.
// A leading v and build metadata are accepted; prereleases are rejected.
func CompareVersions(current, latest string) (int, error) {
	a, err := parseStableVersion(current)
	if err != nil {
		return 0, fmt.Errorf("current version: %w", err)
	}
	b, err := parseStableVersion(latest)
	if err != nil {
		return 0, fmt.Errorf("latest version: %w", err)
	}

	for i := range a.parts {
		if comparison := compareNumericString(a.parts[i], b.parts[i]); comparison != 0 {
			return comparison, nil
		}
	}
	return 0, nil
}

func compareNumericString(a, b string) int {
	if len(a) < len(b) {
		return -1
	}
	if len(a) > len(b) {
		return 1
	}
	return strings.Compare(a, b)
}

func normalizedVersion(value string) (string, error) {
	v, err := parseStableVersion(value)
	if err != nil {
		return "", err
	}
	return strings.Join(v.parts[:], "."), nil
}
