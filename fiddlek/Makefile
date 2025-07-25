include ../make/bazel.mk
include ../make/docker.mk

# First create a list of all the modules that ./go/ts depends upon.
tsDependentModules := $(shell go list -f '{{ join .Imports "\n" }}' go.skia.org/infra/fiddlek/go/ts/  )

# Then use that list to find all the Go files that make up those dependent modules.
tsDependentFiles := $(shell echo $(tsDependentModules) | xargs -L 1 go list -f '{{ $$dir := .Dir }}{{ range  .GoFiles }}{{ $$dir }}/{{ . }}{{ "\n" }}{{ end }}' )

./modules/json/index.ts: $(tsDependentFiles)
	go generate ./go/...

fiddle_secwrap:
	cd cpp; docker build --tag secwrap_builder .
	# This doesn't actually make the built executable available, just tests that
	# we can build it.

build: ./modules/json/index.ts fiddle_secwrap
	$(BAZEL) build --config=mayberemote go/... modules/... pages/...

serve:
	@echo "Bazel doesn't have a serve-all option."
	@echo "For the module under development, try running something like:"
	@echo "bazel run modules/fiddle-embed-sk:demo_page_server"

cli:
	go install -v ./go/fiddlecli

test: testgo testjs

testjs:
	$(BAZEL) test --config=mayberemote modules/...

testgo:
	go test -v ./...

# This presumes a fiddler is available at port :8000 and scrapexchange is available at port :9000.
# You can either run a local copy of scrapexchange, or port-forward the production instance:
#   kubectl port-forward service/scrapexchange 9000
.PHONY: run_local_fiddle
run_local_fiddle: build
	../_bazel_bin/fiddlek/go/fiddle/fiddle_/fiddle --local --port=:8080 \
		--source_image_dir=/etc/fiddle/source --scrapexchange=http://localhost:9000 \
		--dist_dir=../_bazel_bin/fiddlek/pages/development

# Runs a local fiddler.
.PHONY: run_local_fiddler
run_local_fiddler:
	$(DOCKER) run -ti -p8000:8000 --cap-add=SYS_PTRACE gcr.io/skia-public/fiddler:prod --fiddle_root=/tmp --checkout=/tmp/skia/skia/ --local
