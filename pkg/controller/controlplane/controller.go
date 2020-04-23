package controlplane

import (
	"context"
	"errors"
	"fmt"

	iofogclient "github.com/eclipse-iofog/iofog-go-sdk/v2/pkg/client"
	"github.com/eclipse-iofog/iofog-operator/v2/pkg/apis/iofog"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	b64 "encoding/base64"
)

var log = logf.Log.WithName("controller_control_plane")

// Add creates a new ControlPlane Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	iofogclient.SetGlobalRetries(iofogclient.Retries{
		CustomMessage: map[string]int{
			"timeout": 10,
			"refuse":  10,
		},
	})
	return &ReconcileControlPlane{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("control-plane-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource ControlPlane
	err = c.Watch(&source.Kind{Type: &iofog.ControlPlane{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner ControlPlane
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &iofog.ControlPlane{},
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
	client client.Client
	scheme *runtime.Scheme
	logger logr.Logger
	cp     iofog.ControlPlane
}

// Reconcile reads that state of the cluster for a ControlPlane object and makes changes based on the state read
// and what is in the ControlPlane.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileControlPlane) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	r.logger = log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	r.logger.Info("Reconciling Control Plane")

	// Fetch the ControlPlane control plane
	err := r.client.Get(context.TODO(), request.NamespacedName, &r.cp)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// Error chan for reconcile routines
	reconcilerCount := 3
	errCh := make(chan error, reconcilerCount)

	// Reconcile Router
	go reconcileRoutine(r.reconcileRouter, errCh)

	// Reconcile Iofog Controller and Kubelet
	go reconcileRoutine(r.reconcileIofogController, errCh)

	// Reconcile Port Manager
	go reconcileRoutine(r.reconcilePortManager, errCh)

	for idx := 0; idx < reconcilerCount; idx++ {
		routineErr := <-errCh
		if routineErr != nil {
			if err == nil {
				// Create new err
				err = routineErr
			} else {
				// Append
				err = errors.New(fmt.Sprintf("%s\n%s", err.Error(), routineErr.Error()))
			}
		}
	}
	if err != nil {
		return reconcile.Result{}, err
	}

	r.logger.Info("Completed Reconciliation")

	return reconcile.Result{}, nil
}

func decodeBase64(encoded string) (string, error) {
	decodedBytes, err := b64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	return string(decodedBytes), nil
}
