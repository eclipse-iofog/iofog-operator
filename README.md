# iofog-operator
Created using operator-sdk: https://github.com/operator-framework/operator-sdk

Read documentation before making changes.

### Run operator locally:
kubectl create -f deploy/service_account.yaml  
kubectl create -f deploy/role.yaml  
kubectl create -f deploy/role_binding.yaml  
kubectl create -f deploy/crds/k8s_v1alpha1_iofog_crd.yaml  
`$GOPATH`/bin/operator-sdk up local --namespace=default  

### Build Docker image
`$GOPATH`/bin/operator-sdk build iofog/iofog-operator:`$VERSION`  
`$VERSION should follow Semantic versioning pattern (x.y.z format)`
