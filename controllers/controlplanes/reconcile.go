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
	"github.com/eclipse-iofog/iofog-operator/v3/controllers/controlplanes/router"
	"github.com/skupperproject/skupper-cli/pkg/certs"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

const (
	loadBalancerTimeout       = 360
	requeueDuration           = time.Second * 5
	errProxyRouterMissing     = "missing Proxy.Router data for non LoadBalancer Router service"
	errParseControllerURL     = "failed to parse Controller endpoint as URL (%s): %s"
	portManagerDeploymentName = "port-manager"
)

func reconcileRoutine(recon func() op.Reconciliation, reconChan chan op.Reconciliation) {
	reconChan <- recon()
}

func (r *ControlPlaneReconciler) updateIofogUserPassword(iofogClient *iofogclient.Client) error {
	r.log.Info(fmt.Sprintf("Updating user password %s for ControlPlane %s", r.cp.Spec.User.Password, r.cp.Name))
	// Retrieve old password from secrets
	found := &corev1.Secret{}

	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: controllerCredentialsSecretName, Namespace: r.cp.Namespace}, found)
	if err != nil {
		return err
	}
	// Try to log in with old password
	passwordBytes, ok := found.Data[passwordSecretKey]
	if !ok {
		return fmt.Errorf("password secret key %s not found in secret %s", passwordSecretKey, controllerCredentialsSecretName)
	}

	oldPassword, err := DecodeBase64(string(passwordBytes))
	if err != nil {
		return fmt.Errorf("password %s in secret %s is not a valid base64 string", string(passwordBytes), controllerCredentialsSecretName)
	}

	emailBytes, ok := found.Data[emailSecretKey]
	if !ok {
		return fmt.Errorf("email secret key %s not found in secret %s", emailSecretKey, controllerCredentialsSecretName)
	}

	email := string(emailBytes)

	if err := iofogClient.Login(iofogclient.LoginRequest{
		Email:    email,
		Password: oldPassword,
	}); err != nil {
		r.log.Info(fmt.Sprintf("Failed to log in with old credentials for ControlPlane %s: %s %s", r.cp.Name, email, oldPassword))

		return err
	}
	// Update password
	newPassword, err := DecodeBase64(r.cp.Spec.User.Password)
	if err != nil {
		return fmt.Errorf("new password %s for ControlPlane %s is not a valid base64 string", r.cp.Name, r.cp.Spec.User.Password)
	}

	if err := r.updateIofogUser(iofogClient, oldPassword, newPassword); err != nil {
		return err
	}

	// Update secret
	found.StringData = map[string]string{
		passwordSecretKey: r.cp.Spec.User.Password,
		emailSecretKey:    r.cp.Spec.User.Email,
	}
	if err := r.Client.Update(context.TODO(), found); err != nil {
		return err
	}

	// Restart required pods
	if err := r.restartPodsForDeployment(portManagerDeploymentName, r.cp.Namespace); err != nil {
		return err
	}

	return nil
}

func (r *ControlPlaneReconciler) reconcileDBCredentialsSecret(ms *microservice) (shouldRestartPod bool, err error) {
	for i := range ms.secrets {
		secret := &ms.secrets[i]

		if secret.Name == controllerDBCredentialsSecretName {
			found := &corev1.Secret{}

			err := r.Client.Get(context.TODO(), types.NamespacedName{Name: secret.Name, Namespace: secret.Namespace}, found)
			if err != nil {
				if !k8serrors.IsNotFound(err) {
					return false, err
				}
				// Create secret
				err = r.Client.Create(context.TODO(), secret)
				if err != nil {
					return false, err
				}

				return false, nil
			}
			// Secret already exists
			// Update secret
			err = r.Client.Update(context.TODO(), secret)
			if err != nil {
				return false, err
			}
			// Restart pod
			return true, nil
		}
	}

	return false, nil
}

