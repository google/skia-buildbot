# The list of files we want to go into core.js, which is concat'd and
# minified. These files should be either present in the project, brought
# into third_party/bower_components via bower, or in node_modules.
CORE_SOURCE_FILES = node_modules/native-promise-only/npo.js \
  third_party/bower_components/webcomponentsjs/webcomponents-lite.min.js \
  ../res/js/common.js

BOWER_DIR=third_party/bower_components

VULCANIZE1=true

.PHONY: build
build: elements_html core_js
	go install -v ./go/fiddle
	go install -v ./go/fiddler
	go install -v ./go/fiddlecli
	go install -v ./go/smoketest
	mkdir -p ./bin
	clang++ -O2 ./cpp/fiddle_secwrap.cpp -o ./bin/fiddle_secwrap

.PHONY: fiddle_secwrap
fiddle_secwrap:
	mkdir -p ./bin
	clang++ -O2 ./cpp/fiddle_secwrap.cpp -o ./bin/fiddle_secwrap


.PHONY: image
image: build
	CGO_ENABLED=0 GOOS=linux go install -a ./go/fiddle
	docker pull gcr.io/skia-public/skia-release:prod
	SKIP_UPLOAD=1 ./build_fiddle_release
	SKIP_UPLOAD=1 ./build_fiddler_release

.PHONY: release
release: build
	CGO_ENABLED=0 GOOS=linux go install -a ./go/fiddle
	docker pull gcr.io/skia-public/skia-release:prod
	./build_fiddle_release
	./build_fiddler_release

.PHONY: release_ci
release_ci: build
	# Don't docker pull here, as we are in the CI pipeline and it already has
	# done the pull.
	CGO_ENABLED=0 GOOS=linux go install -a ./go/fiddle
	./build_fiddle_release
	./build_fiddler_release

.PHONY: push
push: release skia-public
	pushk fiddle fiddler

.PHONY: push_fiddle
push_fiddle: skia-public
	CGO_ENABLED=0 GOOS=linux go install -a ./go/fiddle
	./build_fiddle_release
	pushk fiddle

.PHONY: cli
cli:
	go install -v ./go/fiddlecli

.PHONY: test
test: testgo testcpp

.PHONY: testgo
testgo:
	go test -v ./...

.PHONY: build testcpp
testcpp:
	clang ./cpp/fail1.cpp -o ./bin/fail1
	clang ./cpp/fail2.cpp -o ./bin/fail2
	clang ./cpp/fail3.cpp -o ./bin/fail3
	clang ./cpp/fail4.cpp -o ./bin/fail4
	./bin/fiddle_secwrap ./bin/fail1; test $$? -eq 1
	./bin/fiddle_secwrap ./bin/fail2; test $$? -eq 255
	./bin/fiddle_secwrap ./bin/fail3; test $$? -eq 255
	./bin/fiddle_secwrap ./bin/fail4; test $$? -eq 255
	echo "SUCCESS"

.PHONY: run_local_fiddle
run_local_fiddle:
	fiddle --local --logtostderr --port=:8080 --source_image_dir=/etc/fiddle/source

.PHONY: run_local_fiddler
run_local_fiddler:
	docker run -ti -p8000:8000 --cap-add=SYS_PTRACE fiddler

# Build debug versions of core.js and elements.html.
.PHONY: debug
debug: clean_webtools debug_core_js debug_elements_html

include ../webtools/webtools.mk
include ../make/clusters.mk
