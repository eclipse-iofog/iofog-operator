package kog

import (
	"context"
	"strings"
	"time"

	iofogclient "github.com/eclipse-iofog/iofog-go-sdk/pkg/client"
	iofogv1 "github.com/eclipse-iofog/iofog-operator/pkg/apis/iofog/v1"
	"github.com/eclipse-iofog/iofog-operator/pkg/controller/kog/skupper"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *ReconcileKog) createDeployment(kog *iofogv1.Kog, ms *microservice) error {
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

	// Resource already exists - update it
	r.logger.Info("Updating existing Deployment", "Deployment.Namespace", found.Namespace, "Deployment.Name", found.Name)
	if err = r.client.Update(context.TODO(), dep); err != nil {
		return err
	}

	return nil
}

func (r *ReconcileKog) createPersistentVolumeClaims(kog *iofogv1.Kog, ms *microservice) error {
	for _, vol := range ms.volumes {
		if vol.VolumeSource.PersistentVolumeClaim == nil {
			continue
		}
		storageSize, err := resource.ParseQuantity("1Gi")
		if err != nil {
			return err
		}
		pvc := corev1.PersistentVolumeClaim{
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{
					corev1.ReadWriteOnce,
				},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						"storage": storageSize,
					},
				},
			},
		}
		pvc.ObjectMeta.Name = vol.Name
		pvc.ObjectMeta.Namespace = kog.Namespace
		// Set Kog instance as the owner and controller
		if err := controllerutil.SetControllerReference(kog, &pvc, r.scheme); err != nil {
			return err
		}

		// Check if this resource already exists
		found := &corev1.PersistentVolumeClaim{}
		err = r.client.Get(context.TODO(), types.NamespacedName{Name: pvc.Name, Namespace: pvc.Namespace}, found)
		if err != nil && errors.IsNotFound(err) {
			r.logger.Info("Creating a new PersistentVolumeClaim", "PersistentVolumeClaim.Namespace", pvc.Namespace, "PersistentVolumeClaim.Name", pvc.Name)
			err = r.client.Create(context.TODO(), &pvc)
			if err != nil {
				return err
			}

			// Resource created successfully - don't requeue
			continue
		} else if err != nil {
			return err
		}

		// Resource already exists - don't requeue
		r.logger.Info("Skip reconcile: Secret already exists", "Secret.Namespace", found.Namespace, "Secret.Name", found.Name)
	}
	return nil
}

func (r *ReconcileKog) createSecrets(kog *iofogv1.Kog, ms *microservice) error {
	for _, secret := range ms.secrets {
		// Set Kog instance as the owner and controller
		if err := controllerutil.SetControllerReference(kog, &secret, r.scheme); err != nil {
			return err
		}

		// Check if this resource already exists
		found := &corev1.Secret{}
		err := r.client.Get(context.TODO(), types.NamespacedName{Name: secret.Name, Namespace: secret.Namespace}, found)
		if err != nil && errors.IsNotFound(err) {
			r.logger.Info("Creating a new Secret", "Secret.Namespace", secret.Namespace, "Service.Name", secret.Name)
			err = r.client.Create(context.TODO(), &secret)
			if err != nil {
				return err
			}

			// Resource created successfully - don't requeue
			continue
		} else if err != nil {
			return err
		}

		// Resource already exists - don't requeue
		r.logger.Info("Skip reconcile: Secret already exists", "Secret.Namespace", found.Namespace, "Secret.Name", found.Name)
	}
	return nil
}

