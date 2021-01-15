package controllers

import (
	"context"
	"strings"
	"time"

	cpv2 "github.com/eclipse-iofog/iofog-operator/apis/controlplanes/v2"

	iofogclient "github.com/eclipse-iofog/iofog-go-sdk/v2/pkg/client"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *ControlPlaneReconciler) createDeployment(ms *microservice) error {
	dep := newDeployment(r.cp.ObjectMeta.Namespace, ms)
	// Set ControlPlane instance as the owner and controller
	if err := controllerutil.SetControllerReference(&r.cp, dep, r.Scheme); err != nil {
		return err
	}

	// Check if this resource already exists
	found := &appsv1.Deployment{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: dep.Name, Namespace: dep.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		r.log.Info("Creating a new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
		err = r.Client.Create(context.TODO(), dep)
		if err != nil {
			return err
		}

		// Resource created successfully - don't requeue
		return nil
	} else if err != nil {
		return err
	}

	// Resource already exists - update it
	r.log.Info("Updating existing Deployment", "Deployment.Namespace", found.Namespace, "Deployment.Name", found.Name)
	if err := r.Client.Update(context.TODO(), dep); err != nil {
		return err
	}

	return nil
}

func (r *ControlPlaneReconciler) createPersistentVolumeClaims(ms *microservice) error {
	for idx := range ms.volumes {
		if ms.volumes[idx].VolumeSource.PersistentVolumeClaim == nil {
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
		pvc.ObjectMeta.Name = ms.volumes[idx].Name
		pvc.ObjectMeta.Namespace = r.cp.Namespace
		// Set ControlPlane instance as the owner and controller
		if err := controllerutil.SetControllerReference(&r.cp, &pvc, r.Scheme); err != nil {
			return err
		}

		// Check if this resource already exists
		found := &corev1.PersistentVolumeClaim{}
		err = r.Client.Get(context.TODO(), types.NamespacedName{Name: pvc.Name, Namespace: pvc.Namespace}, found)
		if err != nil && errors.IsNotFound(err) {
			r.log.Info("Creating a new PersistentVolumeClaim", "PersistentVolumeClaim.Namespace", pvc.Namespace, "PersistentVolumeClaim.Name", pvc.Name)
			err = r.Client.Create(context.TODO(), &pvc)
			if err != nil {
				return err
			}

			// Resource created successfully - don't requeue
			continue
		} else if err != nil {
			return err
		}

		// Resource already exists - don't requeue
		r.log.Info("Skip reconcile: Secret already exists", "Secret.Namespace", found.Namespace, "Secret.Name", found.Name)
	}
	return nil
}

func (r *ControlPlaneReconciler) createSecrets(ms *microservice) error {
	for idx := range ms.secrets {
		secret := &ms.secrets[idx]
		// Set ControlPlane instance as the owner and controller
		if err := controllerutil.SetControllerReference(&r.cp, secret, r.Scheme); err != nil {
			return err
		}

		// Check if this resource already exists
		found := &corev1.Secret{}
		err := r.Client.Get(context.TODO(), types.NamespacedName{Name: secret.Name, Namespace: secret.Namespace}, found)
		if err != nil && errors.IsNotFound(err) {
			r.log.Info("Creating a new Secret", "Secret.Namespace", secret.Namespace, "Service.Name", secret.Name)
			err = r.Client.Create(context.TODO(), secret)
			if err != nil {
				return err
			}

			// Resource created successfully - don't requeue
			continue
		} else if err != nil {
			return err
		}

		// Resource already exists - don't requeue
		r.log.Info("Skip reconcile: Secret already exists", "Secret.Namespace", found.Namespace, "Secret.Name", found.Name)
	}
	return nil
}

func (r *ControlPlaneReconciler) createService(ms *microservice) error {
	svcs := newServices(r.cp.ObjectMeta.Namespace, ms)
	for _, svc := range svcs {
		// Set ControlPlane instance as the owner and controller
		if err := controllerutil.SetControllerReference(&r.cp, svc, r.Scheme); err != nil {
			return err
		}

		// Check if this resource already exists
		found := &corev1.Service{}
		err := r.Client.Get(context.TODO(), types.NamespacedName{Name: svc.Name, Namespace: svc.Namespace}, found)
		if err != nil && errors.IsNotFound(err) {
			r.log.Info("Creating a new Service", "Service.Namespace", svc.Namespace, "Service.Name", svc.Name)
			err = r.Client.Create(context.TODO(), svc)
			if err != nil {
				return err
			}

			// Resource created successfully - don't requeue
			continue
		} else if err != nil {
			return err
		}

		// Resource already exists - don't requeue
		r.log.Info("Skip reconcile: Service already exists", "Service.Namespace", found.Namespace, "Service.Name", found.Name)
	}
	return nil
}

func (r *ControlPlaneReconciler) createServiceAccount(ms *microservice) error {
	svcAcc := newServiceAccount(r.cp.ObjectMeta.Namespace, ms)

	// Set image pull secret for the service account
	if ms.imagePullSecret != "" {
		secret := &corev1.Secret{}
		err := r.Client.Get(context.TODO(), types.NamespacedName{
			Namespace: svcAcc.Namespace,
			Name:      ms.imagePullSecret,
		}, secret)
		if err != nil || secret.Type != corev1.SecretTypeDockerConfigJson {
			r.log.Error(err, "Failed to create a new Service Account with imagePullSecret",
				"ServiceAccount.Namespace", svcAcc.Namespace,
				"ServiceAccount.Name", svcAcc.Name,
				"pullSecret", ms.imagePullSecret)
			return err
		}
		svcAcc.ImagePullSecrets = []corev1.LocalObjectReference{
			{Name: ms.imagePullSecret},
		}
	}

	// Set ControlPlane instance as the owner and controller
	if err := controllerutil.SetControllerReference(&r.cp, svcAcc, r.Scheme); err != nil {
		return err
	}

	// Check if this resource already exists
	found := &corev1.ServiceAccount{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: svcAcc.Name, Namespace: svcAcc.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		r.log.Info("Creating a new Service Account", "ServiceAccount.Namespace", svcAcc.Namespace, "ServiceAccount.Name", svcAcc.Name)
		// TODO: Find out why the IsAlreadyExists() check is necessary here. Happens when CP redeployed
		if err = r.Client.Create(context.TODO(), svcAcc); err != nil && !errors.IsAlreadyExists(err) {
			return err
		}

		// Resource created successfully - don't requeue
		return nil
	} else if err != nil {
		return err
	}

	// Resource already exists - don't requeue
	r.log.Info("Skip reconcile: Service Account already exists", "ServiceAccount.Namespace", found.Namespace, "ServiceAccount.Name", found.Name)
	return nil
}

func (r *ControlPlaneReconciler) createRole(ms *microservice) error {
	role := newRole(r.cp.ObjectMeta.Namespace, ms)

	// Set ControlPlane instance as the owner and controller
	if err := controllerutil.SetControllerReference(&r.cp, role, r.Scheme); err != nil {
		return err
	}

	// Check if this resource already exists
	found := &rbacv1.Role{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: role.Name, Namespace: role.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		r.log.Info("Creating a new Role ", "Role.Namespace", role.Namespace, "Role.Name", role.Name)
		err = r.Client.Create(context.TODO(), role)
		if err != nil {
			return err
		}

		// Resource created successfully - don't requeue
		return nil
	} else if err != nil {
		return err
	}

	// Resource already exists - don't requeue
	r.log.Info("Skip reconcile: Role already exists", "Role.Namespace", found.Namespace, "Role.Name", found.Name)

	return nil
}

func (r *ControlPlaneReconciler) createRoleBinding(ms *microservice) error {
	crb := newRoleBinding(r.cp.ObjectMeta.Namespace, ms)

	// Set ControlPlane instance as the owner and controller
	if err := controllerutil.SetControllerReference(&r.cp, crb, r.Scheme); err != nil {
		return err
	}

	// Check if this resource already exists
	found := &rbacv1.RoleBinding{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: crb.Name, Namespace: crb.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		r.log.Info("Creating a new Role Binding", "RoleBinding.Namespace", crb.Namespace, "RoleBinding.Name", crb.Name)
		err = r.Client.Create(context.TODO(), crb)
		if err != nil {
			return err
		}

		// Resource created successfully - don't requeue
		return nil
	} else if err != nil {
		return err
	}

	// Resource already exists - don't requeue
	r.log.Info("Skip reconcile: Role Binding already exists", "RoleBinding.Namespace", found.Namespace, "RoleBinding.Name", found.Name)
	return nil
}

func (r *ControlPlaneReconciler) waitForControllerAPI(iofogClient *iofogclient.Client) (err error) {
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
		if _, err = iofogClient.GetStatus(); err != nil {
			// Retry if connection is refused, this is usually only necessary on K8s Controller
			if strings.Contains(err.Error(), "connection refused") {
				time.Sleep(time.Millisecond * 1000)
				iter++
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

func (r *ControlPlaneReconciler) createIofogUser(iofogClient *iofogclient.Client) error {
	user := iofogclient.User(r.cp.Spec.User)
	password, err := decodeBase64(user.Password)
	if err == nil {
		user.Password = password
	}

	if err := iofogClient.CreateUser(user); err != nil {
		// If not error about account existing, fail
		if !strings.Contains(err.Error(), "already an account associated") {
			return err
		}
	}

	// Try to log in
	if err := iofogClient.Login(iofogclient.LoginRequest{
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

func (r *ControlPlaneReconciler) createDefaultRouter(iofogClient *iofogclient.Client, proxy cpv2.RouterIngress) (err error) {
	routerConfig := iofogclient.Router{
		Host: proxy.Address,
		RouterConfig: iofogclient.RouterConfig{
			InterRouterPort: newInt(proxy.InteriorPort),
			EdgeRouterPort:  newInt(proxy.EdgePort),
			MessagingPort:   newInt(proxy.MessagePort),
		},
	}
	if err = iofogClient.PutDefaultRouter(routerConfig); err != nil {
		return
	}
	return
}
