package controllers

import (
	"context"
	b64 "encoding/base64"
	"encoding/json"
	"fmt"
	"maps"
	"strings"

	iofogclient "github.com/eclipse-iofog/iofog-go-sdk/v3/pkg/client"
	cpv3 "github.com/eclipse-iofog/iofog-operator/v3/apis/controlplanes/v3"
	"github.com/eclipse-iofog/iofog-operator/v3/internal/auth/controllerlogin"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *ControlPlaneReconciler) deploymentExists(ctx context.Context, namespace, name string) (bool, error) {
	key := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}
	dep := &appsv1.Deployment{}

	err := r.Client.Get(ctx, key, dep)
	if err == nil {
		return true, nil
	}

	if k8serrors.IsNotFound(err) {
		return false, nil
	}

	return false, err
}

func (r *ControlPlaneReconciler) restartPodsForDeployment(ctx context.Context, deploymentName, namespace string) error {
	// Check if this resource already exists
	found := &appsv1.Deployment{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: deploymentName, Namespace: namespace}, found); err != nil {
		return err
	}

	originValue := int32(1)
	if found.Spec.Replicas == nil {
		found.Spec.Replicas = &originValue
	}

	if err := r.Client.Update(ctx, found); err != nil {
		return err
	}

	return r.Client.Update(ctx, found)
}

func (r *ControlPlaneReconciler) createDeployment(ctx context.Context, ms *microservice) error {
	dep := newDeployment(r.cp.ObjectMeta.Namespace, r.cp.Name, ms)
	// Set ControlPlane instance as the owner and controller
	if err := controllerutil.SetControllerReference(&r.cp, dep, r.Scheme); err != nil {
		return err
	}

	// Check if this resource already exists
	found := &appsv1.Deployment{}

	err := r.Client.Get(ctx, types.NamespacedName{Name: dep.Name, Namespace: dep.Namespace}, found)
	if err != nil && k8serrors.IsNotFound(err) {
		r.log.Info("Creating a new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)

		err = r.Client.Create(ctx, dep)
		if err != nil {
			return err
		}

		// Resource created successfully - don't requeue
		return nil
	} else if err != nil {
		return err
	}

	// Resource already exists - update it
	r.log.Info("Updating existing Deployment", "Deployment.Namespace", found.Namespace, "Deployment.Name", found.Name)

	if err := r.Client.Update(ctx, dep); err != nil {
		return err
	}

	return nil
}

func (r *ControlPlaneReconciler) createStatefulSet(ctx context.Context, ms *microservice) error {
	st := newStatefulSet(r.cp.ObjectMeta.Namespace, r.cp.Name, ms)
	if err := controllerutil.SetControllerReference(&r.cp, st, r.Scheme); err != nil {
		return err
	}
	found := &appsv1.StatefulSet{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: st.Name, Namespace: st.Namespace}, found)
	if err != nil && k8serrors.IsNotFound(err) {
		r.log.Info("Creating a new StatefulSet", "StatefulSet.Namespace", st.Namespace, "StatefulSet.Name", st.Name)
		return r.Client.Create(ctx, st)
	}
	if err != nil {
		return err
	}
	r.log.Info("Updating existing StatefulSet", "StatefulSet.Namespace", found.Namespace, "StatefulSet.Name", found.Name)

	if err := r.Client.Update(ctx, st); err != nil {
		return err
	}
	return nil
}

func (r *ControlPlaneReconciler) createPersistentVolumeClaims(ctx context.Context, ms *microservice) error {
	for i := range ms.volumes {
		if ms.volumes[i].VolumeSource.PersistentVolumeClaim == nil {
			continue
		}

		storageSize, err := resource.ParseQuantity("1Gi")
		if err != nil {
			return err
		}

		pvc := corev1.PersistentVolumeClaim{
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{
					corev1.ReadWriteOnce,
				},
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						"storage": storageSize,
					},
				},
			},
		}

		pvc.ObjectMeta.Name = ms.volumes[i].Name
		pvc.ObjectMeta.Namespace = r.cp.Namespace
		pvc.ObjectMeta.Labels = getStandardLabels(getComponentFromMicroservice(ms), r.cp.Name)
		// Set ControlPlane instance as the owner and controller
		if err := controllerutil.SetControllerReference(&r.cp, &pvc, r.Scheme); err != nil {
			return err
		}

		// Check if this resource already exists
		found := &corev1.PersistentVolumeClaim{}

		err = r.Client.Get(ctx, types.NamespacedName{Name: pvc.Name, Namespace: pvc.Namespace}, found)
		if err != nil && k8serrors.IsNotFound(err) {
			r.log.Info("Creating a new PersistentVolumeClaim", "PersistentVolumeClaim.Namespace", pvc.Namespace, "PersistentVolumeClaim.Name", pvc.Name)

			err = r.Client.Create(ctx, &pvc)
			if err != nil {
				return err
			}

			// Resource created successfully - don't requeue
			continue
		} else if err != nil {
			return err
		}

		// Resource already exists - don't requeue
		r.log.Info("Skip reconcile: Secret already exists", "Secret.Namespace", found.Namespace, "Secret.Name", found.Name)
	}

	return nil
}

