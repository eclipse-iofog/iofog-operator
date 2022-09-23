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

	op "github.com/eclipse-iofog/iofog-go-sdk/v3/pkg/k8s/operator"
)

type reconcileFunc = func() op.Reconciliation

func (r *ControlPlaneReconciler) getReconcileFunc() (reconcileFunc, error) {
	if r.cp.IsReady() {
		return r.reconcileReady, nil
	}

	if r.cp.IsDeploying() {
		return r.reconcileDeploying, nil
	}
	// If invalid state, migrate state to deploying to restart on sane basis
	r.cp.SetConditionDeploying()

	if err := r.Status().Update(context.Background(), &r.cp); err != nil {
		return nil, err
	}

	return r.reconcileDeploying, nil
}

func (r *ControlPlaneReconciler) reconcileReady() op.Reconciliation {
	// Do nothing
	r.log.Info(fmt.Sprintf("reconcileReady() ControlPlane %s", r.cp.Name))

	return op.Reconcile()
}

func (r *ControlPlaneReconciler) reconcileDeploying() op.Reconciliation {
	r.log.Info(fmt.Sprintf("reconcileDeploying() ControlPlane %s", r.cp.Name))
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

	for i := 0; i < reconcilerCount; i++ {
		recon := <-reconChan
		if recon.Err != nil {
			if finRecon.Err == nil {
				// Create new err
				finRecon.Err = recon.Err
			} else {
				// Append
				finRecon.Err = fmt.Errorf("%s\n%s", finRecon.Err.Error(), recon.Err.Error()) //nolint:errorlint
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
		r.log.Info(fmt.Sprintf("reconcileDeploying() ControlPlane %s isFinal", r.cp.Name))

		return finRecon
	}

	// deploying -> ready
	if r.cp.IsDeploying() {
		r.log.Info(fmt.Sprintf("reconcileDeploying() ControlPlane %s setReady", r.cp.Name))
		r.cp.SetConditionReady(&r.log) // temporary logger
		r.log.Info(fmt.Sprintf("reconcileDeploying() ControlPlane %s -- write status update, new conditions %v", r.cp.Name, r.cp.Status.Conditions))

		if err := r.Status().Update(ctx, &r.cp); err != nil {
			r.log.Error(err, fmt.Sprintf("reconcileDeploying() ControlPlane %s -- failed to update status", r.cp.Name))

			return op.ReconcileWithError(err)
		}

		if err := r.Update(ctx, &r.cp); err != nil {
			return op.ReconcileWithError(err)
		}

		r.log.Info(fmt.Sprintf("Control Plane %s is ready", r.cp.Name))

		return op.Reconcile()
	}
	return op.Continue()
}
