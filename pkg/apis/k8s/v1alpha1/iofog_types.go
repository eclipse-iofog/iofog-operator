package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// IOFogSpec defines the desired state of IOFog
type IOFogSpec struct {
	Replicas      int32           `json:"replicas"`
	Microservices []Microservices `json:"microservices"`
}

type Microservices struct {
	CatalogItemId  int              `json:"catalog-item-id"`
	Cmd            []string         `json:"cmd"`
	Config         string           `json:"config"`
	Env            []Env            `json:"env"`
	HostAccess     bool             `json:"host-access"`
	LogSize        int              `json:"logSize"`
	Name           string           `json:"name"`
	Ports          []Ports          `json:"ports"`
	Routes         []string         `json:"routes"`
	VolumeMappings []VolumeMappings `json:"volume-mappings"`
}

type Ports struct {
	Internal   int  `json:"internal"`
	External   int  `json:"external"`
	PublicMode bool `json:"publicMode"`
}

type VolumeMappings struct {
	HostDestination      string `json:"hostDestination"`
	ContainerDestination string `json:"containerDestination"`
	AccessMode           string `json:"accessMode"`
}

// IOFogStatus defines the observed state of IOFog
type IOFogStatus struct {
	Replicas      int32    `json:"replicas"`
	LabelSelector string   `json:"labelSelector"`
	PodNames      []string `json:"podNames"`
}

type Env struct {
	Key   string `json:"key"`
	Value string `json:"value"`
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
