package controllers

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	iofogclient "github.com/eclipse-iofog/iofog-go-sdk/v3/pkg/client"
	k8sclient "github.com/eclipse-iofog/iofog-go-sdk/v3/pkg/k8s"
	op "github.com/eclipse-iofog/iofog-go-sdk/v3/pkg/k8s/operator"
	cpv3 "github.com/eclipse-iofog/iofog-operator/v3/apis/controlplanes/v3"
	"github.com/eclipse-iofog/iofog-operator/v3/controllers/controlplanes/nats"
	"github.com/eclipse-iofog/iofog-operator/v3/controllers/controlplanes/router"
	openidutil "github.com/eclipse-iofog/iofog-operator/v3/internal/util"
	util "github.com/eclipse-iofog/iofog-operator/v3/internal/util/certs"

	// "github.com/skupperproject/skupper/pkg/certs"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	loadBalancerTimeout   = 360
	errProxyRouterMissing = "missing Proxy.Router data for non LoadBalancer Router service"
	errParseControllerURL = "failed to parse Controller endpoint as URL (%s): %s"
)

// getEventsIfConfigured returns a pointer to Events if it's configured (at least one field is set), otherwise nil
func getEventsIfConfigured(events cpv3.Events) *cpv3.Events {
	// Check if at least one field is configured
	if events.AuditEnabled != nil || events.CaptureIpAddress != nil || events.RetentionDays != 0 || events.CleanupInterval != 0 {
		return &events
	}
	return nil
}

// getVaultIfConfigured returns spec.Vault if vault is configured (non-nil and has provider or provider-specific block), otherwise nil.
func getVaultIfConfigured(spec cpv3.ControlPlaneSpec) *cpv3.Vault {
	if spec.Vault == nil {
		return nil
	}
	v := spec.Vault
	if v.Provider != "" || v.Hashicorp != nil || v.Aws != nil || v.Azure != nil || v.Google != nil {
		return v
	}
	return nil
}

func reconcileRoutine(ctx context.Context, recon func(context.Context) op.Reconciliation, reconChan chan op.Reconciliation) {
	reconChan <- recon(ctx)
}

func (r *ControlPlaneReconciler) reconcileDBCredentialsSecret(ctx context.Context, ms *microservice) (shouldRestartPod bool, err error) {
	stdLabels := getStandardLabels("controller", r.cp.Name)
	for i := range ms.secrets {
		secret := &ms.secrets[i]

		if secret.Name == controllerDBCredentialsSecretName {
			secret.Labels = mergeLabels(stdLabels, secret.Labels)
			found := &corev1.Secret{}

			err := r.Client.Get(ctx, types.NamespacedName{Name: secret.Name, Namespace: secret.Namespace}, found)
			if err != nil {
				if !k8serrors.IsNotFound(err) {
					return false, err
				}
				// Create secret
				err = r.Client.Create(ctx, secret)
				if err != nil {
					return false, err
				}

				return false, nil
			}
			// Secret already exists
			// Update secret
			err = r.Client.Update(ctx, secret)
			if err != nil {
				return false, err
			}
			// Restart pod
			return true, nil
		}
	}

	return false, nil
}

func (r *ControlPlaneReconciler) reconcileVaultCredentialsSecret(ctx context.Context, ms *microservice) (shouldRestartPod bool, err error) {
	stdLabels := getStandardLabels("controller", r.cp.Name)
	for i := range ms.secrets {
		secret := &ms.secrets[i]
		if secret.Name != controllerVaultCredentialsSecretName {
			continue
		}
		secret.Labels = mergeLabels(stdLabels, secret.Labels)
		if setErr := controllerutil.SetControllerReference(&r.cp, secret, r.Scheme); setErr != nil {
			return false, setErr
		}
		found := &corev1.Secret{}
		getErr := r.Client.Get(ctx, types.NamespacedName{Name: secret.Name, Namespace: secret.Namespace}, found)
		if getErr != nil {
			if !k8serrors.IsNotFound(getErr) {
				return false, getErr
			}
			if createErr := r.Client.Create(ctx, secret); createErr != nil {
				return false, createErr
			}
			return false, nil
		}
		if updateErr := r.Client.Update(ctx, secret); updateErr != nil {
			return false, updateErr
		}
		return true, nil
	}
	return false, nil
}

