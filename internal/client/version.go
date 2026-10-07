package client

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseVersion reads "1.2.3", "v1.2.3" or "1.2.3-rc.1" into its numeric core. ok is false for anything
// else (such as a "dev" build), which callers treat as "don't enforce a minimum".
func ParseVersion(s string) (v [3]int, ok bool) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		s = s[:i]
	}
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return v, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return v, false
		}
		v[i] = n
	}
	return v, true
}

// CheckMinimum returns an error when the server's version is parseable and older than min.
func CheckMinimum(serverVersion, min string) error {
	server, ok := ParseVersion(serverVersion)
	if !ok {
		return nil
	}
	want, ok := ParseVersion(min)
	if !ok {
		return nil
	}
	for i := range server {
		if server[i] != want[i] {
			if server[i] < want[i] {
				return fmt.Errorf("Flare server %s is older than the minimum supported by this provider (%s); upgrade Flare or use an older provider", serverVersion, min)
			}
			return nil
		}
	}
	return nil
}
