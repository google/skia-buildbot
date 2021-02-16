.PHONY: default
default: ./dist
	go install -v ./go/...

docs:
	go install ./go/perf-tool ./go/perfserver
	perf-tool markdown > PERFTOOL.md
	perfserver markdown > PERFSERVER.md

# The command line tool that embeds plain files into Go code.
${GOPATH}/bin/rice:
	go get github.com/GeertJohan/go.rice/rice

go/sql/migrations/cockroachdb/rice-box.go: $(wildcard ./migrations/cockroachdb/*) ${GOPATH}/bin/rice
	cd go/sql/migrations/cockroachdb; rice embed-go

go/dist/rice-box.go: ./dist ${GOPATH}/bin/rice
	cd go/dist; rice embed-go

# The webpack output in ./dist is embedded via Go files and linked into the executable.
.PHONY: dist_source
dist_source: go/dist/rice-box.go

# The SQL migration files are embedded via Go files and linked into the executable.
.PHONY: migration_source
migration_source: go/sql/migrations/cockroachdb/rice-box.go

build: migration_source dist_source
	CGO_ENABLED=0 GOOS=linux go install -a ./go/perfserver ./go/perf-tool
	./images/perfserver/release

push: build
	pushk perfserver

package-lock.json: package.json
	npm ci
	touch package-lock.json

build_backup: migration_source
	CGO_ENABLED=0 GOOS=linux go install -a ./go/perf-tool
	./images/backup/release

push_backup: build_backup
	pushk perf-cockroachdb-backup

# First create a list of all the modules that ./go/ts depends upon.
tsDependentModules := $(shell go list -f '{{ join .Imports "\n" }}' go.skia.org/infra/perf/go/ts/  )

# Then use that list to find all the Go files that make up those dependent modules.
tsDependentFiles := $(shell echo $(tsDependentModules) | xargs -L 1 go list -f '{{ $$dir := .Dir }}{{ range  .GoFiles }}{{ $$dir }}/{{ . }}{{ "\n" }}{{ end }}' )

./modules/json/index.ts: $(tsDependentFiles)
	go generate ./go/ts

MODULES = $(shell find ./modules -type f)

# We can depend on the timestamp of the dist directory since webpack will delete
# the directory before repopulating it.
./dist: package-lock.json $(MODULES) ./modules/json/index.ts
	npx webpack-cli --mode=production

watch: package-lock.json
	npx webpack --mode=development --watch

.PHONY: testjs
testjs:
	# Run the generated tests just once under Xvfb.
	xvfb-run --auto-servernum --server-args "-screen 0 1280x1024x24" npx karma start --single-run

# Increase Node's heap size to accommodate for ts-node's higher memory usage.
NODE_MEMORY_LIMIT=4096

.PHONY: testpptr
testpptr:
	NODE_OPTIONS="--max-old-space-size=$(NODE_MEMORY_LIMIT)" npx mocha -r ts-node/register ./**/*_puppeteer_test.ts

.PHONY: test
test: testgo testjs testpptr

.PHONY: testgo
testgo:
	go test ./go/...

.PHONY: debug
debug:
	npx webpack-cli --mode=development

.PHONY: all # For ../Makefile 'all' target.
all:
	go install -v ./go/...
	npm ci
	npx webpack-cli --mode=production

start_pubsub_emulator:
	docker run -ti -p 8010:8010 google/cloud-sdk:latest gcloud beta emulators pubsub start \
		  --project test-project --host-port 0.0.0.0:8010

start_cockroachdb_emulator:
	cd /tmp; cockroach start-single-node --insecure  --listen-addr=127.0.0.1

.PHONY: build-frontend-ci
build-frontend-ci:
	npm ci
	npx webpack --mode=production

.PHONY: test-frontend-ci
test-frontend-ci:
	npm ci
	xvfb-run --auto-servernum --server-args "-screen 0 1280x1024x24" npx karma start --single-run

# Presumes you've already run:
#
#   kubectl port-forward perf-cockroachdb-0 25000:26257
#
run-local-instance:
	perfserver frontend --local --do_clustering=true --port=:8001 --noemail --config_filename=./configs/cdb-nano.json --num_continuous_parallel=1 --display_group_by=true --connection_string=postgresql://root@127.0.0.1:25000/skia?sslmode=disable
