/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v3

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	cond "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	conditionReady     = "ready"
	conditionDeploying = "deploying"
	conditionUpdating  = "updating"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ControlPlaneSpec defines the desired state of ControlPlane.
type ControlPlaneSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
	// Auth contains Keycloak Client Configuration of Controller and ECN Viewer
	Auth Auth `json:"auth"`
	// Database for ioFog Controller
	Database Database `json:"database"`
	// Ingresses allow Router and Port Manager to configure endpoint addresses correctly
	Ingresses Ingresses `json:"ingresses,omitempty"`
	// Services should be LoadBalancer unless Ingress is being configured
	Services Services `json:"services,omitempty"`
	// Replicas of ioFog Controller should be 1 unless an external DB is configured
	Replicas Replicas `json:"replicas,omitempty"`
	// Images specifies which containers to run for each component of the ControlPlane
	Images Images `json:"images,omitempty"`
	// Controller contains runtime configuration for ioFog Controller
	Controller Controller `json:"controller,omitempty"`
	// // Router contains runtime configuration for ioFog Router
	// Router Router `json:"router,omitempty"`
	// Events contains runtime configuration for ioFog Controller events
	Events Events `json:"events,omitempty"`
	// Nats contains NATS hub configuration (StatefulSet, JetStream, etc.). When omitted, NATS is enabled with defaults.
	Nats *Nats `json:"nats,omitempty"`
	// Vault is optional. When set, the Controller uses the configured vault provider for secrets. Operator creates a Secret from provider-specific config and injects env vars.
	Vault *Vault `json:"vault,omitempty"`
}

// Vault configures vault integration for the Controller. Optional; when omitted, no vault env vars are set.
// Provide only the block for the selected provider (hashicorp, aws, azure, or google). The operator creates a Secret from it and injects env vars.
type Vault struct {
	// +optional
	Enabled *bool `json:"enabled,omitempty"`
	// Provider: hashicorp, openbao, vault, aws, aws-secrets-manager, azure, azure-key-vault, google, google-secret-manager.
	// +optional
	Provider string `json:"provider,omitempty"`
	// BasePath for secrets in vault; $namespace is replaced with the ControlPlane namespace (e.g. pot/$namespace/secrets).
	// +optional
	BasePath string `json:"basePath,omitempty"`
	// Hashicorp (and openbao/vault) provider config. Set when provider is hashicorp, openbao, or vault.
	// +optional
	Hashicorp *VaultHashicorp `json:"hashicorp,omitempty"`
	// Aws provider config. Set when provider is aws or aws-secrets-manager.
	// +optional
	Aws *VaultAws `json:"aws,omitempty"`
	// Azure provider config. Set when provider is azure or azure-key-vault.
	// +optional
	Azure *VaultAzure `json:"azure,omitempty"`
	// Google provider config. Set when provider is google or google-secret-manager.
	// +optional
	Google *VaultGoogle `json:"google,omitempty"`
}

// VaultHashicorp holds HashiCorp Vault (or OpenBao) configuration. Operator stores these in a Secret and maps to VAULT_HASHICORP_* env vars.
type VaultHashicorp struct {
	Address string `json:"address,omitempty"`
	Token   string `json:"token,omitempty"`
	Mount   string `json:"mount,omitempty"`
}

// VaultAws holds AWS Secrets Manager configuration. Operator stores these in a Secret and maps to VAULT_AWS_* env vars.
type VaultAws struct {
	Region      string `json:"region,omitempty"`
	AccessKeyId string `json:"accessKeyId,omitempty"`
	AccessKey   string `json:"accessKey,omitempty"`
}

// VaultAzure holds Azure Key Vault configuration. Operator stores these in a Secret and maps to VAULT_AZURE_* env vars.
type VaultAzure struct {
	URL          string `json:"url,omitempty"`
	TenantId     string `json:"tenantId,omitempty"`
	ClientId     string `json:"clientId,omitempty"`
	ClientSecret string `json:"clientSecret,omitempty"`
}

// VaultGoogle holds Google Secret Manager configuration. Operator stores these in a Secret and maps to VAULT_GOOGLE_* env vars.
type VaultGoogle struct {
	ProjectId   string `json:"projectId,omitempty"`
	Credentials string `json:"credentials,omitempty"` // path to service account key file or JSON content
}

