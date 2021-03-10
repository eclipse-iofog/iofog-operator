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
	"fmt"
	"time"

	op "github.com/eclipse-iofog/iofog-go-sdk/v3/pkg/k8s/operator"
	cond "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	conditionReady     = "ready"
	conditionDeploying = "" // Update iofogctl etc to set default state
)

type reconcileFunc = func() op.Reconciliation

func (r *ControlPlaneReconciler) getReconcileFunc() (reconcileFunc, error) {
	state := ""
	for _, condition := range r.cp.Status.Conditions {
		if condition.Status == metav1.ConditionTrue {
			state = condition.Type
			break
		}
	}
	switch state {
	case conditionReady:
		return r.reconcileReady, nil
	case conditionDeploying:
		return r.reconcileDeploying, nil
	default:
		return nil, fmt.Errorf("invalid state %s for ECN %s", state, r.cp.Name)
	}
}

func (r *ControlPlaneReconciler) reconcileReady() op.Reconciliation {
	// Do nothing
	return op.Reconcile()
}

func (r *ControlPlaneReconciler) reconcileDeploying() op.Reconciliation {
	ctx := context.Background()
	// Error chan for reconcile routines
	reconcilerCount := 3
	reconChan := make(chan op.Reconciliation, reconcilerCount)

	// Reconcile Router
	go reconcileRoutine(r.reconcileRouter, reconChan)

	// Reconcile Iofog Controller and Kubelet
	go reconcileRoutine(r.reconcileIofogController, reconChan)

	// Reconcile Port Manager
	go reconcileRoutine(r.reconcilePortManager, reconChan)

	// Wait for all parallel recons and evaluate results
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
		return finRecon
	}

	// deploying -> ready
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
			return op.ReconcileWithError(err)
		}
		r.log.Info(fmt.Sprintf("Control Plane %s is ready", r.cp.Name))
		return op.Reconcile()
	}
	return op.Continue()
}
