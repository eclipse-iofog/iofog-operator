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
	"github.com/go-logr/logr"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	cond "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cpv2 "github.com/eclipse-iofog/iofog-operator/v2/apis/controlplanes/v2"
	ctrls "github.com/eclipse-iofog/iofog-operator/v2/controllers"
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
	err := r.Client.Get(ctx, request.NamespacedName, &r.cp)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return ctrls.DoNotRequeue()
		}
		// Error reading the object - requeue the request.
		return ctrls.RequeueWithError(err)
	}

	// Error chan for reconcile routines
	reconcilerCount := 3
	reconChan := make(chan ctrls.Reconciliation, reconcilerCount)

	// Reconcile Router
	go reconcileRoutine(r.reconcileRouter, reconChan)

	// Reconcile Iofog Controller and Kubelet
	go reconcileRoutine(r.reconcileIofogController, reconChan)

	// Reconcile Port Manager
	go reconcileRoutine(r.reconcilePortManager, reconChan)

	result := ctrl.Result{}
	for idx := 0; idx < reconcilerCount; idx++ {
		recon := <-reconChan
		if recon.Err != nil {
			if err == nil {
				// Create new err
				err = recon.Err
			} else {
				// Append
				err = fmt.Errorf("%s\n%s", err.Error(), recon.Err.Error())
			}
		}
		// Use largest requeue
		if recon.Result.RequeueAfter > result.RequeueAfter {
			result = recon.Result
		}
	}
	if err != nil || result.Requeue {
		return result, err
	}

	conditionReady := "ready"
	if !cond.IsStatusConditionPresentAndEqual(r.cp.Status.Conditions, conditionReady, metav1.ConditionTrue) {
		newCondition := metav1.Condition{
			Type:               conditionReady,
			Status:             metav1.ConditionTrue,
			LastTransitionTime: metav1.NewTime(time.Now()),
		}
		cond.SetStatusCondition(&r.cp.Status.Conditions, newCondition)
		if err := r.Update(ctx, &r.cp); err != nil {
			return ctrls.RequeueWithError(err)
		}
		// Update will trigger reconciliation again
		return ctrls.DoNotRequeue()
	}

	r.log.Info("Completed Reconciliation")

	return ctrls.DoNotRequeue()
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
