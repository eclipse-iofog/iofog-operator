module github.com/eclipse-iofog/iofog-operator

go 1.13

require (
	cloud.google.com/go v0.51.0 // indirect
	github.com/Azure/go-autorest/autorest v0.9.6 // indirect
	github.com/eapache/channels v1.1.0 // indirect
	github.com/eapache/queue v1.1.0 // indirect
	github.com/eclipse-iofog/iofog-go-sdk/v2 v2.0.0
	github.com/go-logr/logr v0.3.0
	github.com/gorilla/websocket v1.4.1 // indirect
	github.com/gregjones/httpcache v0.0.0-20190611155906-901d90724c79 // indirect
	github.com/onsi/ginkgo v1.14.1
	github.com/onsi/gomega v1.10.2
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d // indirect
	k8s.io/api v0.19.4
	k8s.io/apimachinery v0.19.4
	k8s.io/client-go v11.0.0+incompatible
	sigs.k8s.io/controller-runtime v0.6.4
	sigs.k8s.io/structured-merge-diff v0.0.0-20190525122527-15d366b2352e // indirect
)

replace (
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v13.3.2+incompatible // Required by OLM
	github.com/mattn/go-sqlite3 => github.com/mattn/go-sqlite3 v1.10.0
	golang.org/x/text => golang.org/x/text v0.3.3 // Required to fix CVE-2020-14040
)

replace (
	github.com/go-logr/logr => github.com/go-logr/logr v0.3.0
	github.com/go-logr/zapr => github.com/go-logr/zapr v0.3.0
	k8s.io/client-go => k8s.io/client-go v0.19.4
)

exclude github.com/spf13/viper v1.3.2 // Required to fix CVE-2018-1098