type Replicas struct {
	Controller int32 `json:"controller,omitempty"`
	// Nats is the number of NATS server replicas (default 2, min 2 when NATS enabled).
	// +kubebuilder:validation:Minimum=2
	Nats int32 `json:"nats,omitempty"`
}

type Services struct {
	Controller Service `json:"controller,omitempty"`
	Router     Service `json:"router,omitempty"`
	Nats       Service `json:"nats,omitempty"`
	NatsServer Service `json:"natsServer,omitempty"`
}

type Service struct {
	Type        string            `json:"type,omitempty"`
	Address     string            `json:"address,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	// ExternalTrafficPolicy for LoadBalancer/NodePort: "Local" or "Cluster". When omitted, LoadBalancer defaults to Local, others to Cluster. Use "Cluster" on local K8s (e.g. OrbStack) if Local causes issues.
	ExternalTrafficPolicy string `json:"externalTrafficPolicy,omitempty"`
}

type Images struct {
	PullSecret string `json:"pullSecret,omitempty"`
	Controller string `json:"controller,omitempty"`
	Router     string `json:"router,omitempty"`
	Nats       string `json:"nats,omitempty"`
}

type Auth struct {
	URL              string `json:"url"`
	Realm            string `json:"realm"`
	SSL              string `json:"ssl"`
	RealmKey         string `json:"realmKey"`
	ControllerClient string `json:"controllerClient"`
	ControllerSecret string `json:"controllerSecret"`
	ViewerClient     string `json:"viewerClient"`
}

type Events struct {
	AuditEnabled     *bool `json:"auditEnabled,omitempty"`
	RetentionDays    int   `json:"retentionDays,omitempty"`
	CleanupInterval  int   `json:"cleanupInterval,omitempty"`
	CaptureIpAddress *bool `json:"captureIpAddress,omitempty"`
}

type Database struct {
	Provider     string  `json:"provider"`
	Host         string  `json:"host"`
	Port         int     `json:"port"`
	User         string  `json:"user"`
	Password     string  `json:"password"`
	DatabaseName string  `json:"databaseName"`
	SSL          *bool   `json:"ssl,omitempty"`
	CA           *string `json:"ca,omitempty"`
}

type User struct {
	Name            string `json:"name"`
	Surname         string `json:"surname"`
	Email           string `json:"email"`
	Password        string `json:"password"`
	SubscriptionKey string `json:"subscriptionKey"`
}

type RouterIngress struct {
	Address      string `json:"address,omitempty"`
	MessagePort  int    `json:"messagePort,omitempty"`
	InteriorPort int    `json:"interiorPort,omitempty"`
	EdgePort     int    `json:"edgePort,omitempty"`
}

type ControllerIngress struct {
	Annotations      map[string]string `json:"annotations,omitempty"`
	IngressClassName string            `json:"ingressClassName,omitempty"`
	Host             string            `json:"host,omitempty"`
	SecretName       string            `json:"secretName,omitempty"`
}

type Ingress struct {
	Address string `json:"address,omitempty"`
}

// NatsIngress specifies the external address and ports for NATS hub registration (required when using ingress).
// Ports are optional and default to: server 4222, cluster 6222, leaf 7422, mqtt 8883, http 8222.
type NatsIngress struct {
	Address     string `json:"address,omitempty"`
	ServerPort  int    `json:"serverPort,omitempty"`
	ClusterPort int    `json:"clusterPort,omitempty"`
	LeafPort    int    `json:"leafPort,omitempty"`
	MqttPort    int    `json:"mqttPort,omitempty"`
	HttpPort    int    `json:"httpPort,omitempty"`
}

type Ingresses struct {
	Controller ControllerIngress `json:"controller,omitempty"`
	Router     RouterIngress     `json:"router,omitempty"`
	Nats       NatsIngress       `json:"nats,omitempty"`
}

type Controller struct {
	PidBaseDir    string `json:"pidBaseDir,omitempty"`
	EcnViewerPort int    `json:"ecnViewerPort,omitempty"`
	EcnViewerURL  string `json:"ecnViewerUrl,omitempty"`
	ECNName       string `json:"ecn,omitempty"`
	Https         *bool  `json:"https,omitempty"`
	SecretName    string `json:"secretName,omitempty"`
	LogLevel      string `json:"logLevel,omitempty"`
}

// type Router struct {
// 	HA *bool `json:"ha,omitempty"`
// }

// NatsJetStream configures JetStream storage.
type NatsJetStream struct {
	// StorageSize is used for the PVC size and max_file_store in server.conf (default 10Gi).
	StorageSize string `json:"storageSize,omitempty"`
	// MemoryStoreSize is used for max_memory_store in server.conf only (default 1Gi).
	MemoryStoreSize string `json:"memoryStoreSize,omitempty"`
	// StorageClassName for the JetStream PVC (optional).
	StorageClassName string `json:"storageClassName,omitempty"`
}

// Nats configures the NATS hub (StatefulSet, JetStream, services).
// When Enabled is omitted, NATS is enabled. When false, no NATS resources are created and no hub is registered.
type Nats struct {
	// Enabled toggles NATS deployment. When omitted, treated as true.
	Enabled *bool `json:"enabled,omitempty"`
	// JetStream storage and memory limits.
	JetStream NatsJetStream `json:"jetStream,omitempty"`
}

// ControlPlaneStatus defines the observed state of ControlPlane.
type ControlPlaneStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Conditions []metav1.Condition `json:"conditions"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// ControlPlane is the Schema for the controlplanes API.
type ControlPlane struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ControlPlaneSpec   `json:"spec,omitempty"`
	Status ControlPlaneStatus `json:"status,omitempty"`
}

