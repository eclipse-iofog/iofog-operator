package controller

import (
	"github.com/eclipse-iofog/iofog-operator/v2/pkg/controller/app"
	"github.com/eclipse-iofog/iofog-operator/v2/pkg/controller/controlplane"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, app.Add, controlplane.Add)
}
