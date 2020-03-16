module github.com/eclipse-iofog/iofog-operator/v2

go 1.12

require (
	github.com/eclipse-iofog/iofog-go-sdk v1.3.0 // indirect
	github.com/eclipse-iofog/iofog-go-sdk/v2 v2.0.0-beta
	github.com/eclipse-iofog/iofog-operator v1.3.0
	github.com/go-logr/logr v0.1.0
	github.com/operator-framework/operator-sdk v0.10.0
	github.com/skupperproject/skupper-cli v0.0.1-beta6.0.20191022215135-8088454e7fda
	golang.org/x/tools v0.0.0-20191212203136-8facea2ecf42 // indirect
	k8s.io/api v0.17.3
	k8s.io/apimachinery v0.17.3
	k8s.io/client-go v11.0.0+incompatible
	k8s.io/gengo v0.0.0-20191120174120-e74f70b9b27e // indirect
	sigs.k8s.io/controller-runtime v0.1.10
)

// Pinned to kubernetes-1.13.4
replace (
	github.com/openshift/api => github.com/openshift/api v0.0.0-20180801171038-322a19404e37
	k8s.io/api => k8s.io/api v0.0.0-20190222213804-5cb15d344471
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.0.0-20190228180357-d002e88f6236
	k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20190221213512-86fb29eff628
	k8s.io/apiserver => k8s.io/apiserver v0.0.0-20190228174905-79427f02047f
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.0.0-20190228180923-a9e421a79326
	k8s.io/client-go => k8s.io/client-go v0.0.0-20190228174230-b40b2a5939e4
	k8s.io/code-generator => k8s.io/code-generator v0.0.0-20181117043124-c2090bec4d9b
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.0.0-20190228175259-3e0149950b0e
	k8s.io/kube-openapi => k8s.io/kube-openapi v0.0.0-20180711000925-0cf8f7e6ed1d
	k8s.io/kubernetes => k8s.io/kubernetes v1.13.4
)