func (r *ControlPlaneReconciler) reconcileIofogController(ctx context.Context) op.Reconciliation {
	// Configure Controller
	config := &controllerMicroserviceConfig{
		controllerName:        r.cp.Name,
		replicas:              r.cp.Spec.Replicas.Controller,
		image:                 r.cp.Spec.Images.Controller,
		imagePullSecret:       r.cp.Spec.Images.PullSecret,
		routerImage:           r.cp.Spec.Images.Router,
		natsImage:             r.cp.Spec.Images.Nats,
		natsEnabled:           isNatsEnabled(r.cp),
		db:                    &r.cp.Spec.Database,
		auth:                  &r.cp.Spec.Auth,
		serviceType:           r.cp.Spec.Services.Controller.Type,
		serviceAnnotations:    r.cp.Spec.Services.Controller.Annotations,
		externalTrafficPolicy: r.cp.Spec.Services.Controller.ExternalTrafficPolicy,
		loadBalancerAddr:      r.cp.Spec.Services.Controller.Address,
		https:                 r.cp.Spec.Controller.Https,
		secretName:            r.cp.Spec.Controller.SecretName,
		ecn:                   r.cp.Spec.Controller.ECNName,
		pidBaseDir:            r.cp.Spec.Controller.PidBaseDir,
		ecnViewerPort:         r.cp.Spec.Controller.EcnViewerPort,
		ecnViewerURL:          r.cp.Spec.Controller.EcnViewerURL,
		logLevel:              r.cp.Spec.Controller.LogLevel,
		events:                getEventsIfConfigured(r.cp.Spec.Events),
		vault:                 getVaultIfConfigured(r.cp.Spec),
	}

	ingressConfig := &controllerIngressConfig{
		annotations:      r.cp.Spec.Ingresses.Controller.Annotations,
		ingressClassName: r.cp.Spec.Ingresses.Controller.IngressClassName,
		host:             r.cp.Spec.Ingresses.Controller.Host,
		secretName:       r.cp.Spec.Ingresses.Controller.SecretName,
	}

	// get scheme for controller endpoint
	var scheme string
	if config.https == nil || *config.https == false {
		scheme = "http"
	} else {
		scheme = "https"
	}

	// Create Controller Microservice
	ms := newControllerMicroservice(r.cp.Namespace, config)

	// Service Account
	if err := r.createServiceAccount(ctx, ms); err != nil {
		return op.ReconcileWithError(err)
	}

	// Role
	if err := r.createRole(ctx, ms); err != nil {
		return op.ReconcileWithError(err)
	}

	// Role binding
	if err := r.createRoleBinding(ctx, ms); err != nil {
		return op.ReconcileWithError(err)
	}

	// Handle DB credentials secret
	shouldRestartPods, err := r.reconcileDBCredentialsSecret(ctx, ms)
	if err != nil {
		return op.ReconcileWithError(err)
	}
	// Handle vault credentials secret (operator-created from spec)
	restartVault, err := r.reconcileVaultCredentialsSecret(ctx, ms)
	if err != nil {
		return op.ReconcileWithError(err)
	}
	if restartVault {
		shouldRestartPods = true
	}
	// Create secrets
	r.log.Info(fmt.Sprintf("Creating secrets for controller reconcile for Controlplane %s", r.cp.Name))

	if err := r.createSecrets(ctx, ms); err != nil {
		r.log.Info(fmt.Sprintf("Failed to create secrets %v for controller reconcile for Controlplane %s", err, r.cp.Name))

		return op.ReconcileWithError(err)
	}

	// Service
	if err := r.createService(ctx, ms); err != nil {
		return op.ReconcileWithError(err)
	}

	// Ingress
	if strings.EqualFold(r.cp.Spec.Services.Controller.Type, string(corev1.ServiceTypeClusterIP)) {
		if err := r.createIngress(ctx, ingressConfig); err != nil {
			return op.ReconcileWithError(err)
		}
	}

	// PVC
	if err := r.createPersistentVolumeClaims(ctx, ms); err != nil {
		return op.ReconcileWithError(err)
	}

	alreadyExists, err := r.deploymentExists(ctx, r.cp.Namespace, ms.name)
	if err != nil {
		return op.ReconcileWithError(err)
	}

	// Deployment
	if err := r.createDeployment(ctx, ms); err != nil {
		return op.ReconcileWithError(err)
	}

	// The deployment was just created, requeue to hide latency
	if !alreadyExists {
		return op.ReconcileWithRequeue(time.Second * 5) //nolint:gomnd
	}
	// Instantiate Controller client
	ctrlPort, err := getControllerPort(ms)
	if err != nil {
		return op.ReconcileWithError(err)
	}

	host := fmt.Sprintf("%s.%s.svc.cluster.local", ms.name, r.cp.ObjectMeta.Namespace)
	iofogClient, fin := r.getIofogClient(scheme, host, ctrlPort)

	if fin.IsFinal() {
		return fin
	}
	// Set up user
	if err := r.loginIofogClient(iofogClient); err != nil {
		if !strings.Contains(strings.ToLower(err.Error()), "invalid credentials") {
			r.log.Info(fmt.Sprintf("Could not login to ControlPlane %s: %s", r.cp.Name, err.Error()))
			return op.ReconcileWithError(err)
		}
	}

	// Connect to cluster
	k8sClient, err := newK8sClient()
	if err != nil {
		return op.ReconcileWithError(err)
	}

	// Get Router or Router Proxy
	var routerProxy cpv3.RouterIngress

	if strings.EqualFold(r.cp.Spec.Services.Router.Type, string(corev1.ServiceTypeLoadBalancer)) {
		//nolint:contextcheck // k8sClient unfortunately does not accept context
		routerAddr, err := k8sClient.WaitForLoadBalancer(r.cp.Namespace, routerName, loadBalancerTimeout)
		if err != nil {
			return op.ReconcileWithError(err)
		}

		routerProxy = cpv3.RouterIngress{
			Address:      routerAddr,
			MessagePort:  router.MessagePort,
			InteriorPort: router.InteriorPort,
			EdgePort:     router.EdgePort,
		}
	} else if r.cp.Spec.Ingresses.Router.Address != "" {
		routerProxy = r.cp.Spec.Ingresses.Router
	} else {
		err := fmt.Errorf("reconcile Controller failed: %s", errProxyRouterMissing)

		return op.ReconcileWithError(err)
	}

	if err := r.createDefaultRouter(iofogClient, routerProxy); err != nil {
		return op.ReconcileWithError(err)
	}

	// Register default NATS hub when NATS is enabled
	if isNatsEnabled(r.cp) {
		natsIngress := r.cp.Spec.Ingresses.Nats
		if strings.EqualFold(r.cp.Spec.Services.Nats.Type, string(corev1.ServiceTypeLoadBalancer)) {
			//nolint:contextcheck // k8sClient does not accept context
			natsAddr, err := k8sClient.WaitForLoadBalancer(r.cp.Namespace, nats.ClientServiceName, loadBalancerTimeout)
			if err != nil {
				return op.ReconcileWithError(err)
			}
			natsIngress.Address = natsAddr
		}
		if natsIngress.Address != "" {
			if err := r.createDefaultNatsHub(iofogClient, natsIngress); err != nil {
				r.log.Info(fmt.Sprintf("Failed to register NATS hub for ControlPlane %s: %s", r.cp.Name, err.Error()))
				return op.ReconcileWithRequeue(time.Second * 10)
			}
		}
	}

	// Import router and NATS certificates
	if recon := r.ImportCertificates(iofogClient); recon.IsFinal() {
		return recon
	}

	// Wait for Controller LB to actually work
	r.log.Info(fmt.Sprintf("Waiting for IP/LB Service in iofog-controller reconcile for ControlPlane %s", r.cp.Name))

	var viewerEndpoint string

	if strings.EqualFold(r.cp.Spec.Services.Controller.Type, string(corev1.ServiceTypeLoadBalancer)) {
		//nolint:contextcheck // k8sClient unfortunately does not accept context
		host, err := k8sClient.WaitForLoadBalancer(r.cp.Namespace, controllerName, loadBalancerTimeout)
		if err != nil {
			return op.ReconcileWithError(err)
		}
		// Check LB connection works
		if _, fin := r.getIofogClient(scheme, host, ctrlPort); fin.IsFinal() {
			r.log.Info(fmt.Sprintf("LB Connection works for ControlPlane %s", r.cp.Name))

			return fin
		}
		viewerEndpoint = fmt.Sprintf("%s://%s", scheme, host)
	}

	if strings.EqualFold(r.cp.Spec.Services.Controller.Type, string(corev1.ServiceTypeClusterIP)) {
		// Retrieve the Ingress resource
		ingress := &networkingv1.Ingress{}
		err := r.Client.Get(ctx, types.NamespacedName{Name: "pot-controller", Namespace: r.cp.Namespace}, ingress)
		if err != nil {
			return op.ReconcileWithError(fmt.Errorf("failed to get Ingress resource: %w", err))
		}

		// Check if LoadBalancer ingress exists
		if len(ingress.Status.LoadBalancer.Ingress) == 0 {
			return op.ReconcileWithError(fmt.Errorf("no LoadBalancer ingress found for Ingress resource"))
		}

		if r.cp.Spec.Ingresses.Controller.Host != "" {
			viewerEndpoint = fmt.Sprintf("%s://%s", scheme, r.cp.Spec.Ingresses.Controller.Host)
		}
	}

	if shouldRestartPods {
		r.log.Info(fmt.Sprintf("Restarting pods for ControlPlane %s", r.cp.Name))

		if err := r.restartPodsForDeployment(ctx, ms.name, r.cp.Namespace); err != nil {
			return op.ReconcileWithError(err)
		}
	}

	// Update ECN Viewer Client Root URL
	if viewerEndpoint != "" {
		r.log.Info(fmt.Sprintf("Updating ECN Viewer Client Root URL for ControlPlane %s to %s", r.cp.Name, viewerEndpoint))
		if err := openidutil.UpdateECNViewerClientRootURL(r.cp.Spec.Auth, viewerEndpoint); err != nil {
			r.log.Info(fmt.Sprintf("Failed to update ECN Viewer Client Root URL for ControlPlane %s: %s", r.cp.Name, err.Error()))
			// Continue even if update fails, as it's not critical for the reconcile process
		}
	}

	r.log.Info(fmt.Sprintf("op.Continue in iofog-controller reconcile for ControlPlane %s", r.cp.Name))

	return op.Continue()
}

