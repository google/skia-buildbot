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

require (
	cloud.google.com/go v0.58.0
	cloud.google.com/go/bigtable v1.3.0
	cloud.google.com/go/datastore v1.1.0
	cloud.google.com/go/firestore v1.2.0
	cloud.google.com/go/logging v1.0.0
	cloud.google.com/go/pubsub v1.4.0
	cloud.google.com/go/storage v1.9.0
	contrib.go.opencensus.io/exporter/stackdriver v0.13.1
	github.com/GeertJohan/go.rice v1.0.0
	github.com/Jeffail/gabs/v2 v2.5.1
	github.com/Masterminds/goutils v1.1.0 // indirect
	github.com/Masterminds/semver v1.5.0 // indirect
	github.com/Masterminds/sprig v2.22.0+incompatible
	github.com/OneOfOne/struct2ts v1.0.4 // indirect
	github.com/PuerkitoBio/goquery v1.5.1
	github.com/a8m/envsubst v1.1.0
	github.com/alecthomas/units v0.0.0-20190924025748-f65c72e2690d // indirect
	github.com/andybalholm/cascadia v1.2.0 // indirect
	github.com/aws/aws-sdk-go v1.31.14 // indirect
	github.com/cenkalti/backoff v2.2.1+incompatible
	github.com/codegangsta/negroni v1.0.0 // indirect
	github.com/daaku/go.zipexe v1.0.1 // indirect
	github.com/danjacques/gofslock v0.0.0-20191023191349-0a45f885bc37 // indirect
	github.com/davecgh/go-spew v1.1.1
	github.com/docker/distribution v2.7.1+incompatible
	github.com/dustin/go-humanize v1.0.0 // indirect
	github.com/fiorix/go-web v1.0.1-0.20150221144011-5b593f1e8966
	github.com/flynn/json5 v0.0.0-20160717195620-7620272ed633
	github.com/fsnotify/fsnotify v1.4.9 // indirect
	github.com/go-python/gpython v0.0.3
	github.com/godbus/dbus v0.0.0-20181101234600-2ff6f7ffd60f // indirect
	github.com/golang-migrate/migrate/v4 v4.11.0
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/golang/groupcache v0.0.0-20200121045136-8c9f03a8e57e
	github.com/golang/mock v1.4.3
	github.com/golang/protobuf v1.4.2
	github.com/google/go-github/v29 v29.0.3
	github.com/google/go-licenses v0.0.0-20200602185517-f29a4c695c3d
	github.com/google/licenseclassifier v0.0.0-20200402202327-879cb1424de0 // indirect
	github.com/google/uuid v1.1.1
	github.com/googleapis/gnostic v0.4.0 // indirect
	github.com/gopherjs/gopherjs v0.0.0-20190915194858-d3ddacdb130f // indirect
	github.com/gorilla/csrf v1.7.0
	github.com/gorilla/mux v1.7.4
	github.com/gorilla/securecookie v1.1.1
	github.com/hashicorp/go-multierror v1.1.0
	github.com/hashicorp/golang-lru v0.5.4
	github.com/huandu/xstrings v1.3.1 // indirect
	github.com/imdario/mergo v0.3.9 // indirect
	github.com/jcgregorio/logger v0.1.2
	github.com/jcgregorio/slog v0.0.0-20190423190439-e6f2d537f900
	github.com/json-iterator/go v1.1.10 // indirect
	github.com/julienschmidt/httprouter v1.3.0 // indirect
	github.com/kisielk/errcheck v1.2.0
	github.com/kr/text v0.2.0 // indirect
	github.com/kylelemons/godebug v1.1.0 // indirect
	github.com/maruel/subcommands v0.0.0-20200206125935-de1d40e70d4b // indirect
	github.com/maruel/ut v1.0.1 // indirect
	github.com/mattn/go-runewidth v0.0.9 // indirect
	github.com/mitchellh/copystructure v1.0.0 // indirect
	github.com/mitchellh/mapstructure v1.3.2 // indirect
	github.com/mitchellh/reflectwalk v1.0.1 // indirect
	github.com/nfnt/resize v0.0.0-20180221191011-83c6a9932646
	github.com/niemeyer/pretty v0.0.0-20200227124842-a10e7caefd8e // indirect
	github.com/op/go-logging v0.0.0-20160315200505-970db520ece7 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/pelletier/go-toml v1.8.0 // indirect
	github.com/peterh/liner v1.2.0 // indirect
	github.com/pmezard/go-difflib v1.0.0
	github.com/prometheus/client_golang v1.6.0
	github.com/robertkrimen/otto v0.0.0-20191219234010-c382bd3c16ff // indirect
	github.com/russross/blackfriday/v2 v2.0.1
	github.com/sergi/go-diff v1.1.0 // indirect
	github.com/skia-dev/go-systemd v0.0.0-20181025131956-1cc903e82ae4
	github.com/skia-dev/go2ts v1.3.0
	github.com/skia-dev/google-api-go-client v0.10.1-0.20200109184256-16c3d6f408b2
	github.com/spf13/cast v1.3.1 // indirect
	github.com/spf13/cobra v1.0.0
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/viper v1.7.0
	github.com/stretchr/testify v1.6.1
	github.com/syndtr/goleveldb v1.0.0
	github.com/texttheater/golang-levenshtein v0.0.0-20191208221605-eb6844b05fc6
	github.com/unrolled/secure v1.0.8
	github.com/vektra/mockery v0.0.0-20181123154057-e78b021dcbb5
	github.com/willf/bitset v1.1.10
	github.com/yosuke-furukawa/json5 v0.1.1 // indirect
	github.com/zeebo/bencode v1.0.0
	go.chromium.org/gae v0.0.0-20190826183307-50a499513efa // indirect
	go.chromium.org/luci v0.0.0-20200610031951-aa35210c29b0
	go.opencensus.io v0.22.3
	golang.org/x/crypto v0.0.0-20200604202706-70a84ac30bf9 // indirect
	golang.org/x/net v0.0.0-20200602114024-627f9648deb9 // indirect
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d
	golang.org/x/sync v0.0.0-20200317015054-43a5402ce75a
	golang.org/x/sys v0.0.0-20200602225109-6fdc65e7d980 // indirect
	golang.org/x/time v0.0.0-20200416051211-89c76fbcd5d1
	golang.org/x/tools v0.0.0-20200609164405-eb789aa7ce50
	google.golang.org/api v0.26.0
	google.golang.org/genproto v0.0.0-20200608115520-7c474a2e3482
	google.golang.org/grpc v1.29.1
	google.golang.org/protobuf v1.24.0
	gopkg.in/check.v1 v1.0.0-20200227125254-8fa46927fb4f // indirect
	gopkg.in/fsnotify.v1 v1.4.7
	gopkg.in/ini.v1 v1.57.0 // indirect
	gopkg.in/olivere/elastic.v5 v5.0.85
	gopkg.in/sourcemap.v1 v1.0.5 // indirect
	gopkg.in/yaml.v2 v2.3.0
	gopkg.in/yaml.v3 v3.0.0-20200605160147-a5ece683394c // indirect
	k8s.io/api v0.18.3
	k8s.io/apimachinery v0.18.3
	k8s.io/client-go v0.18.3
	rsc.io/sampler v1.99.99 // indirect
)

go 1.13
