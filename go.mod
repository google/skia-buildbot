module go.skia.org/infra

require (
	cloud.google.com/go/bigtable v1.6.0
	cloud.google.com/go/compute v1.5.0
	cloud.google.com/go/datastore v1.3.0
	cloud.google.com/go/firestore v1.6.1
	cloud.google.com/go/iam v0.3.0
	cloud.google.com/go/logging v1.1.1
	cloud.google.com/go/monitoring v1.5.0
	cloud.google.com/go/pubsub v1.8.2
	cloud.google.com/go/secretmanager v1.4.0
	cloud.google.com/go/storage v1.14.0
	contrib.go.opencensus.io/exporter/stackdriver v0.13.4
	github.com/Jeffail/gabs/v2 v2.6.0
	github.com/Masterminds/semver v1.5.0
	github.com/Masterminds/sprig v2.22.0+incompatible
	github.com/a8m/envsubst v1.2.0
	github.com/aclements/go-moremath v0.0.0-20190830160640-d16893ddf098
	github.com/alecthomas/jsonschema v0.0.0-20210526225647-edb03dcab7bc
	github.com/bazelbuild/bazel-gazelle v0.23.0
	github.com/bazelbuild/buildtools v0.0.0-20201102150426-f0f162f0456b
	github.com/bazelbuild/remote-apis v0.0.0-20201209220655-9e72daff42c9
	github.com/bazelbuild/remote-apis-sdks v0.0.0-20201110004117-e776219c9bb7
	github.com/blakesmith/ar v0.0.0-20190502131153-809d4375e1fb
	github.com/bradfitz/gomemcache v0.0.0-20190913173617-a41fca850d0b
	github.com/cenkalti/backoff v2.2.1+incompatible
	github.com/cenkalti/backoff/v4 v4.1.3
	github.com/cockroachdb/cockroach-go/v2 v2.1.0
	github.com/davecgh/go-spew v1.1.1
	github.com/dustin/go-humanize v1.0.0
	github.com/fiorix/go-web v1.0.1-0.20150221144011-5b593f1e8966
	github.com/flynn/json5 v0.0.0-20160717195620-7620272ed633
	github.com/go-python/gpython v0.0.3
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da
	github.com/golang/mock v1.6.0
	github.com/golang/protobuf v1.5.2
	github.com/google/go-github/v29 v29.0.3
	github.com/google/go-licenses v0.0.0-20210816172045-3099c18c36e1
	github.com/google/uuid v1.1.2
	github.com/googleapis/gax-go/v2 v2.3.0
	github.com/gorilla/mux v1.8.0
	github.com/gorilla/securecookie v1.1.1
	github.com/hako/durafmt v0.0.0-20200710122514-c0fb7b4da026
	github.com/hashicorp/go-multierror v1.1.0
	github.com/hashicorp/golang-lru v0.5.4
	github.com/jackc/pgconn v1.12.1
	github.com/jackc/pgtype v1.11.0
	github.com/jackc/pgx/v4 v4.16.1
	github.com/jcgregorio/logger v0.1.3
	github.com/kballard/go-shellquote v0.0.0-20180428030007-95032a82bc51
	github.com/kisielk/errcheck v1.5.0
	github.com/miekg/dns v1.1.41
	github.com/nfnt/resize v0.0.0-20180221191011-83c6a9932646
	github.com/otiai10/copy v1.6.0
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/pmezard/go-difflib v1.0.0
	github.com/prometheus/client_golang v1.11.0
	github.com/r3labs/sse/v2 v2.8.1
	github.com/rs/cors v1.6.0
	github.com/sendgrid/sendgrid-go v3.11.1+incompatible
	github.com/shirou/gopsutil v3.21.11+incompatible
	github.com/skia-dev/go2ts v1.5.0
	github.com/skia-dev/google-api-go-client v0.10.1-0.20200109184256-16c3d6f408b2
	github.com/skia-dev/protoc-gen-twirp_typescript v0.0.0-20220429132620-ad26708b7787
	github.com/spf13/cobra v1.3.0
	github.com/stretchr/testify v1.7.0
	github.com/syndtr/goleveldb v1.0.0
	github.com/tarm/serial v0.0.0-20180830185346-98f6abe2eb07
	github.com/texttheater/golang-levenshtein v1.0.1
	github.com/twitchtv/twirp v7.1.0+incompatible
	github.com/unrolled/secure v1.0.8
	github.com/urfave/cli/v2 v2.3.0
	github.com/vektra/mockery/v2 v2.11.0
	github.com/willf/bitset v1.1.11
	github.com/xeipuuv/gojsonschema v1.2.0
	github.com/yusufpapurcu/wmi v1.2.2
	github.com/zeebo/bencode v1.0.0
	go.chromium.org/luci v0.0.0-20201029184154-594d11850ebf
	go.opencensus.io v0.23.0
	golang.org/x/net v0.7.0
	golang.org/x/oauth2 v0.0.0-20220309155454-6242fa91716a
	golang.org/x/sync v0.0.0-20220722155255-886fb9371eb4
	golang.org/x/sys v0.5.0
	golang.org/x/term v0.5.0
	golang.org/x/time v0.0.0-20220609170525-579cf78fd858
	golang.org/x/tools v0.1.12
	google.golang.org/api v0.74.0
	google.golang.org/genproto v0.0.0-20220426171045-31bebdecfb46
	google.golang.org/grpc v1.45.0
	google.golang.org/protobuf v1.28.0
	gopkg.in/fsnotify.v1 v1.4.7
	gopkg.in/olivere/elastic.v5 v5.0.86
	gopkg.in/yaml.v2 v2.4.0
	howett.net/plist v1.0.0
	k8s.io/api v0.22.12
	k8s.io/apimachinery v0.22.12
	k8s.io/client-go v0.22.12
	k8s.io/kubectl v0.22.12
	sigs.k8s.io/yaml v1.2.0
)