func (r *ControlPlaneReconciler) getIofogClient(scheme string, host string, port int) (*iofogclient.Client, op.Reconciliation) {
	baseURL := fmt.Sprintf("%v://%s:%d/api/v3", scheme, host, port) //nolint:nosprintfhostport

	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, op.ReconcileWithError(fmt.Errorf(errParseControllerURL, baseURL, err.Error()))
	}

	iofogClient := iofogclient.New(iofogclient.Options{
		BaseURL: parsedURL,
		Timeout: 10,
	})

	if _, err = iofogClient.GetStatus(); err != nil {
		r.log.Info(fmt.Sprintf("Could not get Controller status for ControlPlane %s: %s", r.cp.Name, err.Error()))

		return nil, op.ReconcileWithRequeue(time.Second * 3) //nolint:gomnd
	}

	return iofogClient, op.Continue()
}

func (r *ControlPlaneReconciler) ImportCertificates(iofogClient *iofogclient.Client) op.Reconciliation {
	r.log.Info(fmt.Sprintf("Importing certificates for ControlPlane %s", r.cp.Name))
	if err := r.ImportRouterCACertificate(iofogClient, "router-site-ca"); err != nil {
		r.log.Info(fmt.Sprintf("Failed to import certificates for ControlPlane %s: %s", r.cp.Name, err.Error()))
		return op.ReconcileWithRequeue(time.Second * 10)
	}

	if err := r.ImportRouterCACertificate(iofogClient, "default-router-local-ca"); err != nil {
		r.log.Info(fmt.Sprintf("Failed to import certificates for ControlPlane %s: %s", r.cp.Name, err.Error()))
		return op.ReconcileWithRequeue(time.Second * 10)
	}

	if isNatsEnabled(r.cp) {
		if err := r.ImportRouterCACertificate(iofogClient, nats.NatsSiteCASecret); err != nil {
			r.log.Info(fmt.Sprintf("Failed to import NATS site CA for ControlPlane %s: %s", r.cp.Name, err.Error()))
			return op.ReconcileWithRequeue(time.Second * 10)
		}
		if err := r.ImportRouterCACertificate(iofogClient, nats.NatsLocalCASecret); err != nil {
			r.log.Info(fmt.Sprintf("Failed to import NATS local CA for ControlPlane %s: %s", r.cp.Name, err.Error()))
			return op.ReconcileWithRequeue(time.Second * 10)
		}
	}

	return op.Continue()
}

