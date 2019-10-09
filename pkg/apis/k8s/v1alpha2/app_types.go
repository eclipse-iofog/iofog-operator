package v1alpha2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// AppSpec defines the desired state of App
// +k8s:openapi-gen=true
type AppSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
	Microservices []Microservice `json:"microservices"`
	Routes        []Route        `json:"routes"`
	Replicas      int32          `json:"replicas"`
}

// AppStatus defines the observed state of App
// +k8s:openapi-gen=true
type AppStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
	Replicas      int32    `json:"replicas"`
	LabelSelector string   `json:"labelSelector"`
	PodNames      []string `json:"podNames"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// App is the Schema for the apps API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
type App struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AppSpec   `json:"spec,omitempty"`
	Status AppStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AppList contains a list of Apps
type AppList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []App `json:"items"`
}

func init() {
	SchemeBuilder.Register(&App{}, &AppList{})
}

// MicroserviceImages contains information about the images for a microservice
type MicroserviceImages struct {
	CatalogID int    `json:"catalogId"`
	X86       string `json:"x86"`
	ARM       string `json:"arm"`
	Registry  string `json:"registry"`
}

// MicroserviceAgent contains information about required agent configuration for a microservice
type MicroserviceAgent struct {
	Name   string             `json:"name"`
	Config AgentConfiguration `json:"config"`
}

// Microservice contains information for configuring a microservice
type Microservice struct {
	Name           string                      `json:"name"`
	Agent          MicroserviceAgent           `json:"agent"`
	Images         MicroserviceImages          `json:"images"`
	Config         JSON                        `json:"config"`
	RootHostAccess bool                        `json:"rootHostAccess"`
	Ports          []MicroservicePortMapping   `json:"ports"`
	Volumes        []MicroserviceVolumeMapping `json:"volumes"`
	Env            []MicroserviceEnvironment   `json:"env"`
	Routes         []string                    `json:"routes,omitempty"`
}

type JSON map[string]interface{}

func (j JSON) DeepCopy() JSON {
	copy := make(JSON)
	deepCopyJSON(j, copy)
	return copy
}

func deepCopyJSON(src JSON, dest JSON) {
	for key, value := range src {
		switch src[key].(type) {
		case JSON:
			dest[key] = JSON{}
			deepCopyJSON(src[key].(JSON), dest[key].(JSON))
		default:
			dest[key] = value
		}
	}
}

// Route contains information about a route from one microservice to another
type Route struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type AgentConfiguration struct {
	DockerURL                 *string  `json:"dockerUrl,omitempty"`
	DiskLimit                 *int64   `json:"diskLimit,omitempty"`
	DiskDirectory             *string  `json:"diskDirectory,omitempty"`
	MemoryLimit               *int64   `json:"memoryLimit,omitempty"`
	CPULimit                  *int64   `json:"cpuLimit,omitempty"`
	LogLimit                  *int64   `json:"logLimit,omitempty"`
	LogDirectory              *string  `json:"logDirectory,omitempty"`
	LogFileCount              *int64   `json:"logFileCount,omitempty"`
	StatusFrequency           *float64 `json:"statusFrequency,omitempty"`
	ChangeFrequency           *float64 `json:"changeFrequency,omitempty"`
	DeviceScanFrequency       *float64 `json:"deviceScanFrequency,omitempty"`
	BluetoothEnabled          *bool    `json:"bluetoothEnabled,omitempty"`
	WatchdogEnabled           *bool    `json:"watchdogEnabled,omitempty"`
	AbstractedHardwareEnabled *bool    `json:"abstractedHardwareEnabled,omitempty"`
}

type MicroservicePortMapping struct {
	Internal   int  `json:"internal"`
	External   int  `json:"external"`
	PublicMode bool `json:"publicMode"`
}

type MicroserviceVolumeMapping struct {
	HostDestination      string `json:"hostDestination"`
	ContainerDestination string `json:"containerDestination"`
	AccessMode           string `json:"accessMode"`
}

type MicroserviceEnvironment struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}