func (cp *ControlPlane) setCondition(conditionType string, log *logr.Logger) {
	now := metav1.NewTime(time.Now())
	// Clear all
	for idx := range cp.Status.Conditions {
		condition := &cp.Status.Conditions[idx]
		// Migration: all lower case, no spaces, no -
		condition.Reason = strings.ToLower(condition.Reason)
		condition.Reason = strings.Replace(condition.Reason, " ", "_", -1)
		condition.Reason = strings.Replace(condition.Reason, "-", "_", -1)

		if condition.Status == metav1.ConditionTrue {
			condition.Status = metav1.ConditionFalse
			condition.Reason = fmt.Sprintf("transition_to_%s", conditionType)
			condition.LastTransitionTime = now
			condition.ObservedGeneration = cp.ObjectMeta.Generation
		}
	}
	// Add / overwrite
	newCondition := metav1.Condition{
		Type:               conditionType,
		Status:             metav1.ConditionTrue,
		Reason:             "initial_status",
		LastTransitionTime: now,
		ObservedGeneration: cp.ObjectMeta.Generation,
	}

	if log != nil {
		log.Info(fmt.Sprintf("reconcileDeploying() ControlPlane %s setCondition %v -- Existing conditions %v", cp.Name, newCondition, cp.Status.Conditions))
	}

	cond.SetStatusCondition(&cp.Status.Conditions, newCondition)
}

func (cp *ControlPlane) SetConditionDeploying(log *logr.Logger) {
	cp.setCondition(conditionDeploying, log)
}

func (cp *ControlPlane) SetConditionReady(log *logr.Logger) {
	cp.setCondition(conditionReady, log)
}

func (cp *ControlPlane) SetConditionUpdating(log *logr.Logger) {
	cp.setCondition(conditionUpdating, log)
}

func (cp *ControlPlane) GetCondition() string {
	state := conditionDeploying

	for _, condition := range cp.Status.Conditions {
		if condition.Status == metav1.ConditionTrue {
			if condition.ObservedGeneration == cp.ObjectMeta.Generation {
				state = condition.Type
			} else {
				state = conditionUpdating
			}
			break
		}
	}

	return state
}

func (cp *ControlPlane) IsReady() bool {
	return cp.GetCondition() == conditionReady
}

func (cp *ControlPlane) IsDeploying() bool {
	return cp.GetCondition() == conditionDeploying
}

func (cp *ControlPlane) IsUpdating() bool {
	return cp.GetCondition() == conditionUpdating
}

// +kubebuilder:object:root=true

// ControlPlaneList contains a list of ControlPlane.
type ControlPlaneList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ControlPlane `json:"items"`
}

func init() { //nolint:gochecknoinits
	SchemeBuilder.Register(&ControlPlane{}, &ControlPlaneList{})
}
