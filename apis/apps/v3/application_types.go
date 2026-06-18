package v3

import (
	"github.com/eclipse-iofog/iofog-go-sdk/v3/pkg/apps"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ApplicationPhase is the high-level lifecycle phase of an Application.
type ApplicationPhase string

const (
	ApplicationPhasePending ApplicationPhase = "Pending"
	ApplicationPhaseSyncing ApplicationPhase = "Syncing"
	ApplicationPhaseReady   ApplicationPhase = "Ready"
	ApplicationPhaseFailed  ApplicationPhase = "Failed"
)

// ApplicationSpec defines the desired state of Application.
type ApplicationSpec struct {
	// Microservices lists microservices composing the application.
	// +optional
	Microservices []apps.Microservice `json:"microservices,omitempty"`
	// NatsConfig holds application-level NATS access configuration.
	// +optional
	NatsConfig *apps.ApplicationNatsConfig `json:"natsConfig,omitempty"`
	// Template deploys an application from a named template and variables.
	// +optional
	Template *ApplicationTemplate `json:"template,omitempty"`
}

// ApplicationTemplate deploys an application from a named template (SDK-aligned JSON shape).
type ApplicationTemplate struct {
	Name        string                   `json:"name,omitempty"`
	Description string                   `json:"description,omitempty"`
	Variables   []TemplateVariable       `json:"variables,omitempty"`
	Application *ApplicationTemplateInfo `json:"application,omitempty"`
}

// TemplateVariable is a template variable key/value pair.
type TemplateVariable struct {
	Key          string                `json:"key"`
	Description  string                `json:"description,omitempty"`
	DefaultValue *apiextensionsv1.JSON `json:"defaultValue,omitempty"`
	Value        *apiextensionsv1.JSON `json:"value,omitempty"`
}

// ApplicationTemplateInfo holds microservices and NATS config for a template.
type ApplicationTemplateInfo struct {
	Microservices []apps.Microservice         `json:"microservices"`
	NatsConfig    *apps.ApplicationNatsConfig `json:"natsConfig,omitempty"`
}

// ApplicationStatus defines the observed state of Application.
type ApplicationStatus struct {
	// Phase summarizes application reconciliation (Pending, Syncing, Ready, Failed).
	// +optional
	Phase ApplicationPhase `json:"phase,omitempty"`
	// ApplicationStatus mirrors the application as reported by the Controller API.
	// +optional
	ApplicationStatus *ObservedApplicationStatus `json:"applicationStatus,omitempty"`
	// LastSyncTime is when the operator last successfully synced with the Controller.
	// +optional
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`
	// Conditions represent the latest available observations of the Application state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Application is the Schema for the applications API.
type Application struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ApplicationSpec   `json:"spec,omitempty"`
	Status ApplicationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ApplicationList contains a list of Application.
type ApplicationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Application `json:"items"`
}

func init() { //nolint:gochecknoinits
	SchemeBuilder.Register(&Application{}, &ApplicationList{})
}
