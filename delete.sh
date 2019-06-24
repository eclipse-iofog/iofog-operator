kubectl delete -f deploy/crds/k8s_v1alpha1_iofog_cr.yaml
kubectl delete -f deploy/crds/k8s_v1alpha1_iofog_crd.yaml
kubectl delete -f deploy/namespace.yaml
kubectl delete -f deploy/role_binding.yaml
kubectl delete -f deploy/role.yaml
kubectl delete -f deploy/service_account.yaml