// getControllerClientForNats returns an iofog client for the ControlPlane's controller (in-cluster DNS).
// Used to call GET /api/v3/nats/bootstrap. Requeues if the controller is not reachable yet.
func (r *ControlPlaneReconciler) getControllerClientForNats() (*iofogclient.Client, op.Reconciliation) {
	scheme := "http"
	if r.cp.Spec.Controller.Https != nil && *r.cp.Spec.Controller.Https {
		scheme = "https"
	}
	host := fmt.Sprintf("controller.%s.svc.cluster.local", r.cp.Namespace)
	const controllerAPIPort = 51121
	return r.getIofogClient(scheme, host, controllerAPIPort)
}

// isNatsEnabled returns true when NATS is enabled (Spec.Nats nil or Enabled not false, and Replicas.Nats >= 2).
func isNatsEnabled(cp cpv3.ControlPlane) bool {
	if cp.Spec.Nats != nil && cp.Spec.Nats.Enabled != nil && !*cp.Spec.Nats.Enabled {
		return false
	}
	return true
}

func (r *ControlPlaneReconciler) reconcileRouter(ctx context.Context) op.Reconciliation {
	// Check if HA is enabled (default to false if not specified)
	haEnabled := false
	// if r.cp.Spec.Router.HA != nil {
	// 	haEnabled = *r.cp.Spec.Router.HA
	// }

	routerMicroservices := newRouterMicroservices(routerMicroserviceConfig{
		image:                 r.cp.Spec.Images.Router,
		imagePullSecret:       r.cp.Spec.Images.PullSecret,
		serviceType:           r.cp.Spec.Services.Router.Type,
		serviceAnnotations:    r.cp.Spec.Services.Router.Annotations,
		externalTrafficPolicy: r.cp.Spec.Services.Router.ExternalTrafficPolicy,
		ha:                    haEnabled,
	})

	// Use the primary router for service creation and IP resolution
	ms := routerMicroservices[0]

	// Service Account, Role, and Role Binding (only for primary router)
	if err := r.createServiceAccount(ctx, ms); err != nil {
		return op.ReconcileWithError(err)
	}

	if err := r.createRole(ctx, ms); err != nil {
		return op.ReconcileWithError(err)
	}

	if err := r.createRoleBinding(ctx, ms); err != nil {
		return op.ReconcileWithError(err)
	}

	// Service
	if err := r.createService(ctx, ms); err != nil {
		return op.ReconcileWithError(err)
	}

	// Wait for IP
	k8sClient, err := newK8sClient()
	if err != nil {
		return op.ReconcileWithError(err)
	}

	// Wait for external IP of LB Service

	r.log.Info(fmt.Sprintf("Waiting for IP/LB Service in router reconcile for ControlPlane %s", r.cp.Name))

	var address string

	if strings.EqualFold(r.cp.Spec.Services.Router.Type, string(corev1.ServiceTypeLoadBalancer)) {
		//nolint:contextcheck // k8sClient unfortunately does not accept context
		address, err = k8sClient.WaitForLoadBalancer(r.cp.ObjectMeta.Namespace, ms.name, loadBalancerTimeout)
		if err != nil {
			return op.ReconcileWithError(err)
		}
	} else if r.cp.Spec.Ingresses.Router.Address != "" {
		address = r.cp.Spec.Ingresses.Router.Address
	} else {
		err = fmt.Errorf("reconcile Router failed: %s", errProxyRouterMissing)

		return op.ReconcileWithError(err)
	}

	r.log.Info(fmt.Sprintf("Found address %s for router reconcile for Controlplane %s", address, r.cp.Name))

	if err := r.createRouterSecrets(r.cp.ObjectMeta.Namespace, ms, address); err != nil {
		return op.ReconcileWithError(err)
	}
	// Create secrets
	r.log.Info(fmt.Sprintf("Creating secrets for router reconcile for Controlplane %s", r.cp.Name))

	if err := r.createSecrets(ctx, ms); err != nil {
		r.log.Info(fmt.Sprintf("Failed to create secrets %v for router reconcile for Controlplane %s", err, r.cp.Name))

		return op.ReconcileWithError(err)
	}

	// Router ConfigMap
	r.log.Info(fmt.Sprintf("Creating configmap for router reconcile for Controlplane %s", r.cp.Name))

	if err := r.createConfigMap(ctx); err != nil {
		r.log.Info(fmt.Sprintf("Failed to create configmap %v for router reconcile for Controlplane %s", err, r.cp.Name))
		return op.ReconcileWithError(err)
	}

	// Deployments
	r.log.Info(fmt.Sprintf("Creating deployments for router reconcile for Controlplane %s", r.cp.Name))

	for _, routerMS := range routerMicroservices {
		if err := r.createDeployment(ctx, routerMS); err != nil {
			r.log.Info(fmt.Sprintf("Failed to create deployment %v for router reconcile for Controlplane %s", err, r.cp.Name))
			return op.ReconcileWithError(err)
		}
	}

	r.log.Info(fmt.Sprintf("op.Continue for router reconcile for Controlplane %s", r.cp.Name))

	return op.Continue()
}

