package controlplane

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
	"k8s.io/apimachinery/pkg/api/errors"
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

var log = logf.Log.WithName("controller_controlplane")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new ControlPlane Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileControlPlane{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("controlplane-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource ControlPlane
	err = c.Watch(&source.Kind{Type: &k8sv1alpha2.ControlPlane{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner ControlPlane
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &k8sv1alpha2.ControlPlane{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileControlPlane implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileControlPlane{}

// ReconcileControlPlane reconciles a ControlPlane object
type ReconcileControlPlane struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client      client.Client
	scheme      *runtime.Scheme
	apiEndpoint string
}

// Reconcile reads that state of the cluster for a ControlPlane object and makes changes based on the state read
// and what is in the ControlPlane.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileControlPlane) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling ControlPlane")

	// Fetch the ControlPlane instance
	instance := &k8sv1alpha2.ControlPlane{}
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
	if err = r.createIofogController(instance, reqLogger); err != nil {
		return reconcile.Result{}, err
	}

	// Create Iofog Kubelet
	if err = r.createIofogKubelet(instance, reqLogger); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileControlPlane) createIofogController(controlPlane *k8sv1alpha2.ControlPlane, logger logr.Logger) error {
	// Configure
	ms := controllerMicroservice
	ms.replicas = controlPlane.Spec.ControllerReplicaCount

	// Deployment
	if err := r.createDeployment(controlPlane, &ms, logger); err != nil {
		return err
	}
	// Service
	if err := r.createService(controlPlane, &ms, logger); err != nil {
		return err
	}
	// Connect to cluster
	k8sClient, err := k8sclient.NewClient()
	if err != nil {
		return err
	}
	// Wait for Pods
	if err = k8sClient.WaitForPod(controlPlane.ObjectMeta.Namespace, controllerMicroservice.name); err != nil {
		return err
	}
	// Wait for Service
	ip, err := k8sClient.WaitForService(controlPlane.ObjectMeta.Namespace, controllerMicroservice.name)
	if err != nil {
		return err
	}
	r.apiEndpoint = fmt.Sprintf("%s:%d", ip, controllerMicroservice.ports[0])
	if err = r.waitForControllerAPI(); err != nil {
		return err
	}
	// Set up user
	if err = r.createIofogUser(&controlPlane.Spec.IofogUser); err != nil {
		return err
	}

	return nil
}

func (r *ReconcileControlPlane) createIofogKubelet(controlPlane *k8sv1alpha2.ControlPlane, logger logr.Logger) error {
	// Get Kubelet token
	token, err := r.getKubeletToken(&controlPlane.Spec.IofogUser)
	if err != nil {
		return err
	}

	// Configure
	ms := kubeletMicroservice
	ms.containers[0].args = []string{
		"--namespace",
		controlPlane.ObjectMeta.Namespace,
		"--iofog-token",
		token,
		"--iofog-url",
		fmt.Sprintf("http://%s:%d", controllerMicroservice.name, controllerMicroservice.ports[0]),
	}

	// Service Account
	if err := r.createServiceAccount(controlPlane, &kubeletMicroservice, logger); err != nil {
		return err
	}
	// ClusterRoleBinding
	if err := r.createClusterRoleBinding(controlPlane, &kubeletMicroservice, logger); err != nil {
		return err
	}
	// Deployment
	if err := r.createDeployment(controlPlane, &kubeletMicroservice, logger); err != nil {
		return err
	}

	return nil
}

func (r *ReconcileControlPlane) createDeployment(controlPlane *k8sv1alpha2.ControlPlane, ms *microservice, logger logr.Logger) error {
	dep := newDeployment(controlPlane.ObjectMeta.Namespace, ms)
	// Set ControlPlane instance as the owner and controller
	if err := controllerutil.SetControllerReference(controlPlane, dep, r.scheme); err != nil {
		return err
	}

	// Check if this resource already exists
	found := &appsv1.Deployment{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: dep.Name, Namespace: dep.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		logger.Info("Creating a new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
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
	logger.Info("Skip reconcile: Deployment already exists", "Deployment.Namespace", found.Namespace, "Deployment.Name", found.Name)
	return nil
}

func (r *ReconcileControlPlane) createService(controlPlane *k8sv1alpha2.ControlPlane, ms *microservice, logger logr.Logger) error {
	svc := newService(controlPlane.ObjectMeta.Namespace, ms)
	// Set ControlPlane instance as the owner and controller
	if err := controllerutil.SetControllerReference(controlPlane, svc, r.scheme); err != nil {
		return err
	}

	// Check if this resource already exists
	found := &corev1.Service{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: svc.Name, Namespace: svc.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		logger.Info("Creating a new Service", "Service.Namespace", svc.Namespace, "Service.Name", svc.Name)
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
	logger.Info("Skip reconcile: Service already exists", "Service.Namespace", found.Namespace, "Service.Name", found.Name)
	return nil
}

func (r *ReconcileControlPlane) createServiceAccount(controlPlane *k8sv1alpha2.ControlPlane, ms *microservice, logger logr.Logger) error {
	svcAcc := newServiceAccount(controlPlane.ObjectMeta.Namespace, ms)

	// Set ControlPlane instance as the owner and controller
	if err := controllerutil.SetControllerReference(controlPlane, svcAcc, r.scheme); err != nil {
		return err
	}

	// Check if this resource already exists
	found := &corev1.Service{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: svcAcc.Name, Namespace: svcAcc.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		logger.Info("Creating a new Service Account", "ServiceAccount.Namespace", svcAcc.Namespace, "ServiceAccount.Name", svcAcc.Name)
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
	logger.Info("Skip reconcile: Service Account already exists", "ServiceAccount.Namespace", found.Namespace, "ServiceAccount.Name", found.Name)
	return nil
}

func (r *ReconcileControlPlane) createClusterRoleBinding(controlPlane *k8sv1alpha2.ControlPlane, ms *microservice, logger logr.Logger) error {
	crb := newClusterRoleBinding(controlPlane.ObjectMeta.Namespace, ms)

	// Set ControlPlane instance as the owner and controller
	if err := controllerutil.SetControllerReference(controlPlane, crb, r.scheme); err != nil {
		return err
	}

	// Check if this resource already exists
	found := &corev1.Service{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: crb.Name, Namespace: crb.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		logger.Info("Creating a new Cluste Role Binding", "ClusterRoleBinding.Namespace", crb.Namespace, "ClusterRoleBinding.Name", crb.Name)
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
	logger.Info("Skip reconcile: Cluster Role Binding already exists", "ClusterRoleBinding.Namespace", found.Namespace, "ClusterRoleBinding.Name", found.Name)
	return nil
}

func (r *ReconcileControlPlane) waitForControllerAPI() (err error) {
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

func (r *ReconcileControlPlane) createIofogUser(user *k8sv1alpha2.IofogUser) (err error) {
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

func (r *ReconcileControlPlane) getKubeletToken(user *k8sv1alpha2.IofogUser) (token string, err error) {
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