func (r *ControlPlaneReconciler) createSecrets(ctx context.Context, ms *microservice) error {
	return r.createOrUpdateSecrets(ctx, ms, false)
}

func (r *ControlPlaneReconciler) createOrUpdateSecrets(ctx context.Context, ms *microservice, update bool) error {
	defer func() {
		if recoverResult := recover(); recoverResult != nil {
			r.log.Info(fmt.Sprintf("Recover result %v for creating secrets for Controlplane %s", recoverResult, r.cp.Name))
		}
	}()

	stdLabels := getStandardLabels(getComponentFromMicroservice(ms), r.cp.Name)
	for i := range ms.secrets {
		secret := &ms.secrets[i]
		secret.Labels = mergeLabels(stdLabels, secret.Labels)
		r.log.Info(fmt.Sprintf("Creating secret %s", secret.ObjectMeta.Name))
		// Set ControlPlane instance as the owner and controller
		r.log.Info(fmt.Sprintf("Setting owner reference for secret %s", secret.ObjectMeta.Name))

		if err := controllerutil.SetControllerReference(&r.cp, secret, r.Scheme); err != nil {
			r.log.Info(fmt.Sprintf("Failed to set owner reference for secret %s: %v", secret.ObjectMeta.Name, err))

			return err
		}

		// Check if this resource already exists
		r.log.Info(fmt.Sprintf("Checking if secret %s exists", secret.ObjectMeta.Name))

		found := &corev1.Secret{}

		err := r.Client.Get(ctx, types.NamespacedName{Name: secret.Name, Namespace: secret.Namespace}, found)
		r.log.Info(fmt.Sprintf("secret %s: Exists: %s Error: %v", secret.ObjectMeta.Name, found.Name, err))

		if err != nil && k8serrors.IsNotFound(err) {
			r.log.Info("Creating a new Secret", "Secret.Namespace", secret.Namespace, "Service.Name", secret.Name)

			err = r.Client.Create(ctx, secret)
			if err != nil {
				return err
			}

			// Resource created successfully - don't requeue
			continue
		} else if err != nil {
			r.log.Info(fmt.Sprintf("Failed with error %v for secret %s:", err, secret.ObjectMeta.Name))

			return err
		}

		// Resource already exists - don't requeue
		if update {
			r.log.Info("Updating secret...", "Secret.Namespace", found.Namespace, "Secret.Name", found.Name)

			err = r.Client.Update(ctx, secret)
			if err != nil {
				return err
			}
		} else {
			r.log.Info("Skip reconciliation: Secret already exists.", "Secret.Namespace", found.Namespace, "Secret.Name", found.Name)
		}
	}

	r.log.Info(fmt.Sprintf("Done Creating secrets for router reconcile for Controlplane %s", r.cp.Name))

	return nil
}

func (r *ControlPlaneReconciler) createService(ctx context.Context, ms *microservice) error {
	svcs := newServices(r.cp.ObjectMeta.Namespace, r.cp.Name, ms)
	for _, svc := range svcs {
		// Set ControlPlane instance as the owner and controller
		if err := controllerutil.SetControllerReference(&r.cp, svc, r.Scheme); err != nil {
			return err
		}

		// Check if this resource already exists
		found := &corev1.Service{}

		err := r.Client.Get(ctx, types.NamespacedName{Name: svc.Name, Namespace: svc.Namespace}, found)
		if err != nil && k8serrors.IsNotFound(err) {
			r.log.Info("Creating a new Service", "Service.Namespace", svc.Namespace, "Service.Name", svc.Name)

			err = r.Client.Create(ctx, svc)
			if err != nil {
				return err
			}

			// Resource created successfully - don't requeue
			continue
		} else if err != nil {
			return err
		}

		if serviceNeedsPatch(found, svc) {
			r.log.Info("Updating existing Service", "Service.Namespace", found.Namespace, "Service.Name", found.Name)
			applyServicePatch(found, svc)
			if err := r.Client.Update(ctx, found); err != nil {
				return err
			}
			continue
		}

		r.log.Info("Skip reconcile: Service already exists", "Service.Namespace", found.Namespace, "Service.Name", found.Name)
	}

	return nil
}

