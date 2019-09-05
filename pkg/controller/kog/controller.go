package kog

import (
	"context"
	k8sv1alpha2 "github.com/eclipse-iofog/iofog-operator/pkg/apis/k8s/v1alpha2"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_kog")

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

	// Reconcile Iofog Controller
	if err = r.reconcileIofogController(instance); err != nil {
		return reconcile.Result{}, err
	}

	// Reconcile Iofog Kubelet
	if err = r.reconcileIofogKubelet(instance); err != nil {
		return reconcile.Result{}, err
	}

	// Reconcile Connectors
	if err = r.reconcileIofogConnectors(instance); err != nil {
		return reconcile.Result{}, err
	}

	r.logger.Info("Completed Reconciliation")

	return reconcile.Result{}, nil
}