func (r *ControlPlaneReconciler) reconcileNats(ctx context.Context) op.Reconciliation {
	if !isNatsEnabled(r.cp) {
		return op.Continue()
	}
	namespace := r.cp.Namespace
	instanceName := r.cp.Name
	natsLabels := getStandardLabels("nats", instanceName)
	replicas := r.cp.Spec.Replicas.Nats
	if replicas < 2 {
		replicas = 2
	}

	// Bootstrap from Controller API (GET /api/v3/nats/bootstrap). Controller performs bootstrap; operator only saves secrets (creds come base64 in response).
	iofogClient, recon := r.getControllerClientForNats()
	if recon.IsFinal() {
		return recon
	}
	if err := r.loginIofogClient(iofogClient); err != nil {
		if !strings.Contains(strings.ToLower(err.Error()), "invalid credentials") {
			r.log.Info(fmt.Sprintf("Could not login for NATS bootstrap ControlPlane %s: %s", r.cp.Name, err.Error()))
			return op.ReconcileWithError(err)
		}
	}
	bootstrapResp, err := iofogClient.GetNatsBootstrap()
	if err != nil {
		r.log.Error(err, "NATS bootstrap API failed")
		return op.ReconcileWithError(fmt.Errorf("get NATS bootstrap from Controller: %w", err))
	}
	createOrUpdateSecret := func(ctx context.Context, s *corev1.Secret) error {
		if err := controllerutil.SetControllerReference(&r.cp, s, r.Scheme); err != nil {
			return err
		}
		err := r.Client.Create(ctx, s)
		if err == nil {
			return nil
		}
		if !k8serrors.IsAlreadyExists(err) {
			return err
		}
		existing := &corev1.Secret{}
		if err := r.Client.Get(ctx, types.NamespacedName{Name: s.Name, Namespace: s.Namespace}, existing); err != nil {
			return err
		}
		existing.Data = s.Data
		existing.Labels = mergeLabels(natsLabels, existing.Labels)
		return r.Client.Update(ctx, existing)
	}
	bootstrap, err := nats.EnsureNatsBootstrapFromController(ctx, createOrUpdateSecret, namespace, natsLabels, &nats.BootstrapFromAPI{
		OperatorJwt:            bootstrapResp.OperatorJwt,
		OperatorPublicKey:      bootstrapResp.OperatorPublicKey,
		OperatorSeed:           bootstrapResp.OperatorSeed,
		SystemAccountJwt:       bootstrapResp.SystemAccountJwt,
		SystemAccountPublicKey: bootstrapResp.SystemAccountPublicKey,
		SysUserCredsBase64:     bootstrapResp.SysUserCredsBase64,
	})
	if err != nil {
		r.log.Error(err, "NATS bootstrap save secrets failed")
		return op.ReconcileWithError(err)
	}

	// JetStream key secret
	_, err = nats.EnsureJetStreamKeySecret(ctx,
		func(ctx context.Context, nn types.NamespacedName, s *corev1.Secret) error {
			return r.Client.Get(ctx, nn, s)
		},
		func(ctx context.Context, s *corev1.Secret) error {
			if err := controllerutil.SetControllerReference(&r.cp, s, r.Scheme); err != nil {
				return err
			}
			return r.Client.Create(ctx, s)
		},
		namespace, instanceName, natsLabels)
	if err != nil {
		return op.ReconcileWithError(err)
	}

	// PVC uses Gi/Mi (Kubernetes); NATS server.conf uses G/M/T/K (NATS does not support Gi, Mi).
	storageSizePVC := nats.DefaultStorageSizePVC
	storageSizeNats := nats.DefaultStorageSize
	memoryStoreSizeNats := nats.DefaultMemoryStoreSize
	if r.cp.Spec.Nats != nil {
		if r.cp.Spec.Nats.JetStream.StorageSize != "" {
			storageSizePVC = r.cp.Spec.Nats.JetStream.StorageSize
			storageSizeNats = nats.ToNatsSize(r.cp.Spec.Nats.JetStream.StorageSize)
		}
		if r.cp.Spec.Nats.JetStream.MemoryStoreSize != "" {
			memoryStoreSizeNats = nats.ToNatsSize(r.cp.Spec.Nats.JetStream.MemoryStoreSize)
		}
	}
	natsSvcType := corev1.ServiceTypeLoadBalancer
	if r.cp.Spec.Services.Nats.Type != "" {
		natsSvcType = corev1.ServiceType(r.cp.Spec.Services.Nats.Type)
	}
	natsServerSvcType := corev1.ServiceTypeLoadBalancer
	if r.cp.Spec.Services.NatsServer.Type != "" {
		natsServerSvcType = corev1.ServiceType(r.cp.Spec.Services.NatsServer.Type)
	}
	natsMs := newNatsMicroservice(natsMicroserviceConfig{
		image:                       openidutil.GetNatsImage(),
		imagePullSecret:             r.cp.Spec.Images.PullSecret,
		replicas:                    replicas,
		storageSize:                 storageSizePVC,
		storageClassName:            "",
		jetStreamKeySecret:          nats.JetStreamKeySecretName(instanceName),
		serviceType:                 string(natsSvcType),
		serviceAnnotations:          r.cp.Spec.Services.Nats.Annotations,
		externalTrafficPolicy:       r.cp.Spec.Services.Nats.ExternalTrafficPolicy,
		serverServiceType:           string(natsServerSvcType),
		serverServiceAnnotations:    r.cp.Spec.Services.NatsServer.Annotations,
		serverExternalTrafficPolicy: r.cp.Spec.Services.NatsServer.ExternalTrafficPolicy,
	})
	if r.cp.Spec.Images.Nats != "" {
		natsMs.containers[0].image = r.cp.Spec.Images.Nats
	}
	if r.cp.Spec.Nats != nil && r.cp.Spec.Nats.JetStream.StorageClassName != "" {
		natsMs.volumeClaimTemplates[0].Spec.StorageClassName = &r.cp.Spec.Nats.JetStream.StorageClassName
	}

	// Create NATS Services first (via microservice flow) so we can resolve address for TLS SANs, like router
	if err := r.createServiceAccount(ctx, natsMs); err != nil {
		return op.ReconcileWithError(err)
	}
	if err := r.createRole(ctx, natsMs); err != nil {
		return op.ReconcileWithError(err)
	}
	if err := r.createRoleBinding(ctx, natsMs); err != nil {
		return op.ReconcileWithError(err)
	}
	if err := r.createService(ctx, natsMs); err != nil {
		return op.ReconcileWithError(err)
	}

	// Resolve NATS address (LB or ingress) for TLS cert SANs and hub registration, same pattern as router
	var natsAddress string
	if strings.EqualFold(r.cp.Spec.Services.Nats.Type, string(corev1.ServiceTypeLoadBalancer)) {
		k8sClient, k8sErr := newK8sClient()
		if k8sErr != nil {
			return op.ReconcileWithError(k8sErr)
		}
		//nolint:contextcheck // k8sClient does not accept context
		natsAddress, err = k8sClient.WaitForLoadBalancer(namespace, nats.ClientServiceName, loadBalancerTimeout)
		if err != nil {
			return op.ReconcileWithError(err)
		}
	} else if r.cp.Spec.Ingresses.Nats.Address != "" {
		natsAddress = r.cp.Spec.Ingresses.Nats.Address
	}

	// When replica count changes, nats-site-server (and nats-mqtt-server) cert SANs are stale;
	// delete them so EnsureNatsSecrets recreates them with hosts for all current replicas.
	desiredReplicasStr := fmt.Sprintf("%d", replicas)
	existingSiteServer := &corev1.Secret{}
	err = r.Client.Get(ctx, types.NamespacedName{Name: nats.NatsSiteServerSecret, Namespace: namespace}, existingSiteServer)
	if err == nil {
		annotatedReplicas := existingSiteServer.Annotations["datasance.com/nats-replicas"]
		if annotatedReplicas != desiredReplicasStr {
			for _, name := range []string{nats.NatsSiteServerSecret, nats.NatsMqttServerSecret} {
				delErr := r.Client.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}})
				if delErr != nil && !k8serrors.IsNotFound(delErr) {
					return op.ReconcileWithError(delErr)
				}
			}
		}
	}

	// NATS TLS secrets (with external address in SANs like createRouterSecrets)
	tlsSecrets, err := nats.EnsureNatsSecrets(ctx,
		func(ctx context.Context, nn types.NamespacedName, s *corev1.Secret) error {
			return r.Client.Get(ctx, nn, s)
		},
		namespace, instanceName, nats.HeadlessServiceName, int(replicas), natsAddress, natsLabels)
	if err != nil {
		return op.ReconcileWithError(err)
	}
	for i := range tlsSecrets {
		if err := controllerutil.SetControllerReference(&r.cp, &tlsSecrets[i], r.Scheme); err != nil {
			return op.ReconcileWithError(err)
		}
		if err := r.Client.Create(ctx, &tlsSecrets[i]); err != nil && !k8serrors.IsAlreadyExists(err) {
			return op.ReconcileWithError(err)
		}
	}

	// JetStream key value for server.conf (read from secret)
	jetStreamKeySecret := &corev1.Secret{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: nats.JetStreamKeySecretName(instanceName), Namespace: namespace}, jetStreamKeySecret); err != nil {
		return op.ReconcileWithError(err)
	}
	jetStreamKey := string(jetStreamKeySecret.Data["jsk"])
	jetStreamPrev := string("")

	// Preserve controller-added cluster routes when merging: get existing server.conf if present.
	existingServerConf := ""
	existingNatsCM := &corev1.ConfigMap{}
	if getErr := r.Client.Get(ctx, types.NamespacedName{Name: nats.ConfigMapName, Namespace: namespace}, existingNatsCM); getErr == nil {
		if data := existingNatsCM.Data[nats.ServerConfKey()]; data != "" {
			existingServerConf = data
		}
	}

	// Leaf advertise: same host as natsIngress (createDefaultNatsHub), port = ingress leaf port or 7422
	leafPort := nats.DefaultLeafPort
	if r.cp.Spec.Ingresses.Nats.LeafPort > 0 {
		leafPort = r.cp.Spec.Ingresses.Nats.LeafPort
	}
	leafAdvertise := ""
	if natsAddress != "" {
		leafAdvertise = fmt.Sprintf("%s:%d", natsAddress, leafPort)
	}

	// JETSTREAM_DOMAIN = controlplane namespace (Controller: CONTROLLER_NAMESPACE / app.namespace)
	serverConf := nats.BuildServerConf(nats.ServerConfParams{
		ServerPort:      nats.DefaultServerPort,
		HttpPort:        nats.DefaultHttpPort,
		OperatorJWT:     bootstrap.OperatorJWT,
		SystemAccount:   bootstrap.SystemAccountPubKey,
		JetStreamDomain: namespace,
		JetStreamKey:    jetStreamKey,
		JetStreamPrev:   jetStreamPrev,
		ClusterRoutes:   nats.ClusterRoutesMerge(nats.HeadlessServiceName, int(replicas), nats.DefaultClusterPort, existingServerConf),
		SSLDir:          "/etc/nats/certs",
		CertName:        nats.NatsSiteServerSecret,
		MqttCertName:    nats.NatsMqttServerSecret,
		LeafPort:        nats.DefaultLeafPort,
		LeafAdvertise:   leafAdvertise,
		ClusterPort:     nats.DefaultClusterPort,
		MqttPort:        nats.DefaultMqttPort,
		JWTDir:          "/home/runner/nats/jwt",
		ControllerName:  instanceName,
		MaxMemoryStore:  memoryStoreSizeNats,
		MaxFileStore:    storageSizeNats,
	})

	configMap := nats.NewNatsConfigMap(namespace, instanceName, natsLabels, serverConf)
	if err := controllerutil.SetControllerReference(&r.cp, configMap, r.Scheme); err != nil {
		return op.ReconcileWithError(err)
	}
	// Always create or update ConfigMap so replica count / routes stay in sync when CR is updated.
	existingCM := &corev1.ConfigMap{}
	err = r.Client.Get(ctx, types.NamespacedName{Name: configMap.Name, Namespace: configMap.Namespace}, existingCM)
	if err != nil && k8serrors.IsNotFound(err) {
		if err = r.Client.Create(ctx, configMap); err != nil {
			return op.ReconcileWithError(err)
		}
	} else if err != nil {
		return op.ReconcileWithError(err)
	} else {
		existingCM.Data = configMap.Data
		existingCM.Labels = mergeLabels(natsLabels, existingCM.Labels)
		if err = r.Client.Update(ctx, existingCM); err != nil {
			return op.ReconcileWithError(err)
		}
	}

	jwtBundle := nats.NewJWTBundleConfigMap(namespace, natsLabels, map[string]string{bootstrap.SystemAccountPubKey: bootstrap.SystemAccountJWT})
	if err := controllerutil.SetControllerReference(&r.cp, jwtBundle, r.Scheme); err != nil {
		return op.ReconcileWithError(err)
	}
	if err := r.Client.Create(ctx, jwtBundle); err != nil && !k8serrors.IsAlreadyExists(err) {
		return op.ReconcileWithError(err)
	}

	// When NATS replicas are scaled down, cluster routes are removed from server.conf; NATS requires a restart to drop routes.
	// Trigger a StatefulSet rolling restart by updating the pod template with restartedAt when current replicas > desired.
	existingSt := &appsv1.StatefulSet{}
	getStErr := r.Client.Get(ctx, types.NamespacedName{Name: "nats", Namespace: namespace}, existingSt)
	if getStErr == nil {
		existingReplicas := int32(1)
		if existingSt.Spec.Replicas != nil {
			existingReplicas = *existingSt.Spec.Replicas
		}
		if existingReplicas > replicas {
			// Scale-down: add restartedAt so StatefulSet controller does a rolling update.
			ann := make(map[string]string)
			if existingSt.Spec.Template.Annotations != nil {
				for k, v := range existingSt.Spec.Template.Annotations {
					ann[k] = v
				}
			}
			ann["kubectl.kubernetes.io/restartedAt"] = time.Now().UTC().Format(time.RFC3339)
			natsMs.podTemplateAnnotations = ann
		} else {
			// Preserve existing annotations so we don't remove restartedAt and trigger another rollout.
			natsMs.podTemplateAnnotations = existingSt.Spec.Template.Annotations
		}
	}

	// Create StatefulSet via shared microservice flow (same as Deployment for controller/router but with isStatefulSet flag)
	if err := r.createStatefulSet(ctx, natsMs); err != nil {
		return op.ReconcileWithError(err)
	}

	return op.Continue()
}

