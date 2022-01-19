package controllers

import (
	"context"
	b64 "encoding/base64"
	"fmt"
	"strings"

	iofogclient "github.com/eclipse-iofog/iofog-go-sdk/v3/pkg/client"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	cpv3 "github.com/eclipse-iofog/iofog-operator/v3/apis/controlplanes/v3"
)

func (r *ControlPlaneReconciler) deploymentExists(namespace, name string) (bool, error) {
	key := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}
	dep := &appsv1.Deployment{}
	err := r.Client.Get(context.TODO(), key, dep)
	if err == nil {
		return true, nil
	}
	if k8serrors.IsNotFound(err) {
		return false, nil
	}
	return false, err
}

func (r *ControlPlaneReconciler) restartPodsForDeployment(deploymentName, namespace string) error {
	// Check if this resource already exists
	found := &appsv1.Deployment{}
	if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: deploymentName, Namespace: namespace}, found); err != nil {
		return err
	}

	originValue := int32(1)
	if found.Spec.Replicas == nil {
		originValue = *found.Spec.Replicas
	}

	// Set replicas to 0
	desiredReplicas := int32(0)
	found.Spec.Replicas = &desiredReplicas
	if err := r.Client.Update(context.TODO(), found); err != nil {
		return err
	}

	// Set replicas to previous value
	found.Spec.Replicas = &originValue
	return r.Client.Update(context.TODO(), found)
}

func (r *ControlPlaneReconciler) createDeployment(ms *microservice) error {
	dep := newDeployment(r.cp.ObjectMeta.Namespace, ms)
	// Set ControlPlane instance as the owner and controller
	if err := controllerutil.SetControllerReference(&r.cp, dep, r.Scheme); err != nil {
		return err
	}

	// Check if this resource already exists
	found := &appsv1.Deployment{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: dep.Name, Namespace: dep.Namespace}, found)
	if err != nil && k8serrors.IsNotFound(err) {
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
		if err != nil && k8serrors.IsNotFound(err) {
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
	return r.createOrUpdateSecrets(ms, false)
}

func (r *ControlPlaneReconciler) createOrUpdateSecrets(ms *microservice, update bool) error {
	defer func() {
		if recoverResult := recover(); recoverResult != nil {
			r.log.Info(fmt.Sprintf("Recover result %v for creating secrets for Controlplane %s", recoverResult, r.cp.Name))
		}
	}()
	for idx := range ms.secrets {
		secret := &ms.secrets[idx]
		r.log.Info(fmt.Sprintf("Creating secret %s", secret.ObjectMeta.Name))
		// Set ControlPlane instance as the owner and controller
		r.log.Info(fmt.Sprintf("Setting owner reference for secret %s", secret.ObjectMeta.Name))
		if err := controllerutil.SetControllerReference(&r.cp, secret, r.Scheme); err != nil {
			r.log.Info(fmt.Sprintf("Failed to set owner reference for secret %s: %v", secret.ObjectMeta.Name, err))
			return err
		}

		// Check if this resource already exists
		r.log.Info(fmt.Sprintf("Checking if secret %s exists", secret.ObjectMeta.Name))
		found := &corev1.Secret{}
		err := r.Client.Get(context.TODO(), types.NamespacedName{Name: secret.Name, Namespace: secret.Namespace}, found)
		r.log.Info(fmt.Sprintf("secret %s: Exists: %s Error: %v", secret.ObjectMeta.Name, found.Name, err))
		if err != nil && k8serrors.IsNotFound(err) {
			r.log.Info("Creating a new Secret", "Secret.Namespace", secret.Namespace, "Service.Name", secret.Name)
			err = r.Client.Create(context.TODO(), secret)
			if err != nil {
				return err
			}

			// Resource created successfully - don't requeue
			continue
		} else if err != nil {
			r.log.Info(fmt.Sprintf("Failed with error %v for secret %s:", err, secret.ObjectMeta.Name))
			return err
		}

		// Resource already exists - don't requeue
		if update {
			r.log.Info("Updating secret...", "Secret.Namespace", found.Namespace, "Secret.Name", found.Name)
			err = r.Client.Update(context.TODO(), secret)
			if err != nil {
				return err
			}
		} else {
			r.log.Info("Skip reconciliation: Secret already exists.", "Secret.Namespace", found.Namespace, "Secret.Name", found.Name)
		}
	}
	r.log.Info(fmt.Sprintf("Done Creating secrets for router reconcile for Controlplane %s", r.cp.Name))
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
		if err != nil && k8serrors.IsNotFound(err) {
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
	if err != nil && k8serrors.IsNotFound(err) {
		r.log.Info("Creating a new Service Account", "ServiceAccount.Namespace", svcAcc.Namespace, "ServiceAccount.Name", svcAcc.Name)
		// TODO: Find out why the IsAlreadyExists() check is necessary here. Happens when CP redeployed
		if err = r.Client.Create(context.TODO(), svcAcc); err != nil && !k8serrors.IsAlreadyExists(err) {
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
	if err != nil && k8serrors.IsNotFound(err) {
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
	if err != nil && k8serrors.IsNotFound(err) {
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

func (r *ControlPlaneReconciler) createIofogUser(iofogClient *iofogclient.Client) error {
	user := iofogclient.User{
		Name:     r.cp.Spec.User.Name,
		Surname:  r.cp.Spec.User.Surname,
		Email:    r.cp.Spec.User.Email,
		Password: r.cp.Spec.User.Password,
	}
	password, err := DecodeBase64(user.Password)
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

func (r *ControlPlaneReconciler) updateIofogUser(iofogClient *iofogclient.Client, oldPassword, newPassword string) error {
	// Update password
	if newPassword != "" && newPassword != oldPassword {
		if err := iofogClient.UpdateUserPassword(iofogclient.UpdateUserPasswordRequest{
			OldPassword: oldPassword,
			NewPassword: newPassword,
		}); err != nil {
			return err
		}
	}

	// Try to log in
	if err := iofogClient.Login(iofogclient.LoginRequest{
		Email:    r.cp.Spec.User.Email,
		Password: newPassword,
	}); err != nil {
		return err
	}

	return nil
}

func newInt(val int) *int {
	return &val
}

func (r *ControlPlaneReconciler) createDefaultRouter(iofogClient *iofogclient.Client, proxy cpv3.RouterIngress) (err error) {
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

func DecodeBase64(encoded string) (string, error) {
	decodedBytes, err := b64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	return string(decodedBytes), nil
}
