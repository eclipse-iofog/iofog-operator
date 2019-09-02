package v1alpha2

import (
	extsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NewKogCRD(namespace string) *extsv1.CustomResourceDefinition {
	return &extsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
				Name: "kogs.k8s.iofog.org",
			},
			Spec: extsv1.CustomResourceDefinitionSpec{
				Group:   "k8.iofog.org",
				Version: "v1alpha2",
				Versions: []extsv1.CustomResourceDefinitionVersion{{
					Name:    "v1alpha2",
					Served:  true,
					Storage: true,
				}},
				Names: extsv1.CustomResourceDefinitionNames{
					Plural:   "kogs",
					Singular: "kog",
					Kind:     "Kog",
				},
				Scope: extsv1.NamespaceScoped,
				Subresources: &extsv1.CustomResourceSubresources{
					Status: &extsv1.CustomResourceSubresourceStatus{},
				},
				Validation: &extsv1.CustomResourceValidation{
					OpenAPIV3Schema: &extsv1.JSONSchemaProps{
						Properties: map[string]extsv1.JSONSchemaProps{
							"status" : {
								Properties: map[string]extsv1.JSONSchemaProps{
									"controllerPods": {
										Type: "array",
										Items: &extsv1.JSONSchemaPropsOrArray{
											Schema: &extsv1.JSONSchemaProps{
												Type: "string",
											},
										},
									},
								},
							},
							"spec": {
								Properties: map[string]extsv1.JSONSchemaProps{
									"database": {
										Properties: map[string]extsv1.JSONSchemaProps{
											"provider": {
												Type: "string",
											},
											"host": {
												Type: "string",
											},
											"port": {
												Type: "string",
											},
											"user": {
												Type: "string",
											},
											"password": {
												Type: "string",
											},
											"databaseName": {
												Type: "string",
											},
										},
										Required: []string{
											"provider",
											"host",
											"port",
											"user",
											"password",
											"databaseName",
										},
									},
									"iofogUser": {
										Properties: map[string]extsv1.JSONSchemaProps{
											"name": {
												Type: "string",
											},
											"surname": {
												Type: "string",
											},
											"email": {
												Type: "string",
											},
											"password": {
												Type: "string",
											},
										},
										Required: []string{
											"name",
											"surname",
											"email",
											"password",
										},
									},
									"controllerReplicaCount": {
										Type:    "integer",
										Format: "int32",
									},
									"connectorCount": {
										Type:    "integer",
										Format: "int32",
									},
									"controllerImage": {
										Type: "string",
									},
									"connectorImage": {
										Type: "string",
									},
									"kubeletImage": {
										Type: "string",
									},
								},
								Required: []string{
									"controllerReplicaCount",
									"connectorCount",
									"iofogUser",
								},
							},
						},
					},
				},
			},
	}
}