func serviceNeedsPatch(existing, desired *corev1.Service) bool {
	if existing.Spec.Type != desired.Spec.Type {
		return true
	}
	if existing.Spec.ExternalTrafficPolicy != desired.Spec.ExternalTrafficPolicy {
		return true
	}
	return !maps.Equal(existing.Annotations, desired.Annotations)
}

func applyServicePatch(existing, desired *corev1.Service) {
	existing.Annotations = desired.Annotations
	existing.Spec.Type = desired.Spec.Type
	existing.Spec.ExternalTrafficPolicy = desired.Spec.ExternalTrafficPolicy
}

func (r *ControlPlaneReconciler) createIngress(ctx context.Context, cfg *controllerIngressConfig) error {
	ingress := newControllerIngress(r.cp.ObjectMeta.Namespace, r.cp.Name, cfg)

	// Set ControlPlane instance as the owner and controller
	if err := controllerutil.SetControllerReference(&r.cp, ingress, r.Scheme); err != nil {
		return err
	}

	// Check if this resource already exists
	found := &networkingv1.Ingress{}

	err := r.Client.Get(ctx, types.NamespacedName{Name: ingress.Name, Namespace: ingress.Namespace}, found)
	if err != nil && k8serrors.IsNotFound(err) {
		r.log.Info("Creating a new Ingress", "Ingress.Namespace", ingress.Namespace, "Ingress.Name", ingress.Name)

		err = r.Client.Create(ctx, ingress)
		if err != nil {
			return err
		}

		// Resource created successfully - don't requeue
		return nil
	} else if err != nil {
		return err
	}

	if ingressNeedsPatch(found, ingress) {
		r.log.Info("Updating existing Ingress", "Ingress.Namespace", found.Namespace, "Ingress.Name", found.Name)
		applyIngressPatch(found, ingress)
		if err := r.Client.Update(ctx, found); err != nil {
			return err
		}
		return nil
	}

	r.log.Info("Skip reconcile: Ingress already exists", "Ingress.Namespace", found.Namespace, "Ingress.Name", found.Name)
	return nil
}

func ingressNeedsPatch(existing, desired *networkingv1.Ingress) bool {
	if !maps.Equal(existing.Annotations, desired.Annotations) {
		return true
	}
	if !ingressClassNameEqual(existing.Spec.IngressClassName, desired.Spec.IngressClassName) {
		return true
	}
	if !ingressTLSEqual(existing.Spec.TLS, desired.Spec.TLS) {
		return true
	}
	return ingressHost(existing) != ingressHost(desired)
}

func ingressClassNameEqual(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func ingressHost(ing *networkingv1.Ingress) string {
	if len(ing.Spec.Rules) == 0 {
		return ""
	}
	return ing.Spec.Rules[0].Host
}

func ingressTLSEqual(a, b []networkingv1.IngressTLS) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].SecretName != b[i].SecretName {
			return false
		}
		if len(a[i].Hosts) != len(b[i].Hosts) {
			return false
		}
		for j := range a[i].Hosts {
			if a[i].Hosts[j] != b[i].Hosts[j] {
				return false
			}
		}
	}
	return true
}

func applyIngressPatch(existing, desired *networkingv1.Ingress) {
	existing.Annotations = desired.Annotations
	existing.Spec.IngressClassName = desired.Spec.IngressClassName
	existing.Spec.TLS = desired.Spec.TLS
	existing.Spec.Rules = desired.Spec.Rules
}

