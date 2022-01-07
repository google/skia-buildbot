include ../make/bazel.mk
include ../kube/kube.mk

# First create a list of all the modules that ./go/ts depends upon.
tsDependentModules := $(shell go list -f '{{ join .Imports "\n" }}' go.skia.org/infra/fiddlek/go/ts/  )

# Then use that list to find all the Go files that make up those dependent modules.
tsDependentFiles := $(shell echo $(tsDependentModules) | xargs -L 1 go list -f '{{ $$dir := .Dir }}{{ range  .GoFiles }}{{ $$dir }}/{{ . }}{{ "\n" }}{{ end }}' )

./modules/json/index.ts: $(tsDependentFiles)
	go generate ./go/...

fiddle_secwrap:
	clang++ -O2 ./cpp/fiddle_secwrap.cpp -o ./bin/fiddle_secwrap

build: ./modules/json/index.ts fiddle_secwrap
	$(BAZEL) build go/... modules/... pages/...

serve:
	@echo "Bazel doesn't have a serve-all option."
	@echo "For the module under development, try running something like:"
	@echo "bazel run modules/fiddle-embed-sk:demo_page_server"

release-fiddle: build
	$(BAZEL) run //fiddlek:push_fiddle_container

release-fiddler: build
	docker pull gcr.io/skia-public/skia-release:prod
	mkdir /tmp/release-fiddler-skia-tmp-checkout
	# Extract skia checkout from the most recently built docker container.
	# This is built with @skia//docker/skia-wasm-release/Dockerfile
	docker run --rm -u $$(id -u ${USER}):$$(id -g ${USER}) \
	    --volume /tmp/release-fiddler-skia-tmp-checkout:/OUT gcr.io/skia-public/skia-release:prod \
	    sh -c "cp -r /tmp/skia/skia/. /OUT/"
	$(BAZEL) run //fiddlek:push_fiddler_container
	# Cleanup the skia checkout.
	rm -rf /tmp/release-fiddler-skia-tmp-checkout

release: release-fiddle release-fiddler

# Used by the autodeployment framework:
# https://skia.googlesource.com/buildbot/+doc/main/docker_pushes_watcher/README.md
release-fiddler-ci: build
	# COPY_FROM_DIR and STABLE_DOCKER_TAG env variables are expected to be
	# set by the auto deploy mechanism,
	mkdir /tmp/release-fiddler-skia-tmp-checkout
	cp -r $(COPY_FROM_DIR)/. build/skia/
	${BAZEL} run //fiddlek:push_fiddler_container --workspace_status_command=bazel/override_container_label.sh
	# Cleanup the skia checkout.
	rm -rf /tmp/release-fiddler-skia-tmp-checkout

push: pushk release
	pushk --use-temp-checkout fiddle fiddler

push_fiddle: pushk release-fiddle
	pushk --use-temp-checkout fiddle

cli:
	go install -v ./go/fiddlecli

test: testgo testcpp testjs

testjs:
	$(BAZEL) test modules/...

testgo:
	go test -v ./...

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
# You can either run a local copy of scrapexchange, or port-forward the production instance:
#   kubectl port-forward service/scrapexchange 9000
.PHONY: run_local_fiddle
run_local_fiddle: build
	../_bazel_bin/fiddlek/go/fiddle/fiddle_/fiddle --local --port=:8080 --source_image_dir=/etc/fiddle/source --scrapexchange=localhost:9000 --dist_dir=../_bazel_bin/fiddlek/pages/development

# Runs a local fiddler.
.PHONY: run_local_fiddler
run_local_fiddler:
	docker run -ti -p8000:8000 --cap-add=SYS_PTRACE fiddler --fiddle_root=/tmp --checkout=/tmp/skia/skia/ --local
