.PHONY: build
build: web
	go install -v ./go/fiddle
	go install -v ./go/fiddler
	go install -v ./go/fiddlecli
	go install -v ./go/smoketest
	mkdir -p ./bin
	clang++ -O2 ./cpp/fiddle_secwrap.cpp -o ./bin/fiddle_secwrap

web: ./modules/json/index.ts npm-ci
	npx webpack --mode=production

testjs: npm-ci
	xvfb-run --auto-servernum --server-args "-screen 0 1280x1024x24" npx karma start --single-run

debugjs: npm-ci
	npx karma start --no-single-run

watch: npm-ci
	npx webpack --mode=development --watch

.PHONY: build-frontend-ci
build-frontend-ci: npm-ci
	npx webpack --mode=production

.PHONY: test-frontend-ci
test-frontend-ci: npm-ci
	xvfb-run --auto-servernum --server-args "-screen 0 1280x1024x24" npx karma start --single-run

testpptr: npm-ci
	NODE_OPTIONS="--max-old-space-size=$(NODE_MEMORY_LIMIT)" npx mocha -v -r ts-node/register ./modules/**/*_puppeteer_test.ts

# First create a list of all the modules that ./go/ts depends upon.
tsDependentModules := $(shell go list -f '{{ join .Imports "\n" }}' go.skia.org/infra/fiddlek/go/ts/  )

# Then use that list to find all the Go files that make up those dependent modules.
tsDependentFiles := $(shell echo $(tsDependentModules) | xargs -L 1 go list -f '{{ $$dir := .Dir }}{{ range  .GoFiles }}{{ $$dir }}/{{ . }}{{ "\n" }}{{ end }}' )

./modules/json/index.ts: $(tsDependentFiles)
	go generate ./go/...

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
push: release
	pushk fiddle fiddler

.PHONY: push_fiddle
push_fiddle:
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

.PHONY: testcpp
testcpp: build
	clang ./cpp/fail_socket.cpp -o ./bin/fail_socket
	clang ./cpp/fail_execve.cpp -o ./bin/fail_execve
	clang ./cpp/fail_link.cpp -o ./bin/fail_link
	clang ./cpp/fail_rename.cpp -o ./bin/fail_rename
	clang ./cpp/fail_mknod.cpp -o ./bin/fail_mknod
	./bin/fiddle_secwrap ./bin/fail_socket; test $$? -eq 1
	./bin/fiddle_secwrap ./bin/fail_execve; test $$? -eq 255
	./bin/fiddle_secwrap ./bin/fail_link; test $$? -eq 255
	./bin/fiddle_secwrap ./bin/fail_rename; test $$? -eq 255
	./bin/fiddle_secwrap ./bin/fail_mknod; test $$? -eq 255
	echo "SUCCESS"

# This presumes a fiddler is available at port :8000 and scrapexchange is available at port :9000.
.PHONY: run_local_fiddle
run_local_fiddle:
	fiddle --local --logtostderr --port=:8080 --source_image_dir=/etc/fiddle/source --scrapexchange=localhost:9000

# Runs a local fiddler.
.PHONY: run_local_fiddler
run_local_fiddler:
	docker run -ti -p8000:8000 --cap-add=SYS_PTRACE fiddler

include ../make/npm.mk