func (r *ControlPlaneReconciler) createServiceAccount(ctx context.Context, ms *microservice) error {
	svcAcc := newServiceAccount(r.cp.ObjectMeta.Namespace, r.cp.Name, ms)

	// Set image pull secret for the service account
	if ms.imagePullSecret != "" {
		secret := &corev1.Secret{}
		err := r.Client.Get(ctx, types.NamespacedName{
			Namespace: svcAcc.Namespace,
			Name:      ms.imagePullSecret,
		}, secret)

		if err != nil || secret.Type != corev1.SecretTypeDockerConfigJson {
			r.log.Error(err, "Failed to create a new Service Account with imagePullSecret",
				"ServiceAccount.Namespace", svcAcc.Namespace,
				"ServiceAccount.Name", svcAcc.Name,
				"pullSecret", ms.imagePullSecret)

			return err
		}

		svcAcc.ImagePullSecrets = []corev1.LocalObjectReference{
			{Name: ms.imagePullSecret},
		}
	}

	// Set ControlPlane instance as the owner and controller
	if err := controllerutil.SetControllerReference(&r.cp, svcAcc, r.Scheme); err != nil {
		return err
	}

	// Check if this resource already exists
	found := &corev1.ServiceAccount{}

	err := r.Client.Get(ctx, types.NamespacedName{Name: svcAcc.Name, Namespace: svcAcc.Namespace}, found)
	if err != nil && k8serrors.IsNotFound(err) {
		r.log.Info("Creating a new Service Account", "ServiceAccount.Namespace", svcAcc.Namespace, "ServiceAccount.Name", svcAcc.Name)
		// TODO: Find out why the IsAlreadyExists() check is necessary here. Happens when CP redeployed
		if err = r.Client.Create(ctx, svcAcc); err != nil && !k8serrors.IsAlreadyExists(err) {
			return err
		}

		// Resource created successfully - don't requeue
		return nil
	} else if err != nil {
		return err
	}

	// Resource already exists - don't requeue
	r.log.Info("Skip reconcile: Service Account already exists", "ServiceAccount.Namespace", found.Namespace, "ServiceAccount.Name", found.Name)

	return nil
}

func (r *ControlPlaneReconciler) createRole(ctx context.Context, ms *microservice) error { //nolint:dupl
	role := newRole(r.cp.ObjectMeta.Namespace, r.cp.Name, ms)

	// Set ControlPlane instance as the owner and controller
	if err := controllerutil.SetControllerReference(&r.cp, role, r.Scheme); err != nil {
		return err
	}

	// Check if this resource already exists
	found := &rbacv1.Role{}

	err := r.Client.Get(ctx, types.NamespacedName{Name: role.Name, Namespace: role.Namespace}, found)
	if err != nil && k8serrors.IsNotFound(err) {
		r.log.Info("Creating a new Role ", "Role.Namespace", role.Namespace, "Role.Name", role.Name)

		err = r.Client.Create(ctx, role)
		if err != nil {
			return err
		}

		// Resource created successfully - don't requeue
		return nil
	} else if err != nil {
		return err
	}

	// Resource already exists - don't requeue
	r.log.Info("Skip reconcile: Role already exists", "Role.Namespace", found.Namespace, "Role.Name", found.Name)

	return nil
}

func (r *ControlPlaneReconciler) createRoleBinding(ctx context.Context, ms *microservice) error { //nolint:dupl
	crb := newRoleBinding(r.cp.ObjectMeta.Namespace, r.cp.Name, ms)

	// Set ControlPlane instance as the owner and controller
	if err := controllerutil.SetControllerReference(&r.cp, crb, r.Scheme); err != nil {
		return err
	}

	// Check if this resource already exists
	found := &rbacv1.RoleBinding{}

	err := r.Client.Get(ctx, types.NamespacedName{Name: crb.Name, Namespace: crb.Namespace}, found)
	if err != nil && k8serrors.IsNotFound(err) {
		r.log.Info("Creating a new Role Binding", "RoleBinding.Namespace", crb.Namespace, "RoleBinding.Name", crb.Name)

		err = r.Client.Create(ctx, crb)
		if err != nil {
			return err
		}

		// Resource created successfully - don't requeue
		return nil
	} else if err != nil {
		return err
	}

	// Resource already exists - don't requeue
	r.log.Info("Skip reconcile: Role Binding already exists", "RoleBinding.Namespace", found.Namespace, "RoleBinding.Name", found.Name)

	return nil
}

