include ../make/bazel.mk

# First create a list of all the modules that ./go/ts depends upon.
tsDependentModules := $(shell go list -f '{{ join .Imports "\n" }}' go.skia.org/infra/fiddlek/go/ts/  )

# Then use that list to find all the Go files that make up those dependent modules.
tsDependentFiles := $(shell echo $(tsDependentModules) | xargs -L 1 go list -f '{{ $$dir := .Dir }}{{ range  .GoFiles }}{{ $$dir }}/{{ . }}{{ "\n" }}{{ end }}' )

./modules/json/index.ts: $(tsDependentFiles)
	go generate ./go/...

fiddle_secwrap:
	$(BAZEL) build --config=mayberemote //fiddlek/cpp:fiddle_secwrap

build: ./modules/json/index.ts fiddle_secwrap
	$(BAZEL) build --config=mayberemote go/... modules/... pages/...

serve:
	@echo "Bazel doesn't have a serve-all option."
	@echo "For the module under development, try running something like:"
	@echo "bazel run modules/fiddle-embed-sk:demo_page_server"

release-fiddle: build
	$(BAZEL) run --config=mayberemote //fiddlek:push_fiddle_container

build-fiddler:
	docker pull gcr.io/skia-public/skia-release:prod
	# Extract skia checkout from the most recently built docker container.
	# This is built with @skia//docker/skia-wasm-release/Dockerfile
	docker run --rm -u $$(id -u ${USER}):$$(id -g ${USER}) \
	    --volume `pwd`/build:/OUT gcr.io/skia-public/skia-release:prod \
	   sh -c "cd /tmp; tar cvzf skia.tar.gz --directory=/tmp/skia skia; cp /tmp/skia.tar.gz /OUT/"

release-fiddler: build build-fiddler
	$(BAZEL) run --config=mayberemote //fiddlek:push_fiddler_container
	# Cleanup the build directory.
	git checkout build/*

release: release-fiddle release-fiddler

# Used by the autodeployment framework:
# https://skia.googlesource.com/buildbot/+doc/main/docker_pushes_watcher/README.md
release-fiddler-ci: build
	# COPY_FROM_DIR and STABLE_DOCKER_TAG env variables are expected to be
	# set by the auto deploy mechanism,
	cp $(COPY_FROM_DIR)/skia.tar.gz build/skia.tar.gz
	${BAZEL} run //fiddlek:push_fiddler_container --workspace_status_command=bazel/override_container_label.sh
	# Cleanup the build directory.
	git checkout build/*

push: release
	$(BAZEL) run --config=mayberemote //kube/go/pushk -- --use-temp-checkout fiddle fiddler

push_fiddle: release-fiddle
	$(BAZEL) run --config=mayberemote //kube/go/pushk -- --use-temp-checkout fiddle

push_fiddler: release-fiddler
	$(BAZEL) run --config=mayberemote //kube/go/pushk -- --use-temp-checkout fiddler

cli:
	go install -v ./go/fiddlecli

test: testgo testcpp testjs

testjs:
	$(BAZEL) test --config=mayberemote modules/...

testgo:
	go test -v ./...

testcpp: build
	mkdir -p bin
	clang ./cpp/fail_socket.cpp -o ./bin/fail_socket
	clang ./cpp/fail_execve.cpp -o ./bin/fail_execve
	clang ./cpp/fail_link.cpp -o ./bin/fail_link
	clang ./cpp/fail_rename.cpp -o ./bin/fail_rename
	clang ./cpp/fail_mknod.cpp -o ./bin/fail_mknod
	../_bazel_bin/fiddlek/cpp/fiddle_secwrap ./bin/fail_socket; test $$? -eq 1
	../_bazel_bin/fiddlek/cpp/fiddle_secwrap ./bin/fail_execve; test $$? -eq 255
	../_bazel_bin/fiddlek/cpp/fiddle_secwrap ./bin/fail_link; test $$? -eq 255
	../_bazel_bin/fiddlek/cpp/fiddle_secwrap ./bin/fail_rename; test $$? -eq 255
	../_bazel_bin/fiddlek/cpp/fiddle_secwrap ./bin/fail_mknod; test $$? -eq 255
	echo "SUCCESS"

# This presumes a fiddler is available at port :8000 and scrapexchange is available at port :9000.
# You can either run a local copy of scrapexchange, or port-forward the production instance:
#   kubectl port-forward service/scrapexchange 9000
.PHONY: run_local_fiddle
run_local_fiddle: build
	../_bazel_bin/fiddlek/go/fiddle/fiddle_/fiddle --local --port=:8080 --source_image_dir=/etc/fiddle/source --scrapexchange=localhost:9000 --dist_dir=../_bazel_bin/fiddlek/pages/development

# Runs a local fiddler.
.PHONY: run_local_fiddler
run_local_fiddler:
	docker run -ti -p8000:8000 --cap-add=SYS_PTRACE gcr.io/skia-public/fiddler:prod --fiddle_root=/tmp --checkout=/tmp/skia/skia/ --local
