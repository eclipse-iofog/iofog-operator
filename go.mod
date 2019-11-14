module github.com/eclipse-iofog/iofog-operator

go 1.12

require (
	cloud.google.com/go v0.37.2
	dmitri.shuralyov.com/app/changes v0.0.0-20180602232624-0a106ad413e3 // indirect
	dmitri.shuralyov.com/html/belt v0.0.0-20180602232347-f7d459c86be0 // indirect
	dmitri.shuralyov.com/service/change v0.0.0-20181023043359-a85b471d5412 // indirect
	dmitri.shuralyov.com/state v0.0.0-20180228185332-28bcc343414c // indirect
	git.apache.org/thrift.git v0.12.0 // indirect
	github.com/PuerkitoBio/purell v1.1.1
	github.com/PuerkitoBio/urlesc v0.0.0-20170810143723-de5bf2ad4578
	github.com/beorn7/perks v0.0.0-20180321164747-3a771d992973
	github.com/davecgh/go-spew v1.1.1
	github.com/dustin/go-humanize v1.0.0 // indirect
	github.com/eclipse-iofog/iofog-go-sdk v0.0.0-20191110234250-92c6c6d34082
	github.com/emicklei/go-restful v2.9.3+incompatible
	github.com/ghodss/yaml v1.0.0
	github.com/go-logr/logr v0.1.0
	github.com/go-logr/zapr v0.1.1
	github.com/go-openapi/jsonpointer v0.19.0
	github.com/go-openapi/jsonreference v0.19.0
	github.com/go-openapi/spec v0.19.0
	github.com/go-openapi/swag v0.19.0
	github.com/gogo/protobuf v1.2.1
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/golang/groupcache v0.0.0-20190129154638-5b532d6fd5ef
	github.com/golang/protobuf v1.3.1
	github.com/google/btree v1.0.0
	github.com/google/gofuzz v1.0.0
	github.com/google/uuid v1.1.1
	github.com/googleapis/gnostic v0.2.0
	github.com/gopherjs/gopherjs v0.0.0-20181017120253-0766667cb4d1 // indirect
	github.com/gregjones/httpcache v0.0.0-20190212212710-3befbb6ad0cc
	github.com/hashicorp/golang-lru v0.5.1
	github.com/imdario/mergo v0.3.7
	github.com/json-iterator/go v1.1.6
	github.com/mailru/easyjson v0.0.0-20190403194419-1ea4449da983
	github.com/mattbaird/jsonpatch v0.0.0-20171005235357-81af80346b1a
	github.com/matttproud/golang_protobuf_extensions v1.0.1
	github.com/microcosm-cc/bluemonday v1.0.1 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd
	github.com/modern-go/reflect2 v1.0.1
	github.com/neelance/astrewrite v0.0.0-20160511093645-99348263ae86 // indirect
	github.com/neelance/sourcemap v0.0.0-20151028013722-8c68805598ab // indirect
	github.com/operator-framework/operator-sdk v0.10.0
	github.com/pborman/uuid v1.2.0
	github.com/petar/GoLLRB v0.0.0-20130427215148-53be0d36a84c
	github.com/peterbourgon/diskv v2.0.1+incompatible
	github.com/prometheus/client_golang v0.9.3-0.20190127221311-3c4408c8b829
	github.com/prometheus/client_model v0.0.0-20190129233127-fd36f4220a90
	github.com/prometheus/common v0.2.0
	github.com/prometheus/procfs v0.0.0-20190403104016-ea9eea638872
	github.com/shurcooL/component v0.0.0-20170202220835-f88ec8f54cc4 // indirect
	github.com/shurcooL/events v0.0.0-20181021180414-410e4ca65f48 // indirect
	github.com/shurcooL/github_flavored_markdown v0.0.0-20181002035957-2122de532470 // indirect
	github.com/shurcooL/gofontwoff v0.0.0-20180329035133-29b52fc0a18d // indirect
	github.com/shurcooL/gopherjslib v0.0.0-20160914041154-feb6d3990c2c // indirect
	github.com/shurcooL/highlight_diff v0.0.0-20170515013008-09bb4053de1b // indirect
	github.com/shurcooL/highlight_go v0.0.0-20181028180052-98c3abbbae20 // indirect
	github.com/shurcooL/home v0.0.0-20181020052607-80b7ffcb30f9 // indirect
	github.com/shurcooL/htmlg v0.0.0-20170918183704-d01228ac9e50 // indirect
	github.com/shurcooL/httperror v0.0.0-20170206035902-86b7830d14cc // indirect
	github.com/shurcooL/httpfs v0.0.0-20171119174359-809beceb2371 // indirect
	github.com/shurcooL/httpgzip v0.0.0-20180522190206-b1c53ac65af9 // indirect
	github.com/shurcooL/issues v0.0.0-20181008053335-6292fdc1e191 // indirect
	github.com/shurcooL/issuesapp v0.0.0-20180602232740-048589ce2241 // indirect
	github.com/shurcooL/notifications v0.0.0-20181007000457-627ab5aea122 // indirect
	github.com/shurcooL/octicon v0.0.0-20181028054416-fa4f57f9efb2 // indirect
	github.com/shurcooL/reactions v0.0.0-20181006231557-f2e0b4ca5b82 // indirect
	github.com/shurcooL/users v0.0.0-20180125191416-49c67e49c537 // indirect
	github.com/shurcooL/webdavfs v0.0.0-20170829043945-18c3829fa133 // indirect
	github.com/sourcegraph/annotate v0.0.0-20160123013949-f4cad6c6324d // indirect
	github.com/sourcegraph/syntaxhighlight v0.0.0-20170531221838-bd320f5d308e // indirect
	github.com/spf13/pflag v1.0.3
	go.uber.org/atomic v1.3.2
	go.uber.org/multierr v1.1.0
	go.uber.org/zap v1.9.1
	golang.org/x/crypto v0.0.0-20190404164418-38d8ce5564a5
	golang.org/x/net v0.0.0-20190404232315-eb5bcb51f2a3
	golang.org/x/oauth2 v0.0.0-20190402181905-9f3314589c9a
	golang.org/x/sys v0.0.0-20190405154228-4b34438f7a67
	golang.org/x/text v0.3.1-0.20181227161524-e6919f6577db
	golang.org/x/time v0.0.0-20190308202827-9d24e82272b4
	golang.org/x/tools v0.0.0-20190408170212-12dd9f86f350
	google.golang.org/appengine v1.5.0
	gopkg.in/inf.v0 v0.9.1
	gopkg.in/yaml.v2 v2.2.2
	k8s.io/api v0.0.0-20190612125737-db0771252981
	k8s.io/apimachinery v0.0.0-20190612125636-6a5db36e93ad
	k8s.io/client-go v11.0.0+incompatible
	k8s.io/code-generator v0.0.0-20181203235156-f8cba74510f3
	k8s.io/gengo v0.0.0-20190327210449-e17681d19d3a
	k8s.io/klog v0.3.1
	k8s.io/kube-openapi v0.0.0-20190320154901-5e45bb682580
	sigs.k8s.io/controller-runtime v0.1.10
	sourcegraph.com/sourcegraph/go-diff v0.5.0 // indirect
)

// Pinned to kubernetes-1.13.4
replace (
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