func (r *ControlPlaneReconciler) loginIofogClient(ctx context.Context, iofogClient *iofogclient.Client) error {
	auth := r.cp.Spec.Auth

	switch auth.Mode {
	case cpv3.AuthModeEmbedded:
		if auth.Bootstrap == nil {
			return fmt.Errorf("auth.bootstrap is required when mode=embedded")
		}
		username := strings.TrimSpace(auth.Bootstrap.Username)
		if username == "" {
			return fmt.Errorf("auth.bootstrap.username is required when mode=embedded")
		}
		password, err := resolveBootstrapPassword(ctx, r.Client, r.cp.Namespace, &auth)
		if err != nil {
			return fmt.Errorf("resolve bootstrap password: %w", err)
		}
		if password == "" {
			return fmt.Errorf("auth.bootstrap password is required when mode=embedded")
		}
		r.log.Info("Logging in to Controller with bootstrap credentials")
		return controllerlogin.EmbeddedBootstrapLogin(iofogClient, username, password)
	case cpv3.AuthModeExternal:
		if auth.IssuerUrl == "" {
			return fmt.Errorf("auth.issuerUrl is required when mode=external")
		}
		if auth.Client == nil || auth.Client.ID == "" || auth.Client.Secret == "" {
			return fmt.Errorf("auth.client id and secret are required when mode=external")
		}
		r.log.Info("Generating Client Access Token")
		return controllerlogin.ExternalClientCredentialsLogin(
			iofogClient,
			auth.IssuerUrl,
			auth.Client.ID,
			auth.Client.Secret,
		)
	default:
		return fmt.Errorf("unsupported auth mode: %q", auth.Mode)
	}
}

func newInt(val int) *int {
	return &val
}

func (r *ControlPlaneReconciler) createDefaultRouter(iofogClient *iofogclient.Client, proxy cpv3.RouterIngress) (err error) {
	routerConfig := iofogclient.Router{
		Host: proxy.Address,
		RouterConfig: iofogclient.RouterConfig{
			InterRouterPort: newInt(proxy.InteriorPort),
			EdgeRouterPort:  newInt(proxy.EdgePort),
			MessagingPort:   newInt(proxy.MessagePort),
		},
	}

	return iofogClient.PutDefaultRouter(routerConfig)
}

// createDefaultNatsHub registers the default NATS hub with the Controller (only when NATS is enabled).
func (r *ControlPlaneReconciler) createDefaultNatsHub(iofogClient *iofogclient.Client, ing cpv3.NatsIngress) error {
	serverPort := ing.ServerPort
	if serverPort == 0 {
		serverPort = 4222
	}
	clusterPort := ing.ClusterPort
	if clusterPort == 0 {
		clusterPort = 6222
	}
	leafPort := ing.LeafPort
	if leafPort == 0 {
		leafPort = 7422
	}
	mqttPort := ing.MqttPort
	if mqttPort == 0 {
		mqttPort = 8883
	}
	httpPort := ing.HttpPort
	if httpPort == 0 {
		httpPort = 8222
	}
	req := &iofogclient.NatsHubRequest{
		Host:        &ing.Address,
		ServerPort:  &serverPort,
		ClusterPort: &clusterPort,
		LeafPort:    &leafPort,
		MqttPort:    &mqttPort,
		HTTPPort:    &httpPort,
	}
	_, err := iofogClient.UpsertNatsHub(req)
	return err
}

// ConfigEntry represents a single entry in the router configuration
type ConfigEntry []interface{}

// shouldUpdate determines if a config entry should be updated based on its type and name
func shouldUpdate(entry ConfigEntry) bool {
	if len(entry) != 2 {
		return false
	}

	entryType, ok := entry[0].(string)
	if !ok {
		return false
	}

	data, ok := entry[1].(map[string]interface{})
	if !ok {
		return false
	}

	switch entryType {
	case "router", "site", "address", "log":
		return true
	case "sslProfile":
		if name, ok := data["name"].(string); ok {
			return name == "router-site-server" || name == "router-local-server"
		}
		return false
	case "listener":
		if name, ok := data["name"].(string); ok {
			return name == "iofog-router-edge" || name == "amqp" ||
				name == "amqps" || name == "@9090" ||
				name == "iofog-router-inter-router"
		}
		return false
	default:
		return false
	}
}

