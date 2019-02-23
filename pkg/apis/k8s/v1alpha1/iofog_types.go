package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// IOFogSpec defines the desired state of IOFog
type IOFogSpec struct {
	Microservices []Microservices `json:"microservices"`
}

type Microservices struct {
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	Config         string   `json:"config"`
	VolumeMappings []string `json:"volume-mappings"`
	HostAccess     bool     `json:"host-access"`
	STrace         bool     `json:"strace"`
	Ports          []Ports  `json:"ports"`
	Routes         []string `json:"routes"`
}

type Ports struct {
	Internal int
	External int
	public   bool
}

// IOFogStatus defines the observed state of IOFog
type IOFogStatus struct {
	Replicas      int32  `json:"replicas"`
	LabelSelector string `json:"labelSelector"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// IOFog is the Schema for the iofogs API
// +k8s:openapi-gen=true
type IOFog struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   IOFogSpec   `json:"spec,omitempty"`
	Status IOFogStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// IOFogList contains a list of IOFog
type IOFogList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []IOFog `json:"items"`
}

func init() {
	SchemeBuilder.Register(&IOFog{}, &IOFogList{})
}
