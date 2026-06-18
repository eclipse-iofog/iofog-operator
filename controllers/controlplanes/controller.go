package controllers

import (
	"context"

	iofogclient "github.com/eclipse-iofog/iofog-go-sdk/v3/pkg/client"
	op "github.com/eclipse-iofog/iofog-go-sdk/v3/pkg/k8s/operator"
	cpv3 "github.com/eclipse-iofog/iofog-operator/v3/apis/controlplanes/v3"
	"github.com/go-logr/logr"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ControlPlaneReconciler reconciles a ControlPlane object.
type ControlPlaneReconciler struct {
	client.Client
	Log    logr.Logger
	log    logr.Logger
	Scheme *runtime.Scheme
	cp     cpv3.ControlPlane
}

// +kubebuilder:rbac:groups=datasance.com,resources=controlplanes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=datasance.com,resources=controlplanes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=datasance.com,resources=controlplanes/finalizers,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete

func (r *ControlPlaneReconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	r.log = r.Log.WithValues("controlplane", request.NamespacedName)

	// Fetch the ControlPlane control plane
	if err := r.Client.Get(ctx, request.NamespacedName, &r.cp); err != nil {
		if k8serrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return op.DoNotRequeue()
		}
		// Error reading the object - requeue the request.
		return op.RequeueWithError(err)
	}

	// Reconcile based on state
	reconciler, err := r.getReconcileFunc(ctx)
	if err != nil {
		return op.RequeueWithError(err)
	}

	recon := reconciler(ctx)

	return recon.Result()
}

func (r *ControlPlaneReconciler) SetupWithManager(mgr ctrl.Manager) error {
	iofogclient.SetGlobalRetries(iofogclient.Retries{
		Timeout: 0,
		CustomMessage: map[string]int{
			"timeout": 0,
			"refuse":  0,
		},
	})

	return ctrl.NewControllerManagedBy(mgr).
		For(&cpv3.ControlPlane{}).
		Complete(r)
}