// getConfigVersion extracts the pot-config version from the router metadata
func getConfigVersion(config string) (string, error) {
	var entries []ConfigEntry
	if err := json.Unmarshal([]byte(config), &entries); err != nil {
		return "", fmt.Errorf("failed to parse config: %w", err)
	}

	for _, entry := range entries {
		if len(entry) != 2 {
			continue
		}

		entryType, ok := entry[0].(string)
		if !ok || entryType != "router" {
			continue
		}

		data, ok := entry[1].(map[string]interface{})
		if !ok {
			continue
		}

		if metadata, ok := data["metadata"].(string); ok {
			var metadataMap map[string]interface{}
			if err := json.Unmarshal([]byte(metadata), &metadataMap); err != nil {
				return "", fmt.Errorf("failed to parse metadata: %w", err)
			}
			if version, ok := metadataMap["pot-config"].(string); ok {
				return version, nil
			}
		}
	}
	return "", nil
}

// mergeConfigs merges existing and new router configurations
func mergeConfigs(existingConfig, newConfig string) (string, error) {
	// Check versions
	existingVersion, err := getConfigVersion(existingConfig)
	if err != nil {
		return "", fmt.Errorf("failed to get existing config version: %w", err)
	}

	newVersion, err := getConfigVersion(newConfig)
	if err != nil {
		return "", fmt.Errorf("failed to get new config version: %w", err)
	}

	// If versions are the same, keep existing config
	if existingVersion != "" && existingVersion == newVersion {
		return existingConfig, nil
	}

	var existing, new []ConfigEntry

	// Parse existing config
	if err := json.Unmarshal([]byte(existingConfig), &existing); err != nil {
		return "", fmt.Errorf("failed to parse existing config: %w", err)
	}

	// Parse new config
	if err := json.Unmarshal([]byte(newConfig), &new); err != nil {
		return "", fmt.Errorf("failed to parse new config: %w", err)
	}

	// Create map of existing entries for quick lookup
	existingMap := make(map[string]ConfigEntry)
	for _, entry := range existing {
		existingMap[entry[0].(string)] = entry
	}

	// Process new config
	result := make([]ConfigEntry, 0)
	for _, entry := range new {
		if shouldUpdate(entry) {
			// Update specified sections
			result = append(result, entry)
		} else if existing, exists := existingMap[entry[0].(string)]; exists {
			// Keep existing version
			result = append(result, existing)
		}
	}

	// Add any remaining existing entries that weren't in new config
	for _, entry := range existing {
		if !shouldUpdate(entry) {
			result = append(result, entry)
		}
	}

	// Marshal back to JSON
	mergedConfig, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal merged config: %w", err)
	}

	return string(mergedConfig), nil
}

func (r *ControlPlaneReconciler) createConfigMap(ctx context.Context) error {
	configMap := newRouterConfigMap(r.cp.ObjectMeta.Namespace, r.cp.Name)

	// Set owner reference
	if err := controllerutil.SetControllerReference(&r.cp, configMap, r.Scheme); err != nil {
		return err
	}

	// Try to get existing ConfigMap
	existingConfigMap := &corev1.ConfigMap{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: configMap.Name, Namespace: configMap.Namespace}, existingConfigMap)

	if err != nil {
		if k8serrors.IsNotFound(err) {
			// ConfigMap doesn't exist, create it
			return r.Client.Create(ctx, configMap)
		}
		return err
	}

	// ConfigMap exists, merge configurations
	mergedConfig, err := mergeConfigs(existingConfigMap.Data["skrouterd.json"], configMap.Data["skrouterd.json"])
	if err != nil {
		return fmt.Errorf("failed to merge configs: %w", err)
	}

	// Update ConfigMap with merged configuration and standard labels
	existingConfigMap.Data["skrouterd.json"] = mergedConfig
	existingConfigMap.Labels = mergeLabels(configMap.Labels, existingConfigMap.Labels)
	return r.Client.Update(ctx, existingConfigMap)
}

func (r *ControlPlaneReconciler) ImportRouterCACertificate(iofogClient *iofogclient.Client, secretName string) (err error) {

	// Create CA certificate
	request := iofogclient.CACreateRequest{
		Name:       secretName,
		Type:       "k8s-secret",
		SecretName: secretName,
	}

	_, err = iofogClient.GetCA(secretName)
	if err != nil {
		if !strings.Contains(err.Error(), "NotFoundError") {
			return err
		}

		return iofogClient.CreateCA(&request)
	}

	return err
}

func DecodeBase64(encoded string) (string, error) {
	decodedBytes, err := b64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}

	return string(decodedBytes), nil
}
