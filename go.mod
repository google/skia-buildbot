module go.skia.org/infra

require (
	cloud.google.com/go v0.111.0
	cloud.google.com/go/bigquery v1.57.1
	cloud.google.com/go/bigtable v1.19.0
	cloud.google.com/go/compute/metadata v0.2.3
	cloud.google.com/go/datastore v1.15.0
	cloud.google.com/go/firestore v1.14.0
	cloud.google.com/go/iam v1.1.5
	cloud.google.com/go/logging v1.9.0
	cloud.google.com/go/monitoring v1.17.0
	cloud.google.com/go/pubsub v1.33.0
	cloud.google.com/go/redis v1.14.2
	cloud.google.com/go/secretmanager v1.11.4
	cloud.google.com/go/storage v1.31.0
	contrib.go.opencensus.io/exporter/stackdriver v0.13.4
	github.com/Jeffail/gabs/v2 v2.6.0
	github.com/Masterminds/semver v1.5.0
	github.com/Masterminds/sprig v2.22.0+incompatible
	github.com/a8m/envsubst v1.2.0
	github.com/aclements/go-moremath v0.0.0-20190830160640-d16893ddf098
	github.com/bazelbuild/bazel-gazelle v0.33.0
	github.com/bazelbuild/buildtools v0.0.0-20231017121127-23aa65d4e117
	github.com/bazelbuild/remote-apis v0.0.0-20230822133051-6c32c3b917cc
	github.com/bazelbuild/remote-apis-sdks v0.0.0-20231114220034-042d9851eb28
	github.com/blakesmith/ar v0.0.0-20190502131153-809d4375e1fb
	github.com/bradfitz/gomemcache v0.0.0-20190913173617-a41fca850d0b
	github.com/cenkalti/backoff v2.2.1+incompatible
	github.com/cenkalti/backoff/v4 v4.1.3
	github.com/cockroachdb/cockroach-go/v2 v2.1.0
	github.com/davecgh/go-spew v1.1.1
	github.com/dustin/go-humanize v1.0.1
	github.com/fiorix/go-web v1.0.1-0.20150221144011-5b593f1e8966
	github.com/flynn/json5 v0.0.0-20160717195620-7620272ed633
	github.com/go-chi/chi/v5 v5.0.8
	github.com/go-python/gpython v0.0.3
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da
	github.com/golang/mock v1.6.0
	github.com/golang/protobuf v1.5.3
	github.com/google/go-cmp v0.6.0
	github.com/google/go-github/v29 v29.0.3
	github.com/google/go-licenses v0.0.0-20210816172045-3099c18c36e1
	github.com/google/uuid v1.5.0
	github.com/googleapis/gax-go/v2 v2.12.0
	github.com/gorilla/securecookie v1.1.1
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.18.1
	github.com/hako/durafmt v0.0.0-20200710122514-c0fb7b4da026
	github.com/hashicorp/go-multierror v1.1.0
	github.com/hashicorp/golang-lru v0.5.4
	github.com/invopop/jsonschema v0.7.0
	github.com/jackc/pgconn v1.14.0
	github.com/jackc/pgtype v1.14.0
	github.com/jackc/pgx/v4 v4.18.1
	github.com/jcgregorio/logger v0.1.3
	github.com/kballard/go-shellquote v0.0.0-20180428030007-95032a82bc51
	github.com/kisielk/errcheck v1.5.0
	github.com/miekg/dns v1.1.41
	github.com/nfnt/resize v0.0.0-20180221191011-83c6a9932646
	github.com/olekukonko/tablewriter v0.0.4
	github.com/otiai10/copy v1.6.0
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/pkg/errors v0.9.1
	github.com/pmezard/go-difflib v1.0.0
	github.com/prometheus/client_golang v1.11.1
	github.com/protocolbuffers/txtpbfmt v0.0.0-20230730201308-0c31dbd32b9f
	github.com/r3labs/sse/v2 v2.8.1
	github.com/redis/go-redis/v9 v9.5.3
	github.com/rs/cors v1.6.0
	github.com/sendgrid/sendgrid-go v3.11.1+incompatible
	github.com/shirou/gopsutil v3.21.11+incompatible
	github.com/skia-dev/google-api-go-client v0.10.1-0.20200109184256-16c3d6f408b2
	github.com/skia-dev/protoc-gen-twirp_typescript v0.0.0-20220429132620-ad26708b7787
	github.com/spf13/cobra v1.6.1
	github.com/stretchr/testify v1.8.4
	github.com/syndtr/goleveldb v1.0.0
	github.com/tarm/serial v0.0.0-20180830185346-98f6abe2eb07
	github.com/texttheater/golang-levenshtein v1.0.1
	github.com/twitchtv/twirp v7.1.0+incompatible
	github.com/unrolled/secure v1.0.8
	github.com/urfave/cli/v2 v2.17.0
	github.com/vektra/mockery/v2 v2.38.0
	github.com/willf/bitset v1.1.11
	github.com/xeipuuv/gojsonschema v1.2.0
	github.com/yannh/kubeconform v0.6.3
	github.com/yusufpapurcu/wmi v1.2.2
	github.com/zeebo/bencode v1.0.0
	go.chromium.org/luci v0.0.0-20240206071351-fb32c458db6e
	go.opencensus.io v0.24.0
	go.temporal.io/api v1.26.2
	go.temporal.io/sdk v1.25.2-0.20240108215803-6244097c5aca
	golang.org/x/exp v0.0.0-20231127185646-65229373498e
	golang.org/x/net v0.20.0
	golang.org/x/oauth2 v0.16.0
	golang.org/x/sync v0.6.0
	golang.org/x/sys v0.16.0
	golang.org/x/term v0.16.0
	golang.org/x/time v0.5.0
	golang.org/x/tools v0.16.0
	google.golang.org/api v0.160.0
	google.golang.org/genproto v0.0.0-20240116215550-a9fa1716bcac
	google.golang.org/genproto/googleapis/api v0.0.0-20240125205218-1f4bbc51befe
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240116215550-a9fa1716bcac
	google.golang.org/grpc v1.61.0
	google.golang.org/protobuf v1.32.0
	gopkg.in/fsnotify.v1 v1.4.7
	gopkg.in/olivere/elastic.v5 v5.0.86
	gopkg.in/yaml.v2 v2.4.0
	gopkg.in/yaml.v3 v3.0.1
	howett.net/plist v1.0.0
	k8s.io/api v0.22.12
	k8s.io/apimachinery v0.22.12
	k8s.io/client-go v0.22.12
	k8s.io/kubectl v0.22.12
	sigs.k8s.io/yaml v1.2.0
)

