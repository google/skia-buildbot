#Use Vulcanize 1.0
VULCANIZE1=true
# The list of files we want to go into core.js, which is concat'd and
# minified. These files should be either present in the project, brought
# into third_party/bower_compoents via bower, or in node_modules.
CORE_SOURCE_FILES = third_party/bower_components/webcomponentsjs/webcomponents.min.js \
          ../res/js/common.js \
          res/js/status_utils.js

BOWER_DIR=third_party/bower_components

include ../kube/kube.mk

.PHONY: service_definition
service_definition: 
	go get -u github.com/golang/protobuf/protoc-gen-go
	go get -u github.com/twitchtv/twirp/protoc-gen-twirp
	go get -u go.larrymyers.com/protoc-gen-twirp_typescript
	protoc --proto_path=${GOPATH}/src:. --twirp_out=. --go_out=. go/rpc/statusFe.proto
	# To appease the tryjob gods?
	goimports -w go/rpc/statusFe.pb.go 
	goimports -w go/rpc/statusFe.twirp.go 
	

.PHONY: all
all: clean_webtools status

# Build the web assets in production mode.
.PHONY: web
web: webpack touch_elements core_js elements_html
	ln -sf ../../res res/imp/res

.PHONY: touch_elements
touch_elements:
	rm -f res/js/core.js
	touch elements.html

testgo:
	go test ./go/... -v

status: web
	CGO_ENABLED=0 GOOS=linux go install -a ./go/status

release: status
	./build_release

run_docker: release
	docker run status

push: release pushk
	pushk --message="$(MESSAGE)" --use-temp-checkout status

# Build debug versions of core.js and elements.html.
.PHONY: debug
debug: clean_webtools debug_core_js debug_elements_html

include ../webtools/webtools.mk
include webpack/webpack.mk
