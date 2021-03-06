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

package controllers

import (
	"context"
	b64 "encoding/base64"
	"fmt"
	"time"

	iofogclient "github.com/eclipse-iofog/iofog-go-sdk/v2/pkg/client"
	op "github.com/eclipse-iofog/iofog-go-sdk/v2/pkg/k8s/operator"
	"github.com/go-logr/logr"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	cond "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cpv2 "github.com/eclipse-iofog/iofog-operator/v2/apis/controlplanes/v2"
)

const (
	conditionReady = "ready"
)

// ControlPlaneReconciler reconciles a ControlPlane object
type ControlPlaneReconciler struct {
	client.Client
	Log    logr.Logger
	log    logr.Logger
	Scheme *runtime.Scheme
	cp     cpv2.ControlPlane
}

// +kubebuilder:rbac:groups=iofog.org,resources=controlplanes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=iofog.org,resources=controlplanes/status,verbs=get;update;patch

func (r *ControlPlaneReconciler) Reconcile(request ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
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
	if cond.IsStatusConditionPresentAndEqual(r.cp.Status.Conditions, conditionReady, metav1.ConditionTrue) {
		r.log.Info("Completed Reconciliation")
		return op.DoNotRequeue()
	}

	// Error chan for reconcile routines
	reconcilerCount := 3
	reconChan := make(chan op.Reconciliation, reconcilerCount)

	// Reconcile Router
	go reconcileRoutine(r.reconcileRouter, reconChan)

	// Reconcile Iofog Controller and Kubelet
	go reconcileRoutine(r.reconcileIofogController, reconChan)

	// Reconcile Port Manager
	go reconcileRoutine(r.reconcilePortManager, reconChan)

	finRecon := op.Reconciliation{}
	for idx := 0; idx < reconcilerCount; idx++ {
		recon := <-reconChan
		if recon.Err != nil {
			if finRecon.Err == nil {
				// Create new err
				finRecon.Err = recon.Err
			} else {
				// Append
				finRecon.Err = fmt.Errorf("%s\n%s", finRecon.Err.Error(), recon.Err.Error())
			}
		}
		// End overrides all
		if recon.End {
			finRecon.End = true
		}
		// Record largest requeue
		if recon.Requeue {
			finRecon.Requeue = true
			if recon.Delay > finRecon.Delay {
				finRecon.Delay = recon.Delay
			}
		}
	}
	if finRecon.IsFinal() {
		return finRecon.Result()
	}

	if !cond.IsStatusConditionPresentAndEqual(r.cp.Status.Conditions, conditionReady, metav1.ConditionTrue) {
		// Overwrite
		r.cp.Status.Conditions = []metav1.Condition{
			{
				Type:               conditionReady,
				Status:             metav1.ConditionTrue,
				LastTransitionTime: metav1.NewTime(time.Now()),
			},
		}
		if err := r.Update(ctx, &r.cp); err != nil {
			return op.RequeueWithError(err)
		}
		// Update will trigger reconciliation again
		return op.DoNotRequeue()
	}

	return op.DoNotRequeue()
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
		For(&cpv2.ControlPlane{}).
		Complete(r)
}

func decodeBase64(encoded string) (string, error) {
	decodedBytes, err := b64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	return string(decodedBytes), nil
}
