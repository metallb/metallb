module go.universe.tf/e2etest

go 1.20

replace (
	github.com/onsi/ginkgo/v2 => github.com/onsi/ginkgo/v2 v2.13.0
	k8s.io/api => k8s.io/api v0.28.2
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.28.2
	k8s.io/apimachinery => k8s.io/apimachinery v0.28.2
	k8s.io/apiserver => k8s.io/apiserver v0.28.2
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.28.2
	k8s.io/client-go => k8s.io/client-go v0.28.2
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.28.2
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.28.2
	k8s.io/code-generator => k8s.io/code-generator v0.28.2
	k8s.io/component-base => k8s.io/component-base v0.28.2
	k8s.io/component-helpers => k8s.io/component-helpers v0.28.2
	k8s.io/controller-manager => k8s.io/controller-manager v0.28.2
	k8s.io/cri-api => k8s.io/cri-api v0.28.2
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.28.2
	k8s.io/dynamic-resource-allocation => k8s.io/dynamic-resource-allocation v0.28.2
	k8s.io/kms => k8s.io/kms v0.28.2
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.28.2
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.28.2
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.28.2
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.28.2
	k8s.io/kubectl => k8s.io/kubectl v0.28.2
	k8s.io/kubelet => k8s.io/kubelet v0.28.2
	k8s.io/kubernetes => k8s.io/kubernetes v1.28.2
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.28.2
	k8s.io/metrics => k8s.io/metrics v0.28.2
	k8s.io/mount-utils => k8s.io/mount-utils v0.28.2
	k8s.io/pod-security-admission => k8s.io/pod-security-admission v0.28.2
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.28.2
)

require (
	github.com/google/go-cmp v0.5.9
	github.com/mikioh/ipaddr v0.0.0-20190404000644-d465c8ab6721
	github.com/onsi/ginkgo/v2 v2.11.0
	github.com/onsi/gomega v1.27.10
	github.com/openshift-kni/k8sreporter v1.0.3
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_model v0.4.0
	github.com/prometheus/common v0.44.0
	go.universe.tf/metallb v0.13.9
	golang.org/x/sync v0.3.0
	k8s.io/api v0.28.2
	k8s.io/apimachinery v0.28.2
	k8s.io/client-go v1.5.2
	k8s.io/kubernetes v1.28.2
	k8s.io/pod-security-admission v0.0.0
	sigs.k8s.io/controller-runtime v0.16.2
)

require (
	github.com/NYTimes/gziphandler v1.1.1 // indirect
	github.com/antlr/antlr4/runtime/Go/antlr/v4 v4.0.0-20230305170008-8188dc5388df // indirect
	github.com/asaskevich/govalidator v0.0.0-20190424111038-f61b66f89f4a // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/blang/semver/v4 v4.0.0 // indirect
	github.com/cenkalti/backoff/v4 v4.2.1 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/coreos/go-semver v0.3.1 // indirect
	github.com/coreos/go-systemd/v22 v22.5.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/docker/distribution v2.8.2+incompatible // indirect
	github.com/emicklei/go-restful/v3 v3.10.1 // indirect
	github.com/evanphx/json-patch v5.6.0+incompatible // indirect
	github.com/evanphx/json-patch/v5 v5.6.0 // indirect
	github.com/felixge/httpsnoop v1.0.3 // indirect
	github.com/fsnotify/fsnotify v1.6.0 // indirect
	github.com/go-kit/log v0.2.1 // indirect
	github.com/go-logfmt/logfmt v0.5.1 // indirect
	github.com/go-logr/logr v1.2.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-openapi/jsonpointer v0.19.6 // indirect
	github.com/go-openapi/jsonreference v0.20.2 // indirect
	github.com/go-openapi/swag v0.22.3 // indirect
	github.com/go-task/slim-sprig v0.0.0-20230315185526-52ccab3ef572 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/glog v1.1.0 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/google/cel-go v0.16.1 // indirect
	github.com/google/gnostic-models v0.6.8 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/pprof v0.0.0-20210720184732-4bb14d4b1be1 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.7.0 // indirect
	github.com/imdario/mergo v0.3.13 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.4 // indirect
	github.com/moby/spdystream v0.2.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/prometheus/client_golang v1.16.0 // indirect
	github.com/prometheus/procfs v0.10.1 // indirect
	github.com/spf13/cobra v1.7.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/stoewer/go-strcase v1.2.0 // indirect
	go.etcd.io/etcd/api/v3 v3.5.9 // indirect
	go.etcd.io/etcd/client/pkg/v3 v3.5.9 // indirect
	go.etcd.io/etcd/client/v3 v3.5.9 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.35.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.35.1 // indirect
	go.opentelemetry.io/otel v1.10.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/internal/retry v1.10.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.10.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.10.0 // indirect
	go.opentelemetry.io/otel/metric v0.31.0 // indirect
	go.opentelemetry.io/otel/sdk v1.10.0 // indirect
	go.opentelemetry.io/otel/trace v1.10.0 // indirect
	go.opentelemetry.io/proto/otlp v0.19.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.25.0 // indirect
	golang.org/x/crypto v0.14.0 // indirect
	golang.org/x/exp v0.0.0-20220722155223-a9213eeb770e // indirect
	golang.org/x/net v0.17.0 // indirect
	golang.org/x/oauth2 v0.8.0 // indirect
	golang.org/x/sys v0.13.0 // indirect
	golang.org/x/term v0.13.0 // indirect
	golang.org/x/text v0.13.0 // indirect
	golang.org/x/time v0.3.0 // indirect
	golang.org/x/tools v0.12.0 // indirect
	gomodules.xyz/jsonpatch/v2 v2.4.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto v0.0.0-20230526161137-0005af68ea54 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20230525234035-dd9d682886f9 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20230525234030-28d5490b6b19 // indirect
	google.golang.org/grpc v1.56.3 // indirect
	google.golang.org/protobuf v1.30.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.2.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/apiextensions-apiserver v0.28.2 // indirect
	k8s.io/apiserver v0.28.2 // indirect
	k8s.io/cloud-provider v0.0.0 // indirect
	k8s.io/component-base v0.28.2 // indirect
	k8s.io/component-helpers v0.28.2 // indirect
	k8s.io/controller-manager v0.28.2 // indirect
	k8s.io/klog/v2 v2.100.1 // indirect
	k8s.io/kms v0.28.2 // indirect
	k8s.io/kube-openapi v0.0.0-20230717233707-2695361300d9 // indirect
	k8s.io/kubectl v0.0.0 // indirect
	k8s.io/kubelet v0.0.0 // indirect
	k8s.io/utils v0.0.0-20230406110748-d93618cff8a2 // indirect
	sigs.k8s.io/apiserver-network-proxy/konnectivity-client v0.1.2 // indirect
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.2.3 // indirect
	sigs.k8s.io/yaml v1.3.0 // indirect
)
