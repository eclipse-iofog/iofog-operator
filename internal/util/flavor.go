package util

import "fmt"

// componentLabelDomain is set at link time, e.g. LDFLAGS += -X ...componentLabelDomain=iofog.org
var componentLabelDomain = "datasance.com" //nolint:gochecknoglobals

// ComponentLabelKey returns the mirror-specific component label key (e.g. iofog.org/component).
func ComponentLabelKey() string {
	return fmt.Sprintf("%s/component", componentLabelDomain)
}

// ComponentLabel returns a single-entry map for the flavor component label.
func ComponentLabel(value string) map[string]string {
	return map[string]string{ComponentLabelKey(): value}
}
