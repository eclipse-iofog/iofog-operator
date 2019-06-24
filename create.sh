kubectl create -f deploy/service_account.yaml
kubectl create -f deploy/role.yaml
kubectl create -f deploy/role_binding.yaml
kubectl create -f deploy/namespace.yaml
kubectl create -f deploy/crds/k8s_v1alpha1_iofog_crd.yaml

