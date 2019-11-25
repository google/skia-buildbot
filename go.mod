module go.skia.org/infra

require (
	cloud.google.com/go v0.46.3
	cloud.google.com/go/bigtable v1.0.0
	cloud.google.com/go/datastore v1.0.0
	cloud.google.com/go/firestore v1.0.0
	cloud.google.com/go/logging v1.0.0
	cloud.google.com/go/pubsub v1.0.1
	cloud.google.com/go/storage v1.1.0
	contrib.go.opencensus.io/exporter/stackdriver v0.12.7
	github.com/99designs/goodies v0.0.0-20140916053233-ec7f410f2ff2
	github.com/Jeffail/gabs/v2 v2.1.0
	github.com/Masterminds/goutils v1.1.0 // indirect
	github.com/Masterminds/semver v1.5.0 // indirect
	github.com/Masterminds/sprig v2.22.0+incompatible
	github.com/PuerkitoBio/goquery v1.5.0
	github.com/VividCortex/godaemon v0.0.0-20150910212227-3d9f6e0b234f
	github.com/a8m/envsubst v1.1.0
	github.com/aws/aws-sdk-go v1.25.6 // indirect
	github.com/boltdb/bolt v1.3.1
	github.com/cenkalti/backoff v2.2.1+incompatible
	github.com/danjacques/gofslock v0.0.0-20180405201223-afa47669cc54 // indirect
	github.com/davecgh/go-spew v1.1.1
	github.com/dustin/go-humanize v1.0.0 // indirect
	github.com/fiorix/go-web v1.0.1-0.20150221144011-5b593f1e8966
	github.com/flynn/json5 v0.0.0-20160717195620-7620272ed633
	github.com/godbus/dbus v0.0.0-20181101234600-2ff6f7ffd60f // indirect
	github.com/gogo/protobuf v1.3.0
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/golang/groupcache v0.0.0-20191002201903-404acd9df4cc
	github.com/golang/mock v1.3.1
	github.com/golang/protobuf v1.3.2
	github.com/golang/snappy v0.0.1 // indirect
	github.com/google/go-github v17.0.0+incompatible
	github.com/google/go-querystring v1.0.0 // indirect
	github.com/google/uuid v1.1.1
	github.com/googleapis/gnostic v0.3.1 // indirect
	github.com/gopherjs/gopherjs v0.0.0-20190915194858-d3ddacdb130f // indirect
	github.com/gorilla/csrf v1.6.1
	github.com/gorilla/mux v1.7.3
	github.com/gorilla/securecookie v1.1.1
	github.com/hashicorp/go-multierror v1.0.0
	github.com/hashicorp/golang-lru v0.5.3
	github.com/huandu/xstrings v1.2.0 // indirect
	github.com/huin/goserial v0.0.0-20121012073615-7b90efdb22b1
	github.com/imdario/mergo v0.3.8 // indirect
	github.com/jcgregorio/logger v0.1.2
	github.com/jcgregorio/slog v0.0.0-20190423190439-e6f2d537f900
	github.com/jstemmer/go-junit-report v0.9.1 // indirect
	github.com/julienschmidt/httprouter v1.3.0 // indirect
	github.com/kylelemons/godebug v1.1.0 // indirect
	github.com/luci/gtreap v0.0.0-20161228054646-35df89791e8f // indirect
	github.com/maruel/subcommands v0.0.0-20181220013616-967e945be48b // indirect
	github.com/mitchellh/copystructure v1.0.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.1 // indirect
	github.com/nfnt/resize v0.0.0-20180221191011-83c6a9932646
	github.com/onsi/ginkgo v1.10.2 // indirect
	github.com/op/go-logging v0.0.0-20160315200505-970db520ece7 // indirect
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/pmezard/go-difflib v1.0.0
	github.com/prometheus/client_golang v1.1.0
	github.com/prometheus/client_model v0.0.0-20190812154241-14fe0d1b01d4 // indirect
	github.com/prometheus/common v0.7.0 // indirect
	github.com/prometheus/procfs v0.0.5 // indirect
	github.com/robertkrimen/otto v0.0.0-20180617131154-15f95af6e78d // indirect
	github.com/russross/blackfriday/v2 v2.0.1
	github.com/shurcooL/sanitized_anchor_name v1.0.0 // indirect
	github.com/skia-dev/go-systemd v0.0.0-20181025131956-1cc903e82ae4
	github.com/smartystreets/assertions v1.0.1 // indirect
	github.com/smartystreets/goconvey v0.0.0-20190731233626-505e41936337 // indirect
	github.com/spf13/cobra v0.0.5
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/spf13/viper v1.3.2
	github.com/stretchr/objx v0.2.0 // indirect
	github.com/stretchr/testify v1.4.0
	github.com/syndtr/goleveldb v1.0.0
	github.com/texttheater/golang-levenshtein v0.0.0-20190717060638-b7aaf30637d6
	github.com/unrolled/secure v1.0.4
	github.com/vektra/mockery v0.0.0-20181123154057-e78b021dcbb5 // indirect
	github.com/willf/bitset v1.1.10
	github.com/yosuke-furukawa/json5 v0.1.1 // indirect
	github.com/zeebo/bencode v1.0.0
	go.chromium.org/gae v0.0.0-20190826183307-50a499513efa // indirect
	go.chromium.org/luci v0.0.0-20191118041814-f12496792788
	go.opencensus.io v0.22.1
	golang.org/x/crypto v0.0.0-20191002192127-34f69633bfdc
	golang.org/x/exp v0.0.0-20191002040644-a1355ae1e2c3 // indirect
	golang.org/x/lint v0.0.0-20190930215403-16217165b5de // indirect
	golang.org/x/net v0.0.0-20191003171128-d98b1b443823
	golang.org/x/oauth2 v0.0.0-20190604053449-0f29369cfe45
	golang.org/x/sync v0.0.0-20190911185100-cd5d95a43a6e
	golang.org/x/sys v0.0.0-20191007092633-5f54ce542709 // indirect
	golang.org/x/time v0.0.0-20190921001708-c4c64cad1fd0
	golang.org/x/tools v0.0.0-20191005014404-c9f9432ec4b2 // indirect
	google.golang.org/api v0.10.0
	google.golang.org/appengine v1.6.4 // indirect
	google.golang.org/genproto v0.0.0-20191002211648-c459b9ce5143
	google.golang.org/grpc v1.24.0
	gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15 // indirect
	gopkg.in/fsnotify.v1 v1.4.7
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/olivere/elastic.v5 v5.0.82
	gopkg.in/sourcemap.v1 v1.0.5 // indirect
	gopkg.in/yaml.v2 v2.2.4
	k8s.io/api v0.0.0-20191005115622-2e41325d9e4b // indirect
	k8s.io/apimachinery v0.0.0-20191006235458-f9f2f3f8ab02
	k8s.io/client-go v0.0.0-20190425172711-65184652c889
	k8s.io/utils v0.0.0-20190923111123-69764acb6e8e // indirect
)

go 1.12
