package controller

import (
	"github.com/eclipse-iofog/iofog-operator/pkg/controller/iofog"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, iofog.Add)
}
