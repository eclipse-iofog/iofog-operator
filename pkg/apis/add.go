package apis

import (
	v1 "github.com/eclipse-iofog/iofog-operator/pkg/apis/iofog/v1"
	v2 "github.com/eclipse-iofog/iofog-operator/v2/pkg/apis/iofog"
)

func init() {
	// Register the types with the Scheme so the components can map objects to GroupVersionKinds and back
	AddToSchemes = append(AddToSchemes, v1.SchemeBuilder.AddToScheme)
	AddToSchemes = append(AddToSchemes, v2.SchemeBuilder.AddToScheme)
}
