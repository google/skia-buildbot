module go.skia.org/infra

exclude (
	// NOTE: "go get", "go mod tidy", etc will re-order these excludes, so
	// we can't simply group them under one comment. Instead, add a comment
	// near the top of this section and then add comments at the end of
	// specific exclude lines pointing to them.

	// 1. gnostic v0.4.1 renames a package, which breaks k8s.io/client-go.
	// This should be temporary, until client-go updates to use the new
	// package name. New excludes may need to be added, in the event that
	// new versions of gnostic are released before client-go updates.

	// 2. k8s.io/client-go had a number of releases before adopting go
	// modules, and those releases are now incompatible with go modules due
	// to their module path. After switching to go modules, client-go
	// started using v0.x.y versions, which makes the module path compatible
	// but breaks the assumption of "go get -u" that higher-numbered
	// releases are newer. So we have to ignore these tags indefinitely or
	// until client-go releases go modules-compatible versions which are
	// higher than these old versions.

	github.com/googleapis/gnostic v0.4.1 // #1
	k8s.io/client-go v1.4.0 // #2
	k8s.io/client-go v1.5.0 // #2
	k8s.io/client-go v1.5.1 // #2
	k8s.io/client-go v10.0.0+incompatible // #2
	k8s.io/client-go v11.0.0+incompatible // #2
	k8s.io/client-go v12.0.0+incompatible // #2
	k8s.io/client-go v2.0.0+incompatible // #2
	k8s.io/client-go v3.0.0+incompatible // #2
	k8s.io/client-go v4.0.0+incompatible // #2
	k8s.io/client-go v5.0.0+incompatible // #2
	k8s.io/client-go v5.0.1+incompatible // #2
	k8s.io/client-go v6.0.0+incompatible // #2
	k8s.io/client-go v7.0.0+incompatible // #2
	k8s.io/client-go v8.0.0+incompatible // #2
	k8s.io/client-go v9.0.0+incompatible // #2
)

// We're using this fork-of-a-fork which contains various bug fixes and adds
// support for things like enums.  We can remove it if/when those changes ever
// get upstreamed.
replace go.larrymyers.com/protoc-gen-twirp_typescript => github.com/skia-dev/protoc-gen-twirp_typescript v0.0.0-20200902150932-4a52797b9171

