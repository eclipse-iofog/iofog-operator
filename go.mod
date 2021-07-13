module github.com/eclipse-iofog/iofog-operator/v3

go 1.16

require (
	github.com/eclipse-iofog/iofog-go-sdk/v3 v3.0.0-20210713074357-099a53d65d91
	github.com/go-logr/logr v0.3.0
	github.com/skupperproject/skupper-cli v0.0.1-beta6
	k8s.io/api v0.19.4
	k8s.io/apiextensions-apiserver v0.19.4
	k8s.io/apimachinery v0.19.4
	k8s.io/client-go v11.0.0+incompatible
	sigs.k8s.io/controller-runtime v0.6.4
)

replace (
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v13.3.2+incompatible // Required by OLM
	github.com/go-logr/logr => github.com/go-logr/logr v0.3.0
	github.com/go-logr/zapr => github.com/go-logr/zapr v0.3.0
	github.com/mattn/go-sqlite3 => github.com/mattn/go-sqlite3 v1.10.0
	golang.org/x/text => golang.org/x/text v0.3.3 // Required to fix CVE-2020-14040
	k8s.io/client-go => k8s.io/client-go v0.19.4
)

exclude github.com/spf13/viper v1.3.2 // Required to fix CVE-2018-1098
