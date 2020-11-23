# Require clean_webtools to ensure changes to res/common/imp/*.html are reflected.
.PHONY: ctfe
ctfe:
	cd ../infra-sk && npm ci
	npm ci
	npx webpack --mode=production
	go install -v ./go/ctfe

.PHONY: ctfe_debug
ctfe_debug:
	npx webpack --mode=development
	go install -v ./go/ctfe

.PHONY: test_js
test_js:
	npx webpack --mode=development
	xvfb-run --auto-servernum --server-args "-screen 0 1280x1024x24" npx karma start --single-run

# Increase Node's heap size to accommodate for ts-node's higher memory usage.
NODE_MEMORY_LIMIT=4096

.PHONY: puppeteer-tests
puppeteer-tests:
	NODE_OPTIONS="--max-old-space-size=$(NODE_MEMORY_LIMIT)" npx mocha -r ts-node/register ./**/*_puppeteer_test.ts

.PHONY: serve
serve:
	npx webpack-dev-server --watch_poll --mode=development

.PHONY: local_image
local_image: all
	CGO_ENABLED=0 GOOS=linux go install -a ./go/ctfe
	SKIP_UPLOAD=1 ./build_ctfe_release

.PHONY: release
release: all
	CGO_ENABLED=0 GOOS=linux go install -a ./go/ctfe
	./build_ctfe_release

.PHONY: push
push: release
	pushk --use-temp-checkout ctfe

.PHONY: push_staging
push_staging: release
	pushk --use-temp-checkout ctfe-staging

# Master scripts.
.PHONY: create_pagesets_on_workers
create_pagesets_on_workers:
	go install -v ./go/master_scripts/create_pagesets_on_workers/

.PHONY: capture_archives_on_workers
capture_archives_on_workers:
	go install -v ./go/master_scripts/capture_archives_on_workers/

.PHONY: run_chromium_perf_on_workers
run_chromium_perf_on_workers:
	go install -v ./go/master_scripts/run_chromium_perf_on_workers/

.PHONY: run_chromium_analysis_on_workers
run_chromium_analysis_on_workers:
	go install -v ./go/master_scripts/run_chromium_analysis_on_workers/

.PHONY: metrics_analysis_on_workers
metrics_analysis_on_workers:
	go install -v ./go/master_scripts/metrics_analysis_on_workers/

# Worker scripts.
.PHONY: create_pagesets
create_pagesets:
	go install -v ./go/worker_scripts/create_pagesets/

.PHONY: build_repo
build_repo:
	go install -v ./go/worker_scripts/build_repo/
	GOOS=windows GOARCH=amd64 go build -o ${GOPATH}/bin/build_repo.exe ./go/worker_scripts/build_repo/

.PHONY: isolate_telemetry
isolate_telemetry:
	go install -v ./go/worker_scripts/isolate_telemetry/
	GOOS=windows GOARCH=amd64 go build -o ${GOPATH}/bin/isolate_telemetry.exe ./go/worker_scripts/isolate_telemetry/

.PHONY: capture_archives
capture_archives:
	go install -v ./go/worker_scripts/capture_archives/

.PHONY: run_chromium_perf
run_chromium_perf:
	go install -v ./go/worker_scripts/run_chromium_perf/
	GOOS=windows GOARCH=amd64 go build -o ${GOPATH}/bin/run_chromium_perf.exe ./go/worker_scripts/run_chromium_perf/

.PHONY: run_chromium_analysis
run_chromium_analysis:
	go install -v ./go/worker_scripts/run_chromium_analysis/
	GOOS=windows GOARCH=amd64 go build -o ${GOPATH}/bin/run_chromium_analysis.exe ./go/worker_scripts/run_chromium_analysis/

.PHONY: metrics_analysis
metrics_analysis:
	go install -v ./go/worker_scripts/metrics_analysis/

# This is the quick version triggered by the presubmit script.
.PHONY: testgo
testgo:
	go test ./go/... -v -test.short

.PHONY: test
test: test_js puppeteer-tests
	go test ./go/...

.PHONY: master_scripts
master_scripts: create_pagesets_on_workers capture_archives_on_workers \
        run_chromium_perf_on_workers \
        run_chromium_analysis_on_workers metrics_analysis_on_workers

.PHONY: worker_scripts
worker_scripts: create_pagesets capture_archives \
                run_chromium_perf \
                run_chromium_analysis build_repo metrics_analysis \
                isolate_telemetry

.PHONY: all
all: ctfe master_scripts worker_scripts testgo

.PHONY: build-frontend-ci
build-frontend-ci:
	npm ci
	npx webpack --mode=production

.PHONY: test-frontend-ci
test-frontend-ci:
	npm ci
	xvfb-run --auto-servernum --server-args "-screen 0 1280x1024x24" npx karma start --single-run
