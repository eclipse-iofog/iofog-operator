package util

import "fmt"

// These values are set by the linker, e.g. "LDFLAGS += -X $(PREFIX).controllerTag=v3.0.0-beta1".
var (
	repo          = "undefined" //nolint:gochecknoglobals
	controllerTag = "undefined" //nolint:gochecknoglobals
	routerTag     = "undefined" //nolint:gochecknoglobals
	natsTag       = "undefined" //nolint:gochecknoglobals
)

const (
	controllerImage = "controller"
	routerImage     = "router"
	natsImage       = "nats"
)

func GetControllerImage() string {
	return fmt.Sprintf("%s/%s:%s", repo, controllerImage, controllerTag)
}
func GetRouterImage() string { return fmt.Sprintf("%s/%s:%s", repo, routerImage, routerTag) }
func GetNatsImage() string   { return fmt.Sprintf("%s/%s:%s", repo, natsImage, natsTag) }