require (
	cloud.google.com/go v0.70.0
	cloud.google.com/go/bigtable v1.6.0
	cloud.google.com/go/datastore v1.3.0
	cloud.google.com/go/firestore v1.3.0
	cloud.google.com/go/logging v1.1.1
	cloud.google.com/go/pubsub v1.8.2
	cloud.google.com/go/storage v1.12.0
	contrib.go.opencensus.io/exporter/stackdriver v0.13.4
	github.com/Jeffail/gabs/v2 v2.6.0
	github.com/Masterminds/goutils v1.1.0 // indirect
	github.com/Masterminds/semver v1.5.0 // indirect
	github.com/Masterminds/sprig v2.22.0+incompatible
	github.com/PuerkitoBio/goquery v1.6.0
	github.com/a8m/envsubst v1.2.0
	github.com/aclements/go-moremath v0.0.0-20190830160640-d16893ddf098
	github.com/alecthomas/jsonschema v0.0.0-20210526225647-edb03dcab7bc // indirect
	github.com/andybalholm/cascadia v1.2.0 // indirect
	github.com/aws/aws-sdk-go v1.35.18 // indirect
	github.com/bazelbuild/bazel-gazelle v0.23.0
	github.com/bazelbuild/buildtools v0.0.0-20201102150426-f0f162f0456b // indirect
	github.com/bazelbuild/remote-apis v0.0.0-20201209220655-9e72daff42c9
	github.com/bazelbuild/remote-apis-sdks v0.0.0-20201110004117-e776219c9bb7
	github.com/bradfitz/gomemcache v0.0.0-20190913173617-a41fca850d0b
	github.com/cenkalti/backoff v2.2.1+incompatible
	github.com/census-instrumentation/opencensus-proto v0.3.0 // indirect
	github.com/cockroachdb/cockroach-go/v2 v2.1.0
	github.com/containerd/containerd v1.4.1 // indirect
	github.com/coreos/go-systemd/v22 v22.1.0
	github.com/danjacques/gofslock v0.0.0-20200623023034-5d0bd0fa6ef0 // indirect
	github.com/davecgh/go-spew v1.1.1
	github.com/dgraph-io/ristretto v0.0.3
	github.com/docker/spdystream v0.0.0-20160310174837-449fdfce4d96 // indirect
	github.com/fiorix/go-web v1.0.1-0.20150221144011-5b593f1e8966
	github.com/flynn/json5 v0.0.0-20160717195620-7620272ed633
	github.com/go-python/gpython v0.0.3
	github.com/gofrs/uuid v3.3.0+incompatible // indirect
	github.com/golang-migrate/migrate/v4 v4.13.0
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/golang/groupcache v0.0.0-20200121045136-8c9f03a8e57e
	github.com/golang/mock v1.4.4
	github.com/golang/protobuf v1.4.3
	github.com/golang/snappy v0.0.2 // indirect
	github.com/google/go-github/v29 v29.0.3
	github.com/google/go-licenses v0.0.0-20210727195037-24e2421190d8
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/martian/v3 v3.1.0 // indirect
	github.com/google/uuid v1.1.2
	github.com/googleapis/gnostic v0.5.3 // indirect
	github.com/gopherjs/gopherjs v0.0.0-20200217142428-fce0ec30dd00 // indirect
	github.com/gorilla/mux v1.8.0
	github.com/gorilla/securecookie v1.1.1
	github.com/hako/durafmt v0.0.0-20200710122514-c0fb7b4da026
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.0
	github.com/hashicorp/golang-lru v0.5.4
	github.com/huandu/xstrings v1.3.2 // indirect
	github.com/imdario/mergo v0.3.11 // indirect
	github.com/jackc/pgtype v1.6.1
	github.com/jackc/pgx/v4 v4.9.2
	github.com/jcgregorio/logger v0.1.2
	github.com/jcgregorio/slog v0.0.0-20190423190439-e6f2d537f900
	github.com/kisielk/errcheck v1.5.0
	github.com/kr/text v0.2.0 // indirect
	github.com/lib/pq v1.8.0 // indirect
	github.com/luci/gtreap v0.0.0-20161228054646-35df89791e8f // indirect
	github.com/magiconair/properties v1.8.4 // indirect
	github.com/mitchellh/copystructure v1.0.0 // indirect
	github.com/mitchellh/mapstructure v1.3.3 // indirect
	github.com/mitchellh/reflectwalk v1.0.1 // indirect
	github.com/nfnt/resize v0.0.0-20180221191011-83c6a9932646
	github.com/niemeyer/pretty v0.0.0-20200227124842-a10e7caefd8e // indirect
	github.com/nxadm/tail v1.4.5 // indirect
	github.com/onsi/ginkgo v1.14.2 // indirect
	github.com/onsi/gomega v1.10.3 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/pelletier/go-toml v1.8.1 // indirect
	github.com/pmezard/go-difflib v1.0.0
	github.com/prometheus/client_golang v1.8.0
	github.com/robertkrimen/otto v0.0.0-20200922221731-ef014fd054ac // indirect
	github.com/rs/cors v1.6.0
	github.com/shopspring/decimal v1.2.0 // indirect
	github.com/sirupsen/logrus v1.7.0 // indirect
	github.com/skia-dev/go2ts v1.5.0
	github.com/skia-dev/google-api-go-client v0.10.1-0.20200109184256-16c3d6f408b2
	github.com/smartystreets/assertions v1.2.0 // indirect
	github.com/spf13/afero v1.4.1 // indirect
	github.com/spf13/cast v1.3.1 // indirect
	github.com/spf13/cobra v1.1.1
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/viper v1.7.1
	github.com/stretchr/objx v0.3.0 // indirect
	github.com/stretchr/testify v1.6.1
	github.com/syndtr/goleveldb v1.0.0
	github.com/texttheater/golang-levenshtein v1.0.1
	github.com/twitchtv/twirp v7.1.0+incompatible
	github.com/unrolled/secure v1.0.8
	github.com/urfave/cli/v2 v2.3.0
	github.com/willf/bitset v1.1.11
	github.com/xeipuuv/gojsonschema v1.2.0
	github.com/zeebo/bencode v1.0.0
	go.chromium.org/luci v0.0.0-20201029184154-594d11850ebf
	go.larrymyers.com/protoc-gen-twirp_typescript v0.0.0-20201012232926-5c91a3223921
	go.opencensus.io v0.22.5
	golang.org/x/oauth2 v0.0.0-20200902213428-5d25da1a8d43
	golang.org/x/sync v0.0.0-20201020160332-67f06af15bc9
	golang.org/x/time v0.0.0-20210220033141-f8bda1e9f3ba
	golang.org/x/tools v0.1.0
	google.golang.org/api v0.34.0
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto v0.0.0-20201029200359-8ce4113da6f7
	google.golang.org/grpc v1.33.1
	google.golang.org/protobuf v1.25.0
	gopkg.in/check.v1 v1.0.0-20200902074654-038fdea0a05b // indirect
	gopkg.in/fsnotify.v1 v1.4.7
	gopkg.in/ini.v1 v1.62.0 // indirect
	gopkg.in/olivere/elastic.v5 v5.0.86
	gopkg.in/sourcemap.v1 v1.0.5 // indirect
	gopkg.in/yaml.v2 v2.4.0
	k8s.io/api v0.21.0
	k8s.io/apimachinery v0.21.0
	k8s.io/client-go v0.21.0
	k8s.io/kubectl v0.21.0
)

go 1.16