require (
	cloud.google.com/go v0.100.2 // indirect
	cloud.google.com/go/container v1.2.0 // indirect
	cloud.google.com/go/kms v1.4.0 // indirect
	cloud.google.com/go/trace v1.2.0 // indirect
	github.com/Masterminds/goutils v1.1.0 // indirect
	github.com/aws/aws-sdk-go v1.35.18 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/census-instrumentation/opencensus-proto v0.3.0 // indirect
	github.com/cespare/xxhash/v2 v2.1.2 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.1 // indirect
	github.com/danjacques/gofslock v0.0.0-20200623023034-5d0bd0fa6ef0 // indirect
	github.com/emirpasic/gods v1.12.0 // indirect
	github.com/fsnotify/fsnotify v1.5.1 // indirect
	github.com/go-logr/logr v0.4.0 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b // indirect
	github.com/golang/snappy v0.0.3 // indirect
	github.com/google/go-cmp v0.5.7 // indirect
	github.com/google/go-querystring v1.0.0 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/licenseclassifier v0.0.0-20210722185704-3043a050f148 // indirect
	github.com/googleapis/gnostic v0.5.5 // indirect
	github.com/gopherjs/gopherjs v0.0.0-20200217142428-fce0ec30dd00 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/huandu/xstrings v1.3.2 // indirect
	github.com/iancoleman/orderedmap v0.0.0-20190318233801-ac98e3ecb4b0 // indirect
	github.com/imdario/mergo v0.3.11 // indirect
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/jackc/chunkreader/v2 v2.0.1 // indirect
	github.com/jackc/pgio v1.0.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgproto3/v2 v2.3.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20200714003250-2b9c44734f2b // indirect
	github.com/jackc/puddle v1.2.1 // indirect
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/jcgregorio/slog v0.0.0-20190423190439-e6f2d537f900 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/julienschmidt/httprouter v1.3.0 // indirect
	github.com/kevinburke/ssh_config v0.0.0-20190725054713-01f96b0aa0cd // indirect
	github.com/lib/pq v1.10.2 // indirect
	github.com/luci/gtreap v0.0.0-20161228054646-35df89791e8f // indirect
	github.com/magiconair/properties v1.8.5 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.2-0.20181231171920-c182affec369 // indirect
	github.com/mitchellh/copystructure v1.0.0 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/mapstructure v1.4.3 // indirect
	github.com/mitchellh/reflectwalk v1.0.1 // indirect
	github.com/moby/spdystream v0.2.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/nxadm/tail v1.4.5 // indirect
	github.com/onsi/ginkgo v1.14.2 // indirect
	github.com/onsi/gomega v1.10.3 // indirect
	github.com/op/go-logging v0.0.0-20160315200505-970db520ece7 // indirect
	github.com/pborman/uuid v1.2.1 // indirect
	github.com/pelletier/go-toml v1.9.4 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/prometheus/client_model v0.2.0 // indirect
	github.com/prometheus/common v0.26.0 // indirect
	github.com/prometheus/procfs v0.6.0 // indirect
	github.com/robertkrimen/otto v0.0.0-20200922221731-ef014fd054ac // indirect
	github.com/rs/zerolog v1.26.1 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/sendgrid/rest v2.6.9+incompatible // indirect
	github.com/sergi/go-diff v1.2.0 // indirect
	github.com/smartystreets/assertions v1.2.0 // indirect
	github.com/spf13/afero v1.8.0 // indirect
	github.com/spf13/cast v1.4.1 // indirect
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/spf13/viper v1.10.1 // indirect
	github.com/src-d/gcfg v1.4.0 // indirect
	github.com/stretchr/objx v0.3.0 // indirect
	github.com/subosito/gotenv v1.2.0 // indirect
	github.com/tklauser/go-sysconf v0.3.10 // indirect
	github.com/tklauser/numcpus v0.4.0 // indirect
	github.com/xanzy/ssh-agent v0.2.1 // indirect
	github.com/xeipuuv/gojsonpointer v0.0.0-20180127040702-4e3ac2762d5f // indirect
	github.com/xeipuuv/gojsonreference v0.0.0-20180127040603-bd5ef7bd5415 // indirect
	golang.org/x/crypto v0.0.0-20220112180741-5e0467b6c7ce // indirect
	golang.org/x/mod v0.6.0-dev.0.20220419223038-86c51ed26bb4 // indirect
	golang.org/x/text v0.7.0 // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	gopkg.in/cenkalti/backoff.v1 v1.1.0 // indirect
	gopkg.in/check.v1 v1.0.0-20200902074654-038fdea0a05b // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/ini.v1 v1.66.3 // indirect
	gopkg.in/sourcemap.v1 v1.0.5 // indirect
	gopkg.in/src-d/go-billy.v4 v4.3.2 // indirect
	gopkg.in/src-d/go-git.v4 v4.13.1 // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
	k8s.io/klog/v2 v2.9.0 // indirect
	k8s.io/utils v0.0.0-20211116205334-6203023598ed // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.2.1 // indirect
)

go 1.19