func (r *ReconcileKog) createService(kog *iofogv1.Kog, ms *microservice) error {
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

func (r *ReconcileKog) createServiceAccount(kog *iofogv1.Kog, ms *microservice) error {
	svcAcc := newServiceAccount(kog.ObjectMeta.Namespace, ms)

	// Set image pull secret for the service account
	if ms.imagePullSecret != "" {
		secret := &corev1.Secret{}
		err := r.client.Get(context.TODO(), types.NamespacedName{
			Namespace: svcAcc.Namespace,
			Name:      ms.imagePullSecret,
		}, secret)
		if err != nil || secret.Type != corev1.SecretTypeDockerConfigJson {
			r.logger.Error(err, "Failed to create a new Service Account with imagePullSecret",
				"ServiceAccount.Namespace", svcAcc.Namespace,
				"ServiceAccount.Name", svcAcc.Name,
				"pullSecret", ms.imagePullSecret)
			return err
		}
		svcAcc.ImagePullSecrets = []corev1.LocalObjectReference{
			{Name: ms.imagePullSecret},
		}
	}

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

func (r *ReconcileKog) createRole(kog *iofogv1.Kog, ms *microservice) error {
	role := newRole(kog.ObjectMeta.Namespace, ms)

	// Set Kog instance as the owner and controller
	if err := controllerutil.SetControllerReference(kog, role, r.scheme); err != nil {
		return err
	}

	// Check if this resource already exists
	found := &rbacv1.Role{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: role.Name, Namespace: role.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		r.logger.Info("Creating a new Role ", "Role.Namespace", role.Namespace, "Role.Name", role.Name)
		err = r.client.Create(context.TODO(), role)
		if err != nil {
			return err
		}

		// Resource created successfully - don't requeue
		return nil
	} else if err != nil {
		return err
	}

	// Resource already exists - don't requeue
	r.logger.Info("Skip reconcile: Role already exists", "Role.Namespace", found.Namespace, "Role.Name", found.Name)

	return nil
}

func (r *ReconcileKog) createRoleBinding(kog *iofogv1.Kog, ms *microservice) error {
	crb := newRoleBinding(kog.ObjectMeta.Namespace, ms)

	// Set Kog instance as the owner and controller
	if err := controllerutil.SetControllerReference(kog, crb, r.scheme); err != nil {
		return err
	}

	// Check if this resource already exists
	found := &rbacv1.RoleBinding{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: crb.Name, Namespace: crb.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		r.logger.Info("Creating a new Role Binding", "RoleBinding.Namespace", crb.Namespace, "RoleBinding.Name", crb.Name)
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
	r.logger.Info("Skip reconcile: Role Binding already exists", "RoleBinding.Namespace", found.Namespace, "RoleBinding.Name", found.Name)
	return nil
}

func (r *ReconcileKog) createClusterRoleBinding(kog *iofogv1.Kog, ms *microservice) error {
	crb := newClusterRoleBinding(kog.ObjectMeta.Namespace, ms)

	// Set Kog instance as the owner and controller
	if err := controllerutil.SetControllerReference(kog, crb, r.scheme); err != nil {
		return err
	}

	// Check if this resource already exists
	found := &rbacv1.ClusterRoleBinding{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: crb.Name, Namespace: crb.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		r.logger.Info("Creating a new Cluster Role Binding", "ClusterRoleBinding.Namespace", crb.Namespace, "ClusterRoleBinding.Name", crb.Name)
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
	connected := false
	iter := 0
	const timeoutSeconds = 120
	for !connected {
		// Time out
		if iter > timeoutSeconds {
			err = errors.NewTimeoutError("Timed out waiting for Controller API", iter)
			return
		}
		// Check the status endpoint
		if _, err = r.iofogClient.GetStatus(); err != nil {
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

func (r *ReconcileKog) createIofogUser(user *iofogv1.IofogUser) (err error) {
	if err = r.iofogClient.CreateUser(iofogclient.User(*user)); err != nil {
		// If not error about account existing, fail
		if !strings.Contains(err.Error(), "already an account associated") {
			return err
		}
	}

	// Try to log in
	if err = r.iofogClient.Login(iofogclient.LoginRequest{
		Email:    user.Email,
		Password: user.Password,
	}); err != nil {
		return err
	}

	return nil
}

func newInt(val int) *int {
	return &val
}

func (r *ReconcileKog) createDefaultRouter(user *iofogv1.IofogUser, routerIP string) (err error) {
	routerConfig := iofogclient.Router{
		Host: routerIP,
		RouterConfig: iofogclient.RouterConfig{
			InterRouterPort: newInt(skupper.InteriorPort),
			EdgeRouterPort:  newInt(skupper.EdgePort),
			MessagingPort:   newInt(skupper.MessagePort),
		},
	}
	if err = r.iofogClient.PutDefaultRouter(routerConfig); err != nil {
		return err
	}
	return nil
}
