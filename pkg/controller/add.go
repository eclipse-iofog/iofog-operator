package controller

import (
	appv1 "github.com/eclipse-iofog/iofog-operator/pkg/controller/app"
	kogv1 "github.com/eclipse-iofog/iofog-operator/pkg/controller/kog"
	appv2 "github.com/eclipse-iofog/iofog-operator/v2/pkg/controller/app"
	kogv2 "github.com/eclipse-iofog/iofog-operator/v2/pkg/controller/kog"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, appv1.Add)
	AddToManagerFuncs = append(AddToManagerFuncs, kogv1.Add)
	AddToManagerFuncs = append(AddToManagerFuncs, appv2.Add)
	AddToManagerFuncs = append(AddToManagerFuncs, kogv2.Add)
}