func (r *ControlPlaneReconciler) reconcileIofogController() op.Reconciliation {
	// Configure Controller
	config := &controllerMicroserviceConfig{
		replicas:          r.cp.Spec.Replicas.Controller,
		image:             r.cp.Spec.Images.Controller,
		imagePullSecret:   r.cp.Spec.Images.PullSecret,
		proxyImage:        r.cp.Spec.Images.Proxy,
		routerImage:       r.cp.Spec.Images.Router,
		db:                &r.cp.Spec.Database,
		serviceType:       r.cp.Spec.Services.Controller.Type,
		loadBalancerAddr:  r.cp.Spec.Services.Controller.Address,
		portAllocatorHost: r.cp.Spec.Controller.PortAllocatorHost,
		ecn:               r.cp.Spec.Controller.ECNName,
		pidBaseDir:        r.cp.Spec.Controller.PidBaseDir,
		ecnViewerPort:     r.cp.Spec.Controller.EcnViewerPort,
		ecnViewerURL:      r.cp.Spec.Controller.EcnViewerURL,
		portProvider:      r.cp.Spec.Controller.PortProvider,
	}
	ms := newControllerMicroservice(r.cp.Namespace, config)

	// Service Account
	if err := r.createServiceAccount(ms); err != nil {
		return op.ReconcileWithError(err)
	}

	// Handle DB credentials secret
	shouldRestartPods, err := r.reconcileDBCredentialsSecret(ms)
	if err != nil {
		return op.ReconcileWithError(err)
	}
	// Create secrets
	r.log.Info(fmt.Sprintf("Creating secrets for controller reconcile for Controlplane %s", r.cp.Name))

	if err := r.createSecrets(ms); err != nil {
		r.log.Info(fmt.Sprintf("Failed to create secrets %v for controller reconcile for Controlplane %s", err, r.cp.Name))

		return op.ReconcileWithError(err)
	}

	// Service
	if err := r.createService(ms); err != nil {
		return op.ReconcileWithError(err)
	}

	// PVC
	if err := r.createPersistentVolumeClaims(ms); err != nil {
		return op.ReconcileWithError(err)
	}

	alreadyExists, err := r.deploymentExists(r.cp.Namespace, ms.name)
	if err != nil {
		return op.ReconcileWithError(err)
	}

	// Deployment
	if err := r.createDeployment(ms); err != nil {
		return op.ReconcileWithError(err)
	}

	// The deployment was just created, requeue to hide latency
	if !alreadyExists {
		return op.ReconcileWithRequeue(requeueDuration)
	}
	// Instantiate Controller client
	ctrlPort, err := getControllerPort(ms)
	if err != nil {
		return op.ReconcileWithError(err)
	}

	host := fmt.Sprintf("%s.%s.svc.cluster.local", ms.name, r.cp.ObjectMeta.Namespace)
	iofogClient, fin := r.getIofogClient(host, ctrlPort)

	if fin.IsFinal() {
		return fin
	}

	// Set up user
	if err := r.createIofogUser(iofogClient); err != nil {
		if !strings.Contains(strings.ToLower(err.Error()), "invalid credentials") {
			r.log.Info(fmt.Sprintf("Could not create user for ControlPlane %s: %s", r.cp.Name, err.Error()))

			return op.ReconcileWithRequeue(requeueDuration)
		}
		// If the error is invalid credentials, update user password
		if err := r.updateIofogUserPassword(iofogClient); err != nil {
			r.log.Info(fmt.Sprintf("Could not update user for ControlPlane %s: %s", r.cp.Name, err.Error()))

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

	// Wait for Controller LB to actually work

	r.log.Info(fmt.Sprintf("Waiting for IP/LB Service in iofog-controller reconcile for ControlPlane %s", r.cp.Name))

	if strings.EqualFold(r.cp.Spec.Services.Controller.Type, string(corev1.ServiceTypeLoadBalancer)) {
		host, err := k8sClient.WaitForLoadBalancer(r.cp.Namespace, controllerName, loadBalancerTimeout)
		if err != nil {
			return op.ReconcileWithError(err)
		}
		// Check LB connection works
		if _, fin := r.getIofogClient(host, ctrlPort); fin.IsFinal() {
			r.log.Info(fmt.Sprintf("LB Connection works for ControlPlane %s", r.cp.Name))

			return fin
		}
	}

	if shouldRestartPods {
		r.log.Info(fmt.Sprintf("Restarting pods for ControlPlane %s", r.cp.Name))

		if err := r.restartPodsForDeployment(ms.name, r.cp.Namespace); err != nil {
			return op.ReconcileWithError(err)
		}
	}

	r.log.Info(fmt.Sprintf("op.Continue in iofog-controller reconcile for ControlPlane %s", r.cp.Name))

	return op.Continue()
}

func (r *ControlPlaneReconciler) getIofogClient(host string, port int) (*iofogclient.Client, op.Reconciliation) {
	baseURL := fmt.Sprintf("http://%s:%d/api/v3", host, port) //nolint:nosprintfhostport

	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, op.ReconcileWithError(fmt.Errorf(errParseControllerURL, baseURL, err.Error()))
	}

	iofogClient := iofogclient.New(iofogclient.Options{
		BaseURL: parsedURL,
		Timeout: 1,
	})

	if _, err = iofogClient.GetStatus(); err != nil {
		r.log.Info(fmt.Sprintf("Could not get Controller status for ControlPlane %s: %s", r.cp.Name, err.Error()))

		return nil, op.ReconcileWithRequeue(requeueDuration)
	}

	return iofogClient, op.Continue()
}

func (r *ControlPlaneReconciler) reconcilePortManager() op.Reconciliation {
	ms := newPortManagerMicroservice(&portManagerConfig{
		image:            r.cp.Spec.Images.PortManager,
		proxyImage:       r.cp.Spec.Images.Proxy,
		httpProxyAddress: r.cp.Spec.Ingresses.HTTPProxy.Address,
		tcpProxyAddress:  r.cp.Spec.Ingresses.TCPProxy.Address,
		watchNamespace:   r.cp.ObjectMeta.Namespace,
		userEmail:        r.cp.Spec.User.Email,
		userPass:         r.cp.Spec.User.Password,
	})

	// Service Account
	if err := r.createServiceAccount(ms); err != nil {
		return op.ReconcileWithError(err)
	}
	// Role
	if err := r.createRole(ms); err != nil {
		return op.ReconcileWithError(err)
	}
	// RoleBinding
	if err := r.createRoleBinding(ms); err != nil {
		return op.ReconcileWithError(err)
	}

	// Create secrets
	r.log.Info(fmt.Sprintf("Creating secrets for port-manager reconcile for Controlplane %s", r.cp.Name))

	if err := r.createSecrets(ms); err != nil {
		r.log.Info(fmt.Sprintf("Failed to create secrets %v for port-manager reconcile for Controlplane %s", err, r.cp.Name))

		return op.ReconcileWithError(err)
	}

	// Deployment
	if err := r.createDeployment(ms); err != nil {
		return op.ReconcileWithError(err)
	}

	return op.Continue()
}

func (r *ControlPlaneReconciler) reconcileRouter() op.Reconciliation {
	// Configure
	volumeMountPath := "/etc/qpid-dispatch-certs/"
	ms := newRouterMicroservice(routerMicroserviceConfig{
		image:           r.cp.Spec.Images.Router,
		serviceType:     r.cp.Spec.Services.Router.Type,
		volumeMountPath: volumeMountPath,
	})

	// Service Account
	if err := r.createServiceAccount(ms); err != nil {
		return op.ReconcileWithError(err)
	}

	// Role
	if err := r.createRole(ms); err != nil {
		return op.ReconcileWithError(err)
	}

	// Role binding
	if err := r.createRoleBinding(ms); err != nil {
		return op.ReconcileWithError(err)
	}

	// Service
	if err := r.createService(ms); err != nil {
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

	if strings.EqualFold(r.cp.Spec.Services.Controller.Type, string(corev1.ServiceTypeLoadBalancer)) {
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

	// Secrets
	if err = r.createRouterSecrets(ms, address); err != nil {
		return op.ReconcileWithError(err)
	}

	// Create secrets
	r.log.Info(fmt.Sprintf("Creating secrets for router reconcile for Controlplane %s", r.cp.Name))

	if err := r.createSecrets(ms); err != nil {
		r.log.Info(fmt.Sprintf("Failed to create secrets %v for router reconcile for Controlplane %s", err, r.cp.Name))

		return op.ReconcileWithError(err)
	}

	// Deployment
	r.log.Info(fmt.Sprintf("Creating deployment for router reconcile for Controlplane %s", r.cp.Name))

	if err := r.createDeployment(ms); err != nil {
		r.log.Info(fmt.Sprintf("Failed to create deployment %v for router reconcile for Controlplane %s", err, r.cp.Name))

		return op.ReconcileWithError(err)
	}

	r.log.Info(fmt.Sprintf("op.Continue for router reconcile for Controlplane %s", r.cp.Name))

	return op.Continue()
}

func (r *ControlPlaneReconciler) createRouterSecrets(ms *microservice, address string) (err error) {
	r.log.Info(fmt.Sprintf("Creating routerSecrets definition for router reconcile for Controlplane %s", r.cp.Name))

	defer func() {
		if recoverResult := recover(); recoverResult != nil {
			r.log.Info(fmt.Sprintf("Recover result %v for creating secrets for router reconcile for Controlplane %s", recoverResult, r.cp.Name))
			err = fmt.Errorf("createRouterSecrets failed: %v", recoverResult)
		}
	}()
	// CA

	r.log.Info(fmt.Sprintf("Generating CA Secret secrets for router reconcile for Controlplane %s", r.cp.Name))

	caName := "router-ca"
	caSecret := certs.GenerateCASecret(caName, caName)
	caSecret.ObjectMeta.Namespace = r.cp.ObjectMeta.Namespace
	ms.secrets = append(ms.secrets, caSecret)

	// AMQPS and Internal
	for _, suffix := range []string{"amqps", "internal"} {
		r.log.Info(fmt.Sprintf("Generating %s Secret secrets for router reconcile for Controlplane %s", suffix, r.cp.Name))
		secret := certs.GenerateSecret("router-"+suffix, address, address, &caSecret)
		secret.ObjectMeta.Namespace = r.cp.ObjectMeta.Namespace
		ms.secrets = append(ms.secrets, secret)
	}

	r.log.Info(fmt.Sprintf("Secrets generated for Controlplane %s", r.cp.Name))

	return err
}

func newK8sClient() (*k8sclient.Client, error) {
	kubeConf := os.Getenv("KUBECONFIG")
	if kubeConf == "" {
		return k8sclient.NewInCluster()
	}

	return k8sclient.New(kubeConf)
}
