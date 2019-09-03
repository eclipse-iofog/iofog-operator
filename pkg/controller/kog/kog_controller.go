package kog

import (
	"context"
	"fmt"
	"strings"
	"time"

	k8sv1alpha2 "github.com/eclipse-iofog/iofog-operator/pkg/apis/k8s/v1alpha2"
	k8sclient "github.com/eclipse-iofog/iofog-operator/pkg/controller/client"

	iofogclient "github.com/eclipse-iofog/iofogctl/pkg/iofog/client"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_kog")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new Kog Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileKog{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("kog-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Kog
	err = c.Watch(&source.Kind{Type: &k8sv1alpha2.Kog{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner Kog
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &k8sv1alpha2.Kog{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileKog implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileKog{}

// ReconcileKog reconciles a Kog object
type ReconcileKog struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client      client.Client
	scheme      *runtime.Scheme
	logger      logr.Logger
	apiEndpoint string
}

// Reconcile reads that state of the cluster for a Kog object and makes changes based on the state read
// and what is in the Kog.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileKog) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	r.logger = log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	r.logger.Info("Reconciling Control Plane")

	// Fetch the Kog instance
	instance := &k8sv1alpha2.Kog{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// Create Iofog Controller
	if err = r.createIofogController(instance); err != nil {
		return reconcile.Result{}, err
	}

	// Create Iofog Kubelet
	if err = r.createIofogKubelet(instance); err != nil {
		return reconcile.Result{}, err
	}

	// Create Connectors
	if err = r.createIofogConnectors(instance); err != nil {
		return reconcile.Result{}, err
	}

	r.logger.Info("Completed Reconciliation")

	return reconcile.Result{}, nil
}

func (r *ReconcileKog) createIofogConnectors(kog *k8sv1alpha2.Kog) error {

	// Connect to cluster
	k8sClient, err := k8sclient.NewClient()
	if err != nil {
		return err
	}

	// Find the current state to compare against requested state
	depList := &appsv1.DeploymentList{}
	if err = r.client.List(context.Background(), &client.ListOptions{}, depList); err != nil {
		return err
	}
	// Determine which connectors to create and delete
	createConnectors := make(map[string]bool)
	deleteConnectors := make(map[string]bool)
	for _, connector := range kog.Spec.Connectors.Instances {
		name := prefixConnectorName(connector.Name)
		createConnectors[name] = true
		deleteConnectors[name] = false
	}
	for _, dep := range depList.Items {
		if strings.Contains(dep.ObjectMeta.Name, getConnectorNamePrefix()) {
			createConnectors[dep.ObjectMeta.Name] = false
			if _, exists := deleteConnectors[dep.ObjectMeta.Name]; !exists {
				deleteConnectors[dep.ObjectMeta.Name] = true
			}
		}
	}
	// Delete connectors
	for k, v := range deleteConnectors {
		if v {
			meta := metav1.ObjectMeta{
				Name:      k,
				Namespace: kog.ObjectMeta.Namespace,
			}
			// Delete deployment
			dep := &appsv1.Deployment{ObjectMeta: meta}
			if err = r.client.Delete(context.Background(), dep); err != nil {
				return err
			}

			// Delete service
			svc := &corev1.Service{ObjectMeta: meta}
			if err = r.client.Delete(context.Background(), svc); err != nil {
				return err
			}

			// Delete service account
			svcAcc := &corev1.ServiceAccount{ObjectMeta: meta}
			if err = r.client.Delete(context.Background(), svcAcc); err != nil {
				return err
			}
		}
	}
	// Create connectors
	for k, v := range createConnectors {
		if v {
			ms := newConnectorMicroservice(kog.Spec.Connectors.Image)
			ms.name = k
			// Create
			// Service Account
			if err := r.createServiceAccount(kog, ms); err != nil {
				return err
			}
			// Deployment
			if err := r.createDeployment(kog, ms); err != nil {
				return err
			}
			// Service
			if err := r.createService(kog, ms); err != nil {
				return err
			}

			// Wait for Pods
			if err = k8sClient.WaitForPod(kog.ObjectMeta.Namespace, ms.name, 120); err != nil {
				return err
			}
			// Wait for Service
			ip, err := k8sClient.WaitForService(kog.ObjectMeta.Namespace, ms.name, 240)
			if err != nil {
				return err
			}
			// Log into Controller
			iofogClient := iofogclient.New(r.apiEndpoint)
			if err = iofogClient.Login(iofogclient.LoginRequest{
				Email:    kog.Spec.ControlPlane.IofogUser.Email,
				Password: kog.Spec.ControlPlane.IofogUser.Password,
			}); err != nil {
				return err
			}
			// Provision the Connector
			if err = iofogClient.AddConnector(iofogclient.ConnectorInfo{
				IP:     ip,
				Domain: ip,
				Name:   removeConnectorNamePrefix(k),
			}); err != nil {
				return err
			}
		}
	}

	return nil
}

func (r *ReconcileKog) createIofogController(kog *k8sv1alpha2.Kog) error {
	// Configure
	ms := newControllerMicroservice(kog.Spec.ControlPlane.ControllerReplicaCount, kog.Spec.ControlPlane.ControllerImage)
	r.apiEndpoint = fmt.Sprintf("%s:%d", ms.name, ms.ports[0])

	// Service Account
	if err := r.createServiceAccount(kog, ms); err != nil {
		return err
	}

	// Deployment
	if err := r.createDeployment(kog, ms); err != nil {
		return err
	}
	// Service
	if err := r.createService(kog, ms); err != nil {
		return err
	}
	// Connect to cluster
	k8sClient, err := k8sclient.NewClient()
	if err != nil {
		return err
	}
	// Wait for Pods
	if err = k8sClient.WaitForPod(kog.ObjectMeta.Namespace, ms.name, 120); err != nil {
		return err
	}
	// Wait for Service
	_, err = k8sClient.WaitForService(kog.ObjectMeta.Namespace, ms.name, 240)
	if err != nil {
		return err
	}
	if err = r.waitForControllerAPI(); err != nil {
		return err
	}
	// Set up user
	if err = r.createIofogUser(&kog.Spec.ControlPlane.IofogUser); err != nil {
		return err
	}

	return nil
}

func (r *ReconcileKog) createIofogKubelet(kog *k8sv1alpha2.Kog) error {
	// Get Kubelet token
	token, err := r.getKubeletToken(&kog.Spec.ControlPlane.IofogUser)
	if err != nil {
		return err
	}

	// Configure
	ms := newKubeletMicroservice(kog.Spec.ControlPlane.KubeletImage, kog.ObjectMeta.Namespace, token, r.apiEndpoint)

	// Service Account
	if err := r.createServiceAccount(kog, ms); err != nil {
		return err
	}
	// ClusterRoleBinding
	if err := r.createClusterRoleBinding(kog, ms); err != nil {
		return err
	}
	// Deployment
	if err := r.createDeployment(kog, ms); err != nil {
		return err
	}

	return nil
}

func (r *ReconcileKog) createDeployment(kog *k8sv1alpha2.Kog, ms *microservice) error {
	dep := newDeployment(kog.ObjectMeta.Namespace, ms)
	// Set Kog instance as the owner and controller
	if err := controllerutil.SetControllerReference(kog, dep, r.scheme); err != nil {
		return err
	}

	// Check if this resource already exists
	found := &appsv1.Deployment{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: dep.Name, Namespace: dep.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		r.logger.Info("Creating a new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
		err = r.client.Create(context.TODO(), dep)
		if err != nil {
			return err
		}

		// Resource created successfully - don't requeue
		return nil
	} else if err != nil {
		return err
	}

	// Resource already exists - don't requeue
	r.logger.Info("Skip reconcile: Deployment already exists", "Deployment.Namespace", found.Namespace, "Deployment.Name", found.Name)
	return nil
}

func (r *ReconcileKog) createService(kog *k8sv1alpha2.Kog, ms *microservice) error {
	svc := newService(kog.ObjectMeta.Namespace, ms)
	// Set Kog instance as the owner and controller
	if err := controllerutil.SetControllerReference(kog, svc, r.scheme); err != nil {
		return err
	}

	// Check if this resource already exists
	found := &corev1.Service{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: svc.Name, Namespace: svc.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		r.logger.Info("Creating a new Service", "Service.Namespace", svc.Namespace, "Service.Name", svc.Name)
		err = r.client.Create(context.TODO(), svc)
		if err != nil {
			return err
		}

		// Resource created successfully - don't requeue
		return nil
	} else if err != nil {
		return err
	}

	// Resource already exists - don't requeue
	r.logger.Info("Skip reconcile: Service already exists", "Service.Namespace", found.Namespace, "Service.Name", found.Name)
	return nil
}

func (r *ReconcileKog) createServiceAccount(kog *k8sv1alpha2.Kog, ms *microservice) error {
	svcAcc := newServiceAccount(kog.ObjectMeta.Namespace, ms)

	// Set Kog instance as the owner and controller
	if err := controllerutil.SetControllerReference(kog, svcAcc, r.scheme); err != nil {
		return err
	}

	// Check if this resource already exists
	found := &corev1.ServiceAccount{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: svcAcc.Name, Namespace: svcAcc.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		r.logger.Info("Creating a new Service Account", "ServiceAccount.Namespace", svcAcc.Namespace, "ServiceAccount.Name", svcAcc.Name)
		err = r.client.Create(context.TODO(), svcAcc)
		if err != nil {
			return err
		}

		// Resource created successfully - don't requeue
		return nil
	} else if err != nil {
		return err
	}

	// Resource already exists - don't requeue
	r.logger.Info("Skip reconcile: Service Account already exists", "ServiceAccount.Namespace", found.Namespace, "ServiceAccount.Name", found.Name)
	return nil
}

func (r *ReconcileKog) createClusterRoleBinding(kog *k8sv1alpha2.Kog, ms *microservice) error {
	crb := newClusterRoleBinding(kog.ObjectMeta.Namespace, ms)

	// Set Kog instance as the owner and controller
	if err := controllerutil.SetControllerReference(kog, crb, r.scheme); err != nil {
		return err
	}

	// Check if this resource already exists
	found := &rbacv1.ClusterRoleBinding{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: crb.Name, Namespace: crb.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		r.logger.Info("Creating a new Cluste Role Binding", "ClusterRoleBinding.Namespace", crb.Namespace, "ClusterRoleBinding.Name", crb.Name)
		err = r.client.Create(context.TODO(), crb)
		if err != nil {
			return err
		}

		// Resource created successfully - don't requeue
		return nil
	} else if err != nil {
		return err
	}

	// Resource already exists - don't requeue
	r.logger.Info("Skip reconcile: Cluster Role Binding already exists", "ClusterRoleBinding.Namespace", found.Namespace, "ClusterRoleBinding.Name", found.Name)
	return nil
}

func (r *ReconcileKog) waitForControllerAPI() (err error) {
	iofogClient := iofogclient.New(r.apiEndpoint)

	connected := false
	iter := 0
	for !connected {
		// Time out
		if iter > 60 {
			err = errors.NewTimeoutError("Timed out waiting for Controller API", iter)
			return
		}
		// Check the status endpoint
		if _, err = iofogClient.GetStatus(); err != nil {
			// Retry if connection is refused, this is usually only necessary on K8s Controller
			if strings.Contains(err.Error(), "connection refused") {
				time.Sleep(time.Millisecond * 1000)
				iter = iter + 1
				continue
			}
			// Return the error otherwise
			return
		}
		// No error, connected
		connected = true
		continue
	}

	return
}

func (r *ReconcileKog) createIofogUser(user *k8sv1alpha2.IofogUser) (err error) {
	iofogClient := iofogclient.New(r.apiEndpoint)

	if err = iofogClient.CreateUser(iofogclient.User(*user)); err != nil {
		// If not error about account existing, fail
		if !strings.Contains(err.Error(), "already an account associated") {
			return err
		}
		// Try to log in
		if err = iofogClient.Login(iofogclient.LoginRequest{
			Email:    user.Email,
			Password: user.Password,
		}); err != nil {
			return err
		}
	}

	return nil
}

func (r *ReconcileKog) getKubeletToken(user *k8sv1alpha2.IofogUser) (token string, err error) {
	iofogClient := iofogclient.New(r.apiEndpoint)
	if err = iofogClient.Login(iofogclient.LoginRequest{
		Email:    user.Email,
		Password: user.Password,
	}); err != nil {
		return
	}
	token = iofogClient.GetAccessToken()
	return
}
