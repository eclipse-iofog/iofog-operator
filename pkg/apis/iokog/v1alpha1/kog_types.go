package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// KogSpec defines the desired state of Kog
// +k8s:openapi-gen=true
type KogSpec struct {
	IofogUser       IofogUser `json:"iofogUser"`
	ControllerCount int       `json:"controllerCount"`
	ConnectorCount  int       `json:"connectorCount"`
	ControllerImage string    `json:"controllerImage"`
	ConnectorImage  string    `json:"connectorImage"`
	OperatorImage   string    `json:"operatorImage"`
	KubeletImage    string    `json:"kubeletImage"`
	Database        Database  `json:"database"`
}

type IofogUser struct {
	Name     string `json:"name"`
	Surname  string `json:"surname"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type Database struct {
	Provider     string `json:"provider"`
	Host         string `json:"host"`
	Port         int    `json:"port"`
	User         string `json:"user"`
	Password     string `json:"password"`
	DatabaseName string `json:"databaseName"`
}

// KogStatus defines the observed state of Kog
// +k8s:openapi-gen=true
type KogStatus struct {
	ControllerPods  []string `json:"controllerPods"`
	ConnectorPods   []string `json:"connectorPods"`
	ControllerCount int      `json:"controllerCount"`
	ConnectorCount  int      `json:"connectorCount"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Kog is the Schema for the kogs API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
type Kog struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KogSpec   `json:"spec,omitempty"`
	Status KogStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KogList contains a list of Kog
type KogList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Kog `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Kog{}, &KogList{})
}
