package controller

import (
	"regexp"
	"testing"
)

func TestAccessPointInterfaceId(t *testing.T) {
	crdPattern := regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_]{6,30}[A-Za-z0-9]$`)
	ewbiPattern := regexp.MustCompile(`^[a-z0-9]([-a-z0-9]{0,62}[a-z0-9])?$`)

	for in, want := range map[string]string{
		"http_port80":  "httpport80",
		"HTTP-Port-80": "httpport80",
		"web":          "web00000",
		"__--__":       "interface",
		"a_very_long_interface_identifier_beyond_limits": "averylonginterfaceidentifierbeyo",
	} {
		got := accessPointInterfaceId(in)
		if got != want {
			t.Errorf("accessPointInterfaceId(%q) = %q, want %q", in, got, want)
		}
		if !crdPattern.MatchString(got) || !ewbiPattern.MatchString(got) {
			t.Errorf("accessPointInterfaceId(%q) = %q does not satisfy both patterns", in, got)
		}
	}
}