require (
	cloud.google.com/go/compute v1.23.3 // indirect
	cloud.google.com/go/container v1.29.0 // indirect
	cloud.google.com/go/longrunning v0.5.4 // indirect
	cloud.google.com/go/trace v1.10.4 // indirect
	github.com/Masterminds/goutils v1.1.1 // indirect
	github.com/andybalholm/brotli v1.0.4 // indirect
	github.com/apache/arrow/go/v12 v12.0.0 // indirect
	github.com/apache/thrift v0.16.0 // indirect
	github.com/aws/aws-sdk-go v1.35.18 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/census-instrumentation/opencensus-proto v0.4.1 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/chigopher/pathlib v0.15.0 // indirect
	github.com/cncf/udpa/go v0.0.0-20220112060539-c52dc94e7fbe // indirect
	github.com/cncf/xds/go v0.0.0-20231109132714-523115ebc101 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.2 // indirect
	github.com/danjacques/gofslock v0.0.0-20230728142113-ae8f59f9e88b // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/emirpasic/gods v1.12.0 // indirect
	github.com/envoyproxy/go-control-plane v0.11.1 // indirect
	github.com/envoyproxy/protoc-gen-validate v1.0.2 // indirect
	github.com/facebookgo/clock v0.0.0-20150410010913-600d898af40a // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fsnotify/fsnotify v1.6.0 // indirect
	github.com/go-logr/logr v1.4.1 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/goccy/go-json v0.9.11 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/glog v1.1.2 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/flatbuffers v2.0.8+incompatible // indirect
	github.com/google/go-querystring v1.0.0 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/licenseclassifier v0.0.0-20210722185704-3043a050f148 // indirect
	github.com/google/martian/v3 v3.3.3 // indirect
	github.com/google/s2a-go v0.1.7 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.2 // indirect
	github.com/googleapis/gnostic v0.5.5 // indirect
	github.com/grpc-ecosystem/go-grpc-middleware v1.3.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/huandu/xstrings v1.4.0 // indirect
	github.com/iancoleman/orderedmap v0.0.0-20190318233801-ac98e3ecb4b0 // indirect
	github.com/iancoleman/strcase v0.2.0 // indirect
	github.com/imdario/mergo v0.3.11 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jackc/chunkreader/v2 v2.0.1 // indirect
	github.com/jackc/pgio v1.0.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgproto3/v2 v2.3.2 // indirect
	github.com/jackc/pgservicefile v0.0.0-20221227161230-091c0ba34f0a // indirect
	github.com/jackc/puddle v1.3.0 // indirect
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/jcgregorio/slog v0.0.0-20190423190439-e6f2d537f900 // indirect
	github.com/jinzhu/copier v0.3.5 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/julienschmidt/httprouter v1.3.0 // indirect
	github.com/kevinburke/ssh_config v0.0.0-20190725054713-01f96b0aa0cd // indirect
	github.com/klauspost/asmfmt v1.3.2 // indirect
	github.com/klauspost/compress v1.16.7 // indirect
	github.com/klauspost/cpuid/v2 v2.0.9 // indirect
	github.com/lib/pq v1.10.2 // indirect
	github.com/magiconair/properties v1.8.7 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.17 // indirect
	github.com/mattn/go-runewidth v0.0.7 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.2-0.20181231171920-c182affec369 // indirect
	github.com/minio/asm2plan9s v0.0.0-20200509001527-cdd76441f9d8 // indirect
	github.com/minio/c2goasm v0.0.0-20190812172519-36a3d3bbc4f3 // indirect
	github.com/mitchellh/copystructure v1.0.0 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/go-wordwrap v1.0.1 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.1 // indirect
	github.com/moby/spdystream v0.2.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/mostynb/zstdpool-syncpool v0.0.12 // indirect
	github.com/nxadm/tail v1.4.5 // indirect
	github.com/onsi/ginkgo v1.14.2 // indirect
	github.com/onsi/gomega v1.10.3 // indirect
	github.com/pborman/uuid v1.2.1 // indirect
	github.com/pelletier/go-toml/v2 v2.0.6 // indirect
	github.com/pierrec/lz4/v4 v4.1.15 // indirect
	github.com/pkg/xattr v0.4.9 // indirect
	github.com/prometheus/client_model v0.4.0 // indirect
	github.com/prometheus/common v0.26.0 // indirect
	github.com/prometheus/procfs v0.6.0 // indirect
	github.com/robertkrimen/otto v0.0.0-20200922221731-ef014fd054ac // indirect
	github.com/robfig/cron v1.2.0 // indirect
	github.com/rs/zerolog v1.29.0 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/santhosh-tekuri/jsonschema/v5 v5.1.1 // indirect
	github.com/sendgrid/rest v2.6.9+incompatible // indirect
	github.com/sergi/go-diff v1.3.1 // indirect
	github.com/spf13/afero v1.9.3 // indirect
	github.com/spf13/cast v1.5.0 // indirect
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/spf13/viper v1.15.0 // indirect
	github.com/src-d/gcfg v1.4.0 // indirect
	github.com/stretchr/objx v0.5.0 // indirect
	github.com/subosito/gotenv v1.4.2 // indirect
	github.com/tklauser/go-sysconf v0.3.10 // indirect
	github.com/tklauser/numcpus v0.4.0 // indirect
	github.com/xanzy/ssh-agent v0.2.1 // indirect
	github.com/xeipuuv/gojsonpointer v0.0.0-20180127040702-4e3ac2762d5f // indirect
	github.com/xeipuuv/gojsonreference v0.0.0-20180127040603-bd5ef7bd5415 // indirect
	github.com/xo/terminfo v0.0.0-20220910002029-abceb7e1c41e // indirect
	github.com/xrash/smetrics v0.0.0-20201216005158-039620a65673 // indirect
	github.com/zeebo/xxh3 v1.0.2 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.47.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/httptrace/otelhttptrace v0.46.1 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.47.0 // indirect
	go.opentelemetry.io/otel v1.22.0 // indirect
	go.opentelemetry.io/otel/metric v1.22.0 // indirect
	go.opentelemetry.io/otel/trace v1.22.0 // indirect
	go.uber.org/atomic v1.9.0 // indirect
	golang.org/x/crypto v0.18.0 // indirect
	golang.org/x/mod v0.14.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	golang.org/x/tools/go/vcs v0.1.0-deprecated // indirect
	golang.org/x/xerrors v0.0.0-20220907171357-04be3eba64a2 // indirect
	google.golang.org/appengine v1.6.8 // indirect
	google.golang.org/genproto/googleapis/bytestream v0.0.0-20240116215550-a9fa1716bcac // indirect
	gopkg.in/cenkalti/backoff.v1 v1.1.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
	gopkg.in/sourcemap.v1 v1.0.5 // indirect
	gopkg.in/src-d/go-billy.v4 v4.3.2 // indirect
	gopkg.in/src-d/go-git.v4 v4.13.1 // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
	k8s.io/klog/v2 v2.110.1 // indirect
	k8s.io/utils v0.0.0-20211116205334-6203023598ed // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.2.1 // indirect
)

go 1.21