// createRouterSecrets creates the secrets for the router.
// It generates the CA and secrets for the router.
// It also appends the secrets to the microservice.secrets slice.
func (r *ControlPlaneReconciler) createRouterSecrets(namespace string, ms *microservice, address string) (err error) {
	r.log.Info(fmt.Sprintf("Creating routerSecrets definition for router reconcile for Controlplane %s", r.cp.Name))

	defer func() {
		if recoverResult := recover(); recoverResult != nil {
			r.log.Info(fmt.Sprintf("Recover result %v for creating secrets for router reconcile for Controlplane %s", recoverResult, r.cp.Name))
			err = fmt.Errorf("createRouterSecrets failed: %v", recoverResult)
		}
	}()

	const (
		LocalClientSecret        string = "skupper-local-client"
		LocalServerSecret        string = "router-local-server"
		LocalCaSecret            string = "default-router-local-ca"
		SiteServerSecret         string = "router-site-server"
		SiteCaSecret             string = "router-site-ca"
		ConsoleServerSecret      string = "skupper-console-certs"
		ConsoleUsersSecret       string = "skupper-console-users"
		PrometheusServerSecret   string = "skupper-prometheus-certs"
		OauthRouterConsoleSecret string = "skupper-router-console-certs"
		ServiceCaSecret          string = "skupper-service-ca"
		ServiceClientSecret      string = "skupper-service-client" // Secret that is used in sslProfiles for all http2 connectors with tls enabled
	)

	// Check if secrets already exist
	existingSiteCA := &corev1.Secret{}
	existingLocalCA := &corev1.Secret{}
	existingSiteServer := &corev1.Secret{}
	existingLocalServer := &corev1.Secret{}
	siteSecretAddress := fmt.Sprintf("%s.%s.svc.cluster.local,%s", ms.name, namespace, address)
	localSecretAddress := fmt.Sprintf("%s.%s.svc.cluster.local,%s", ms.name, namespace, address)
	siteSecretSubject := fmt.Sprintf("iofog-router")
	localSecretSubject := fmt.Sprintf("iofog-router-local")

	// Try to get existing secrets
	err = r.Client.Get(context.Background(), types.NamespacedName{Name: SiteCaSecret, Namespace: namespace}, existingSiteCA)
	siteCAExists := err == nil
	err = r.Client.Get(context.Background(), types.NamespacedName{Name: LocalCaSecret, Namespace: namespace}, existingLocalCA)
	localCAExists := err == nil
	err = r.Client.Get(context.Background(), types.NamespacedName{Name: SiteServerSecret, Namespace: namespace}, existingSiteServer)
	siteServerExists := err == nil
	err = r.Client.Get(context.Background(), types.NamespacedName{Name: LocalServerSecret, Namespace: namespace}, existingLocalServer)
	localServerExists := err == nil

	// If all secrets exist, use them
	if siteCAExists && localCAExists && siteServerExists && localServerExists {
		r.log.Info(fmt.Sprintf("Using existing secrets for Controlplane %s", r.cp.Name))
		ms.secrets = append(ms.secrets, *existingSiteServer, *existingLocalServer)
		return nil
	}

	// Generate new secrets if any are missing
	r.log.Info(fmt.Sprintf("Generating new secrets for Controlplane %s", r.cp.Name))

	var siteCA, localCA corev1.Secret

	// Generate CA certificates if they don't exist
	routerLabels := getStandardLabels("router", r.cp.Name)
	if !siteCAExists {
		r.log.Info(fmt.Sprintf("Generating site CA Secret for Controlplane %s", r.cp.Name))
		siteCA = util.GenerateSecret(SiteCaSecret, SiteCaSecret, SiteCaSecret, 0, nil)
		siteCA.Namespace = namespace
		siteCA.Labels = routerLabels
		ms.secrets = append(ms.secrets, siteCA)
	} else {
		siteCA = *existingSiteCA
	}

	if !localCAExists {
		r.log.Info(fmt.Sprintf("Generating local CA Secret for Controlplane %s", r.cp.Name))
		localCA = util.GenerateSecret(LocalCaSecret, LocalCaSecret, LocalCaSecret, 0, nil)
		localCA.Namespace = namespace
		localCA.Labels = routerLabels
		ms.secrets = append(ms.secrets, localCA)
	} else {
		localCA = *existingLocalCA
	}

	// Generate server certificates if they don't exist
	if !siteServerExists {
		r.log.Info(fmt.Sprintf("Generating site server certificate for Controlplane %s with address %s", r.cp.Name, address))
		siteSecret := util.GenerateSecret(SiteServerSecret, siteSecretSubject, siteSecretAddress, 0, &siteCA)
		siteSecret.Namespace = namespace
		siteSecret.Labels = routerLabels
		ms.secrets = append(ms.secrets, siteSecret)
	} else {
		ms.secrets = append(ms.secrets, *existingSiteServer)
	}

	if !localServerExists {
		r.log.Info(fmt.Sprintf("Generating local server certificate for Controlplane %s with address %s", r.cp.Name, address))
		localSecret := util.GenerateSecret(LocalServerSecret, localSecretSubject, localSecretAddress, 0, &localCA)
		localSecret.Namespace = namespace
		localSecret.Labels = routerLabels
		ms.secrets = append(ms.secrets, localSecret)
	} else {
		ms.secrets = append(ms.secrets, *existingLocalServer)
	}

	r.log.Info(fmt.Sprintf("Secrets generated/retrieved for Controlplane %s", r.cp.Name))

	return nil
}

func newK8sClient() (*k8sclient.Client, error) {
	kubeConf := os.Getenv("KUBECONFIG")
	if kubeConf == "" {
		return k8sclient.NewInCluster()
	}

	return k8sclient.New(kubeConf)
}
