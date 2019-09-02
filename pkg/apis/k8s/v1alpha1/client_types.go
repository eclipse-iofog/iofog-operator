package v1alpha1

import (
	extsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NewIofogCRD(name string) *extsv1.CustomResourceDefinition {
	labelSelectorPath := ".status.labelSelector"
	return &extsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: extsv1.CustomResourceDefinitionSpec{
			Group: "k8s.iofog.org",
			Names: extsv1.CustomResourceDefinitionNames{
				Kind:     "IOFog",
				ListKind: "IOFogList",
				Plural:   "iofogs",
				Singular: "iofog",
			},
			Scope:   extsv1.ResourceScope("Namespaced"),
			Version: "v1alpha1",
			Subresources: &extsv1.CustomResourceSubresources{
				Status: &extsv1.CustomResourceSubresourceStatus{},
				Scale: &extsv1.CustomResourceSubresourceScale{
					SpecReplicasPath:   ".spec.replicas",
					StatusReplicasPath: ".status.replicas",
					LabelSelectorPath:  &labelSelectorPath,
				},
			},
		},
	}
}